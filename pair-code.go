// Copyright (c) 2023 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"regexp"
	"strconv"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/pbkdf2"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/util/hkdfutil"
	"go.mau.fi/whatsmeow/util/keys"
)

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

func pairingRandom(length int) []byte {
	random := make([]byte, length)
	_, err := rand.Read(random)
	if err != nil {
		panic(err)
	}
	return random
}

func generateCompanionEphemeralKey() (ephemeralKeyPair *keys.KeyPair, ephemeralKey []byte, encodedLinkingCode string) {
	ephemeralKeyPair = keys.NewKeyPair()
	firstRandom := pairingRandom(32)
	secondRandom := pairingRandom(16)
	linkingCode := pairingRandom(5)
	encodedLinkingCode = linkingBase32.EncodeToString(linkingCode)
	linkCodeKey := pbkdf2.Key([]byte(encodedLinkingCode), firstRandom, 2<<16, 32, sha256.New)
	linkCipherBlock, _ := aes.NewCipher(linkCodeKey)
	encryptedPubkey := ephemeralKeyPair.Pub[:]
	cipher.NewCTR(linkCipherBlock, secondRandom).XORKeyStream(encryptedPubkey, encryptedPubkey)
	ephemeralKey = make([]byte, 80)
	copy(ephemeralKey[0:32], firstRandom)
	copy(ephemeralKey[32:48], secondRandom)
	copy(ephemeralKey[48:80], encryptedPubkey)
	return
}

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
	primaryIdentityPub, ok := node.GetChildByTag("primary_identity_pub").Content.([]byte)

	newRandom1 := pairingRandom(32)
	newRandom2 := pairingRandom(32)
	newRandom3 := pairingRandom(12)

	primaryFirstRandom := wrappedPrimaryEphemeralPub[0:32]
	primarySecondRandom := wrappedPrimaryEphemeralPub[32:48]
	primaryEncryptedPubkey := wrappedPrimaryEphemeralPub[48:80]

	linkCodeKey := pbkdf2.Key([]byte(linkCache.linkingCode), primaryFirstRandom, 2<<16, 32, sha256.New)
	linkCipherBlock, _ := aes.NewCipher(linkCodeKey)
	primaryDecryptedPubkey := make([]byte, 32)
	cipher.NewCTR(linkCipherBlock, primarySecondRandom).XORKeyStream(primaryDecryptedPubkey, primaryEncryptedPubkey)
	ephemeralSharedSecret, err := curve25519.X25519(primaryDecryptedPubkey, linkCache.keyPair.Priv[:])
	if err != nil {
		panic(err)
	}
	expanded := hkdfutil.SHA256(ephemeralSharedSecret, newRandom2, []byte("link_code_pairing_key_bundle_encryption_key"), 32)
	concattedKeys := append(append(cli.Store.IdentityKey.Pub[:], primaryIdentityPub...), newRandom1...)
	expandedBlock, _ := aes.NewCipher(expanded)
	expandedGCM, _ := cipher.NewGCM(expandedBlock)
	encryptedKeyBundle := expandedGCM.Seal(nil, newRandom3, concattedKeys, nil)
	wrappedKeyBundle := append(append(newRandom2, newRandom3...), encryptedKeyBundle...)
	anotherSharedSecret, err := curve25519.X25519(primaryIdentityPub, cli.Store.IdentityKey.Priv[:])
	if err != nil {
		panic(err)
	}
	concattedBuffers := append(append(ephemeralSharedSecret, anotherSharedSecret...), newRandom1...)
	advSecret := hkdfutil.SHA256(concattedBuffers, nil, []byte("adv_secret"), 32)
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
