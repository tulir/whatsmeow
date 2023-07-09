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

	"golang.org/x/crypto/pbkdf2"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
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

func encodeLinkingCode(code []byte) string {
	encoded := linkingBase32.EncodeToString(code)
	return encoded[0:4] + "-" + encoded[4:]
}

type phoneLinkingCache struct {
	keyPair    *keys.KeyPair
	pairingRef string
}

func generateCompanionEphemeralKey() (ephemeralKeyPair *keys.KeyPair, ephemeralKey []byte, encodedLinkingCode string) {
	ephemeralKeyPair = keys.NewKeyPair()
	firstRandom := make([]byte, 32)
	secondRandom := make([]byte, 16)
	linkingCode := make([]byte, 5)
	_, err := rand.Read(firstRandom[:])
	if err != nil {
		panic(err)
	}
	_, err = rand.Read(secondRandom[:])
	if err != nil {
		panic(err)
	}
	_, err = rand.Read(linkingCode[:])
	if err != nil {
		panic(err)
	}
	encodedLinkingCode = encodeLinkingCode(linkingCode)
	linkCodeKey := pbkdf2.Key([]byte(encodedLinkingCode), firstRandom, 2<<16, 32, sha256.New)
	linkCipherBlock, _ := aes.NewCipher(linkCodeKey)
	encryptedPubkey := ephemeralKeyPair.Pub[:]
	cipher.NewCTR(linkCipherBlock, make([]byte, 16)).XORKeyStream(encryptedPubkey, encryptedPubkey)
	ephemeralKey = make([]byte, 80)
	copy(ephemeralKey[0:32], firstRandom)
	copy(ephemeralKey[32:48], secondRandom)
	copy(ephemeralKey[48:80], encryptedPubkey)
	return
}

func (cli *Client) PairPhone(phone string, showPushNotification bool, clientType PairClientType, clientDisplayName string) (string, error) {
	ephemeralKeyPair, ephemeralKey, encodedLinkingCode := generateCompanionEphemeralKey()
	phone = notNumbers.ReplaceAllString(phone, "")
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "md",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "link_code_companion_reg",
			Attrs: waBinary.Attrs{
				"jid":                           types.NewJID(phone, types.DefaultUserServer),
				"stage":                         "companion_hello",
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
		keyPair:    ephemeralKeyPair,
		pairingRef: string(pairingRef),
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
}
