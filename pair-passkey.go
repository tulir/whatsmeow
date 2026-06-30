// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"go.mau.fi/util/random"
	"golang.org/x/crypto/curve25519"
	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/util/gcmutil"
	"go.mau.fi/whatsmeow/util/hkdfutil"
	"go.mau.fi/whatsmeow/util/keys"
)

type passkeyLinkingCache struct {
	keyPair        *keys.KeyPair
	companionNonce []byte
	pairingRef     string
	deviceType     waCompanionReg.DeviceProps_PlatformType

	encryptionKey []byte
}

type passkeyHandoffKey struct {
	hmac []byte
	ts   time.Time
}

func (k *passkeyHandoffKey) Valid() bool {
	return k != nil && time.Since(k.ts) < 5*time.Minute
}

func (cli *Client) handlePasskeyNotification(ctx context.Context, node *waBinary.Node) {
	if fromJID := node.AttrGetter().JID("from"); fromJID != types.ServerJID {
		cli.Log.Warnf("Ignoring passkey notification from non-server JID %s", fromJID)
		return
	}
	pubKey, err := parsePasskeyNotification(node)
	if err != nil {
		cli.Log.Warnf("Failed to parse passkey notification: %v", err)
		var secondErr error
		pubKey, secondErr = cli.getPasskeyRequestOptions(ctx)
		if secondErr != nil {
			cli.Log.Warnf("Failed to fetch passkey options: %v", secondErr)
			cli.dispatchEvent(&events.PairPasskeyError{
				Error:        fmt.Errorf("failed to parse passkey notification: %w (fetching key also failed: %w)", err, secondErr),
				Continuation: false,
			})
			return
		}
		cli.Log.Debugf("Successfully fetched passkey options after failing to parse notification")
	}
	cli.passkeyHandoffKey.Store(&passkeyHandoffKey{
		hmac: hkdfutil.SHA256(cli.Store.AdvSecretKey, nil, []byte("shortcake-passkey-handoff-v1"), 32),
		ts:   time.Now(),
	})
	cli.Store.AdvSecretKey = random.Bytes(32)
	cli.dispatchEvent(&events.PairPasskeyRequest{PublicKey: pubKey})
}

// SendPasskeyResponse sends a WebAuthn response from the authenticator to the server.
// This should be called after receiving an [*events.PairPasskeyRequest] and asking the authenticator for a response.
func (cli *Client) SendPasskeyResponse(ctx context.Context, passkeyResp *types.WebAuthnResponse) error {
	marshaledResp, err := json.Marshal(passkeyResp)
	if err != nil {
		return fmt.Errorf("failed to marshal WebAuthnResponse: %w", err)
	}

	companionRef, err := cli.getCompanionRef(ctx)
	if err != nil {
		return fmt.Errorf("failed to get companion ref: %w", err)
	}

	companionEphemeralKeyPair := keys.NewKeyPair()
	companionNonce := random.Bytes(32)
	deviceType := store.DeviceProps.GetPlatformType()
	ident, err := proto.Marshal(&waCompanionReg.CompanionEphemeralIdentity{
		PublicKey:  companionEphemeralKeyPair.Pub[:],
		DeviceType: &deviceType,
		Ref:        &companionRef,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal CompanionEphemeralIdentity: %w", err)
	}
	commitment := sha256.Sum256(append(ident, companionNonce...))
	prologuePayload, err := proto.Marshal(&waCompanionReg.ProloguePayload{
		CompanionEphemeralIdentity: ident,
		Commitment: &waCompanionReg.CompanionCommitment{
			Hash: commitment[:],
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal ProloguePayload: %w", err)
	}
	cli.passkeyLinkingCache.Store(&passkeyLinkingCache{
		keyPair:        companionEphemeralKeyPair,
		companionNonce: companionNonce,
		pairingRef:     companionRef,
		deviceType:     deviceType,
	})
	prologueContent := []waBinary.Node{
		{Tag: "credential_id", Content: []byte(passkeyResp.RawID)},
		{Tag: "webauthn_assertion", Content: marshaledResp},
		{Tag: "prologue_payload", Content: prologuePayload},
	}
	if handoffKey := cli.passkeyHandoffKey.Load(); handoffKey.Valid() {
		h := hmac.New(sha256.New, handoffKey.hmac)
		h.Write(prologuePayload)
		pairingHandoffProof := h.Sum(nil)
		prologueContent = append(prologueContent, waBinary.Node{
			Tag:     "pairing_handoff_proof",
			Content: pairingHandoffProof,
		})
		cli.passkeySkipHandoffUX.Store(true)
	} else {
		cli.passkeySkipHandoffUX.Store(false)
	}
	_, err = cli.sendIQ(ctx, infoQuery{
		Namespace: "md",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag:     "passkey_prologue",
			Content: prologueContent,
		}},
	})
	if err != nil {
		return fmt.Errorf("failed to send passkey response: %w", err)
	}
	cli.passkeyHandoffKey.Store(nil)
	return nil
}

func (cli *Client) tryHandlePasskeyContinuationNotification(ctx context.Context, node *waBinary.Node) {
	if fromJID := node.AttrGetter().JID("from"); fromJID != types.ServerJID {
		cli.Log.Warnf("Ignoring passkey continuation notification from non-server JID %s", fromJID)
		return
	}
	err := cli.handlePasskeyContinuationNotification(ctx, node)
	if err != nil {
		cli.Log.Warnf("Failed to handle passkey continuation notification: %v", err)
		cli.dispatchEvent(&events.PairPasskeyError{
			Error:        err,
			Continuation: true,
		})
	}
}

func (cli *Client) handlePasskeyContinuationNotification(ctx context.Context, node *waBinary.Node) error {
	cache := cli.passkeyLinkingCache.Load()
	if cache == nil {
		return fmt.Errorf("received passkey continuation notification without a linking cache")
	}
	primaryEphemeralIdentity, err := parsePasskeyContinuationNotification(node)
	if err != nil {
		return fmt.Errorf("failed to parse passkey continuation notification: %w", err)
	}
	sharedSecret, err := curve25519.X25519(cache.keyPair.Priv[:], primaryEphemeralIdentity.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to compute shared secret: %w", err)
	}

	_, err = cli.sendIQ(ctx, infoQuery{
		Namespace: "md",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag:     "companion_nonce",
			Content: cache.companionNonce,
		}},
	})
	if err != nil {
		return fmt.Errorf("failed to send companion nonce: %w", err)
	}

	const info = "Pairing Information Encryption Key"
	salt := fmt.Sprintf("Companion Pairing %d with ref %s", cache.deviceType, cache.pairingRef)
	cache.encryptionKey = hkdfutil.SHA256(sharedSecret, []byte(salt), []byte(info), 32)
	digest := sha256.Sum256(append(cache.companionNonce, primaryEphemeralIdentity.PublicKey...))
	codeBytes := make([]byte, 5)
	for i := range codeBytes {
		codeBytes[i] = primaryEphemeralIdentity.Nonce[i] ^ digest[i]
	}
	encodedCode := linkingBase32.EncodeToString(codeBytes)
	cli.dispatchEvent(&events.PairPasskeyConfirmation{
		Code:          encodedCode[0:4] + "-" + encodedCode[4:],
		SkipHandoffUX: cli.passkeySkipHandoffUX.Load(),
	})
	return nil
}

// SendPasskeyConfirmation sends a confirmation to the server that the pairing code in [*events.PairPasskeyConfirmation]
// was shown to the user and they confirmed it. If the event has the SkipHandoffUX flag, showing the code to the user
// can be skipped.
func (cli *Client) SendPasskeyConfirmation(ctx context.Context) error {
	cache := cli.passkeyLinkingCache.Load()
	if cache == nil {
		return fmt.Errorf("no passkey linking cache available")
	} else if cache.encryptionKey == nil {
		return fmt.Errorf("passkey linking cache does not have an encryption key yet")
	}
	req, err := proto.Marshal(&waCompanionReg.PairingRequest{
		CompanionPublicKey:   cli.Store.NoiseKey.Pub[:],
		CompanionIdentityKey: cli.Store.IdentityKey.Pub[:],
		AdvSecret:            cli.Store.AdvSecretKey,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal PairingRequest: %w", err)
	}
	iv := random.Bytes(12)
	encryptedReq, err := gcmutil.Encrypt(cache.encryptionKey, iv, req, nil)
	if err != nil {
		return fmt.Errorf("failed to encrypt PairingRequest: %w", err)
	}
	wrappedReq, err := proto.Marshal(&waCompanionReg.EncryptedPairingRequest{
		EncryptedPayload: encryptedReq,
		IV:               iv,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal EncryptedPairingRequest: %w", err)
	}
	_, err = cli.sendIQ(ctx, infoQuery{
		Namespace: "md",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag:     "encrypted_pairing_request",
			Content: wrappedReq,
		}},
	})
	if err != nil {
		return err
	}
	cli.passkeyLinkingCache.Store(nil)
	return nil
}

func (cli *Client) getCompanionRef(ctx context.Context) (string, error) {
	resp, err := cli.sendIQ(ctx, infoQuery{
		Namespace: "md",
		Type:      iqGet,
		To:        types.ServerJID,
		Content:   []waBinary.Node{{Tag: "ref"}},
	})
	if err != nil {
		return "", err
	}
	ref, ok := resp.GetOptionalChildByTag("ref")
	if !ok {
		return "", &ElementMissingError{Tag: "ref", In: "get ref response"}
	}
	contentBytes, ok := ref.Content.([]byte)
	if !ok {
		return "", fmt.Errorf("unexpected content type %T for <ref> node", ref.Content)
	}
	return string(contentBytes), nil
}

func (cli *Client) getPasskeyRequestOptions(ctx context.Context) (*types.WebAuthnPublicKey, error) {
	resp, err := cli.sendIQ(ctx, infoQuery{
		Namespace: "md",
		Type:      iqGet,
		To:        types.ServerJID,
		Content:   []waBinary.Node{{Tag: "passkey_request_options"}},
	})
	if err != nil {
		return nil, err
	}
	return parsePasskeyNotification(resp)
}

func parsePasskeyNotification(node *waBinary.Node) (*types.WebAuthnPublicKey, error) {
	opts, ok := node.GetOptionalChildByTag("passkey_request_options")
	if !ok {
		return nil, &ElementMissingError{Tag: "passkey_request_options", In: "passkey notification"}
	}
	content, ok := opts.Content.([]byte)
	if !ok {
		return nil, fmt.Errorf("unexpected content type %T for <passkey_request_options> node", opts.Content)
	}
	var pubKey types.WebAuthnPublicKey
	err := json.Unmarshal(content, &pubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal <passkey_request_options> content: %w", err)
	}
	return &pubKey, nil
}

func parsePasskeyContinuationNotification(node *waBinary.Node) (*waCompanionReg.PrimaryEphemeralIdentity, error) {
	opts, ok := node.GetOptionalChildByTag("primary_ephemeral_identity")
	if !ok {
		return nil, &ElementMissingError{Tag: "primary_ephemeral_identity", In: "passkey continuation notification"}
	}
	content, ok := opts.Content.([]byte)
	if !ok {
		return nil, fmt.Errorf("unexpected content type %T for <primary_ephemeral_identity> node", opts.Content)
	}
	var buf waCompanionReg.PrimaryEphemeralIdentity
	err := proto.Unmarshal(content, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal primary ephemeral identity: %w", err)
	} else if len(buf.PublicKey) != 32 {
		return nil, fmt.Errorf("unexpected public key length %d primary ephemeral identity", len(buf.PublicKey))
	} else if len(buf.Nonce) != 32 {
		return nil, fmt.Errorf("unexpected nonce length %d primary ephemeral identity", len(buf.Nonce))
	}
	return &buf, err
}
