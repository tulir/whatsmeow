// Copyright (c) 2023 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"regexp"
	"strconv"

	"go.mau.fi/util/random"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/pbkdf2"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/util/hkdfutil"
	"go.mau.fi/whatsmeow/util/keys"
)

// PairClientType is the type of client to use with PairCode.
// The type is automatically filled based on store.DeviceProps.PlatformType (which is what QR login uses).
type PairClientType int

const (
	PairClientUnknown PairClientType = iota
	PairClientChrome
	PairClientEdge
	PairClientFirefox
	PairClientIE
	PairClientOpera
	PairClientSafari
	PairClientElectron
	PairClientUWP
	PairClientOtherWebClient
)

var notNumbers = regexp.MustCompile("[^0-9]")
var linkingBase32 = base32.NewEncoding("123456789ABCDEFGHJKLMNPQRSTVWXYZ")

type phoneLinkingCache struct {
	jid         types.JID
	keyPair     *keys.KeyPair
	linkingCode string
	pairingRef  string
}

func generateCompanionEphemeralKey() (ephemeralKeyPair *keys.KeyPair, ephemeralKey []byte, encodedLinkingCode string) {
	ephemeralKeyPair = keys.NewKeyPair()
	salt := random.Bytes(32)
	iv := random.Bytes(16)
	linkingCode := random.Bytes(5)
	encodedLinkingCode = linkingBase32.EncodeToString(linkingCode)
	linkCodeKey := pbkdf2.Key([]byte(encodedLinkingCode), salt, 2<<16, 32, sha256.New)
	linkCipherBlock, _ := aes.NewCipher(linkCodeKey)
	encryptedPubkey := ephemeralKeyPair.Pub[:]
	cipher.NewCTR(linkCipherBlock, iv).XORKeyStream(encryptedPubkey, encryptedPubkey)
	ephemeralKey = make([]byte, 80)
	copy(ephemeralKey[0:32], salt)
	copy(ephemeralKey[32:48], iv)
	copy(ephemeralKey[48:80], encryptedPubkey)
	return
}

// PairPhone generates a pairing code that can be used to link to a phone without scanning a QR code.
//
// You must connect the client normally before calling this (which means you'll also receive a QR code
// event, but that can be ignored when doing code pairing).
//
// The exact expiry of pairing codes is unknown, but QR codes are always generated and the login websocket is closed
// after the QR codes run out, which means there's a 160-second time limit. It is recommended to generate the pairing
// code immediately after connecting to the websocket to have the maximum time.
//
// The clientType parameter must be one of the PairClient* constants, but which one doesn't matter.
// The client display name must be formatted as `Browser (OS)`, and only common browsers/OSes are allowed
// (the server will validate it and return 400 if it's wrong).
//
// See https://faq.whatsapp.com/1324084875126592 for more info
func (cli *Client) PairPhone(phone string, showPushNotification bool, clientType PairClientType, clientDisplayName string) (string, error) {
	ephemeralKeyPair, ephemeralKey, encodedLinkingCode := generateCompanionEphemeralKey()
	phone = notNumbers.ReplaceAllString(phone, "")
	jid := types.NewJID(phone, types.DefaultUserServer)
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "md",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "link_code_companion_reg",
			Attrs: waBinary.Attrs{
				"jid":   jid,
				"stage": "companion_hello",

				"should_show_push_notification": strconv.FormatBool(showPushNotification),
			},
			Content: []waBinary.Node{
				{Tag: "link_code_pairing_wrapped_companion_ephemeral_pub", Content: ephemeralKey},
				{Tag: "companion_server_auth_key_pub", Content: cli.Store.NoiseKey.Pub[:]},
				{Tag: "companion_platform_id", Content: strconv.Itoa(int(clientType))},
				{Tag: "companion_platform_display", Content: clientDisplayName},
				{Tag: "link_code_pairing_nonce", Content: []byte{0}},
			},
		}},
	})
	if err != nil {
		return "", err
	}
	pairingRefNode, ok := resp.GetOptionalChildByTag("link_code_companion_reg", "link_code_pairing_ref")
	if !ok {
		return "", &ElementMissingError{Tag: "link_code_pairing_ref", In: "code link registration response"}
	}
	pairingRef, ok := pairingRefNode.Content.([]byte)
	if !ok {
		return "", fmt.Errorf("unexpected type %T in content of link_code_pairing_ref tag", pairingRefNode.Content)
	}
	cli.phoneLinkingCache = &phoneLinkingCache{
		jid:         jid,
		keyPair:     ephemeralKeyPair,
		linkingCode: encodedLinkingCode,
		pairingRef:  string(pairingRef),
	}
	return encodedLinkingCode[0:4] + "-" + encodedLinkingCode[4:], nil
}

func (cli *Client) tryHandleCodePairNotification(parentNode *waBinary.Node) {
	err := cli.handleCodePairNotification(parentNode)
	if err != nil {
		cli.Log.Errorf("Failed to handle code pair notification: %s", err)
	}
}

func (cli *Client) handleCodePairNotification(parentNode *waBinary.Node) error {
	node, ok := parentNode.GetOptionalChildByTag("link_code_companion_reg")
	if !ok {
		return &ElementMissingError{
			Tag: "link_code_companion_reg",
			In:  "notification",
		}
	}
	linkCache := cli.phoneLinkingCache
	if linkCache == nil {
		return fmt.Errorf("received code pair notification without a pending pairing")
	}
	linkCodePairingRef, _ := node.GetChildByTag("link_code_pairing_ref").Content.([]byte)
	if string(linkCodePairingRef) != linkCache.pairingRef {
		return fmt.Errorf("pairing ref mismatch in code pair notification")
	}
	wrappedPrimaryEphemeralPub, ok := node.GetChildByTag("link_code_pairing_wrapped_primary_ephemeral_pub").Content.([]byte)
	if !ok {
		return &ElementMissingError{
			Tag: "link_code_pairing_wrapped_primary_ephemeral_pub",
			In:  "notification",
		}
	}
	primaryIdentityPub, ok := node.GetChildByTag("primary_identity_pub").Content.([]byte)
	if !ok {
		return &ElementMissingError{
			Tag: "primary_identity_pub",
			In:  "notification",
		}
	}

	advSecretRandom := random.Bytes(32)
	keyBundleSalt := random.Bytes(32)
	keyBundleNonce := random.Bytes(12)

	// Decrypt the primary device's ephemeral public key, which was encrypted with the 8-character pairing code,
	// then compute the DH shared secret using our ephemeral private key we generated earlier.
	primarySalt := wrappedPrimaryEphemeralPub[0:32]
	primaryIV := wrappedPrimaryEphemeralPub[32:48]
	primaryEncryptedPubkey := wrappedPrimaryEphemeralPub[48:80]
	linkCodeKey := pbkdf2.Key([]byte(linkCache.linkingCode), primarySalt, 2<<16, 32, sha256.New)
	linkCipherBlock, err := aes.NewCipher(linkCodeKey)
	if err != nil {
		return fmt.Errorf("failed to create link cipher: %w", err)
	}
	primaryDecryptedPubkey := make([]byte, 32)
	cipher.NewCTR(linkCipherBlock, primaryIV).XORKeyStream(primaryDecryptedPubkey, primaryEncryptedPubkey)
	ephemeralSharedSecret, err := curve25519.X25519(linkCache.keyPair.Priv[:], primaryDecryptedPubkey)
	if err != nil {
		return fmt.Errorf("failed to compute ephemeral shared secret: %w", err)
	}

	// Encrypt and wrap key bundle containing our identity key, the primary device's identity key and the randomness used for the adv key.
	keyBundleEncryptionKey := hkdfutil.SHA256(ephemeralSharedSecret, keyBundleSalt, []byte("link_code_pairing_key_bundle_encryption_key"), 32)
	keyBundleCipherBlock, err := aes.NewCipher(keyBundleEncryptionKey)
	if err != nil {
		return fmt.Errorf("failed to create key bundle cipher: %w", err)
	}
	keyBundleGCM, err := cipher.NewGCM(keyBundleCipherBlock)
	if err != nil {
		return fmt.Errorf("failed to create key bundle GCM: %w", err)
	}
	plaintextKeyBundle := concatBytes(cli.Store.IdentityKey.Pub[:], primaryIdentityPub, advSecretRandom)
	encryptedKeyBundle := keyBundleGCM.Seal(nil, keyBundleNonce, plaintextKeyBundle, nil)
	wrappedKeyBundle := concatBytes(keyBundleSalt, keyBundleNonce, encryptedKeyBundle)

	// Compute the adv secret key (which is used to authenticate the pair-success event later)
	identitySharedKey, err := curve25519.X25519(cli.Store.IdentityKey.Priv[:], primaryIdentityPub)
	if err != nil {
		return fmt.Errorf("failed to compute identity shared key: %w", err)
	}
	advSecretInput := append(append(ephemeralSharedSecret, identitySharedKey...), advSecretRandom...)
	advSecret := hkdfutil.SHA256(advSecretInput, nil, []byte("adv_secret"), 32)
	cli.Store.AdvSecretKey = advSecret

	_, err = cli.sendIQ(infoQuery{
		Namespace: "md",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "link_code_companion_reg",
			Attrs: waBinary.Attrs{
				"jid":   linkCache.jid,
				"stage": "companion_finish",
			},
			Content: []waBinary.Node{
				{Tag: "link_code_pairing_wrapped_key_bundle", Content: wrappedKeyBundle},
				{Tag: "companion_identity_public", Content: cli.Store.IdentityKey.Pub[:]},
				{Tag: "link_code_pairing_ref", Content: linkCodePairingRef},
			},
		}},
	})
	return err
}
