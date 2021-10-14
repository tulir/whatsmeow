// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	"encoding/binary"
	"fmt"

	"github.com/RadicalApp/libsignal-protocol-go/ecc"
	"github.com/RadicalApp/libsignal-protocol-go/keys/identity"
	"github.com/RadicalApp/libsignal-protocol-go/keys/prekey"
	"github.com/RadicalApp/libsignal-protocol-go/util/optional"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/multidevice/keys"
)

func (cli *Client) uploadPreKeys() {
	var registrationIDBytes [4]byte
	binary.BigEndian.PutUint32(registrationIDBytes[:], cli.Session.RegistrationID)
	preKeys := cli.Session.GetOrGenPreKeys(30)
	_, err := cli.sendIQ(InfoQuery{
		Namespace: "encrypt",
		Type:      "set",
		To:        waBinary.ServerJID,
		Content: []waBinary.Node{
			{Tag: "registration", Content: registrationIDBytes[:]},
			{Tag: "type", Content: []byte{ecc.DjbType}},
			{Tag: "identity", Content: cli.Session.IdentityKey.Pub[:]},
			{Tag: "list", Content: preKeysToNodes(preKeys)},
			preKeyToNode(cli.Session.SignedPreKey),
		},
	})
	if err != nil {
		cli.Log.Errorln("Failed to send request to upload prekeys:", err)
		return
	}
	cli.Log.Debugln("Got response to uploading prekeys")
	cli.Session.MarkPreKeysAsUploaded(preKeys[len(preKeys)-1].KeyID)
}

type preKeyResp struct {
	bundle *prekey.Bundle
	err    error
}

func (cli *Client) fetchPreKeys(users []waBinary.FullJID) (map[waBinary.FullJID]preKeyResp, error) {
	requests := make([]waBinary.Node, len(users))
	for i, user := range users {
		requests[i].Tag = "user"
		requests[i].Attrs = map[string]interface{}{
			"jid":    user,
			"reason": "identity",
		}
	}
	resp, err := cli.sendIQ(InfoQuery{
		Namespace: "encrypt",
		Type:      "get",
		To:        waBinary.ServerJID,
		Content: []waBinary.Node{{
			Tag:     "key",
			Content: requests,
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send prekey request: %w", err)
	} else if len(resp.GetChildren()) == 0 {
		return nil, fmt.Errorf("got empty response to prekey request")
	}
	list := resp.GetChildByTag("list")
	respData := make(map[waBinary.FullJID]preKeyResp)
	for _, child := range list.GetChildren() {
		if child.Tag != "user" {
			continue
		}
		jid, bundle, err := nodeToPreKeyBundle(child)
		jid.AD = true
		respData[jid] = preKeyResp{bundle, err}
	}
	return respData, nil
}

func preKeyToNode(key *keys.PreKey) waBinary.Node {
	var keyID [4]byte
	binary.BigEndian.PutUint32(keyID[:], key.KeyID)
	node := waBinary.Node{
		Tag: "key",
		Content: []waBinary.Node{
			{Tag: "id", Content: keyID[1:]},
			{Tag: "value", Content: key.Pub[:]},
		},
	}
	if key.Signature != nil {
		node.Tag = "skey"
		node.Content = append(node.GetChildren(), waBinary.Node{
			Tag:     "signature",
			Content: key.Signature[:],
		})
	}
	return node
}

func nodeToPreKeyBundle(node waBinary.Node) (waBinary.FullJID, *prekey.Bundle, error) {
	jid := node.AttrGetter().JID("jid")

	errorNode := node.GetChildByTag("error")
	if errorNode.Tag == "error" {
		return jid, nil, fmt.Errorf("got error getting prekeys: %s", errorNode.XMLString())
	}

	registrationBytes, ok := node.GetChildByTag("registration").Content.([]byte)
	if !ok || len(registrationBytes) != 4 {
		return jid, nil, fmt.Errorf("invalid registration ID in prekey response")
	}
	registrationID := binary.BigEndian.Uint32(registrationBytes)

	identityKeyRaw, ok := node.GetChildByTag("identity").Content.([]byte)
	if !ok || len(identityKeyRaw) != 32 {
		return jid, nil, fmt.Errorf("invalid identity key in prekey response")
	}
	identityKeyPub := *(*[32]byte)(identityKeyRaw)

	preKey, err := nodeToPreKey(node.GetChildByTag("key"))
	if err != nil {
		return jid, nil, fmt.Errorf("invalid prekey in prekey response: %w", err)
	}
	signedPreKey, err := nodeToPreKey(node.GetChildByTag("skey"))
	if err != nil {
		return jid, nil, fmt.Errorf("invalid signed prekey in prekey response: %w", err)
	}

	return jid, prekey.NewBundle(registrationID, uint32(jid.Device),
		optional.NewOptionalUint32(preKey.KeyID), signedPreKey.KeyID,
		ecc.NewDjbECPublicKey(*preKey.Pub), ecc.NewDjbECPublicKey(*signedPreKey.Pub), *signedPreKey.Signature,
		identity.NewKey(ecc.NewDjbECPublicKey(identityKeyPub))), nil
}

func nodeToPreKey(node waBinary.Node) (*keys.PreKey, error) {
	key := keys.PreKey{
		KeyPair:   keys.KeyPair{},
		KeyID:     0,
		Signature: nil,
	}
	if id := node.GetChildByTag("id"); id.Tag != "id" {
		return nil, fmt.Errorf("prekey node doesn't contain ID tag")
	} else if idBytes, ok := id.Content.([]byte); !ok {
		return nil, fmt.Errorf("prekey ID has unexpected content (%T)", id.Content)
	} else if len(idBytes) != 3 {
		return nil, fmt.Errorf("prekey ID has unexpected number of bytes (%d, expected 3)", len(idBytes))
	} else {
		key.KeyID = binary.BigEndian.Uint32(append([]byte{0}, idBytes...))
	}
	if pubkey := node.GetChildByTag("value"); pubkey.Tag != "value" {
		return nil, fmt.Errorf("prekey node doesn't contain value tag")
	} else if pubkeyBytes, ok := pubkey.Content.([]byte); !ok {
		return nil, fmt.Errorf("prekey value has unexpected content (%T)", pubkey.Content)
	} else if len(pubkeyBytes) != 32 {
		return nil, fmt.Errorf("prekey value has unexpected number of bytes (%d, expected 32)", len(pubkeyBytes))
	} else {
		key.KeyPair.Pub = (*[32]byte)(pubkeyBytes)
	}
	if node.Tag == "skey" {
		if sig := node.GetChildByTag("signature"); sig.Tag != "signature" {
			return nil, fmt.Errorf("prekey node doesn't contain signature tag")
		} else if sigBytes, ok := sig.Content.([]byte); !ok {
			return nil, fmt.Errorf("prekey signature has unexpected content (%T)", sig.Content)
		} else if len(sigBytes) != 64 {
			return nil, fmt.Errorf("prekey signature has unexpected number of bytes (%d, expected 64)", len(sigBytes))
		} else {
			key.Signature = (*[64]byte)(sigBytes)
		}
	}
	return &key, nil
}

func preKeysToNodes(prekeys []*keys.PreKey) []waBinary.Node {
	nodes := make([]waBinary.Node, len(prekeys))
	for i, key := range prekeys {
		nodes[i] = preKeyToNode(key)
	}
	return nodes
}
