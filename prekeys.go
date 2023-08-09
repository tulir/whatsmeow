// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"go.mau.fi/libsignal/ecc"
	"go.mau.fi/libsignal/keys/identity"
	"go.mau.fi/libsignal/keys/prekey"
	"go.mau.fi/libsignal/util/optional"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/util/keys"
)

const (
	// WantedPreKeyCount is the number of prekeys that the client should upload to the WhatsApp servers in a single batch.
	WantedPreKeyCount = 50
	// MinPreKeyCount is the number of prekeys when the client will upload a new batch of prekeys to the WhatsApp servers.
	MinPreKeyCount = 5
)

func (cli *Client) getServerPreKeyCount() (int, error) {
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "encrypt",
		Type:      "get",
		To:        types.ServerJID,
		Content: []waBinary.Node{
			{Tag: "count"},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get prekey count on server: %w", err)
	}
	count := resp.GetChildByTag("count")
	ag := count.AttrGetter()
	val := ag.Int("value")
	return val, ag.Error()
}

func (cli *Client) uploadPreKeys() {
	cli.uploadPreKeysLock.Lock()
	defer cli.uploadPreKeysLock.Unlock()
	if cli.lastPreKeyUpload.Add(10 * time.Minute).After(time.Now()) {
		sc, _ := cli.getServerPreKeyCount()
		if sc >= WantedPreKeyCount {
			cli.Log.Debugf("Canceling prekey upload request due to likely race condition")
			return
		}
	}
	var registrationIDBytes [4]byte
	binary.BigEndian.PutUint32(registrationIDBytes[:], cli.Store.RegistrationID)
	preKeys, err := cli.Store.PreKeys.GetOrGenPreKeys(WantedPreKeyCount)
	if err != nil {
		cli.Log.Errorf("Failed to get prekeys to upload: %v", err)
		return
	}
	cli.Log.Infof("Uploading %d new prekeys to server", len(preKeys))
	_, err = cli.sendIQ(infoQuery{
		Namespace: "encrypt",
		Type:      "set",
		To:        types.ServerJID,
		Content: []waBinary.Node{
			{Tag: "registration", Content: registrationIDBytes[:]},
			{Tag: "type", Content: []byte{ecc.DjbType}},
			{Tag: "identity", Content: cli.Store.IdentityKey.Pub[:]},
			{Tag: "list", Content: preKeysToNodes(preKeys)},
			preKeyToNode(cli.Store.SignedPreKey),
		},
	})
	if err != nil {
		cli.Log.Errorf("Failed to send request to upload prekeys: %v", err)
		return
	}
	cli.Log.Debugf("Got response to uploading prekeys")
	err = cli.Store.PreKeys.MarkPreKeysAsUploaded(preKeys[len(preKeys)-1].KeyID)
	if err != nil {
		cli.Log.Warnf("Failed to mark prekeys as uploaded: %v", err)
	}
	cli.lastPreKeyUpload = time.Now()
}

type preKeyResp struct {
	bundle *prekey.Bundle
	err    error
}

func (cli *Client) fetchPreKeys(ctx context.Context, users []types.JID) (map[types.JID]preKeyResp, error) {
	requests := make([]waBinary.Node, len(users))
	for i, user := range users {
		requests[i].Tag = "user"
		requests[i].Attrs = waBinary.Attrs{
			"jid":    user,
			"reason": "identity",
		}
	}
	resp, err := cli.sendIQ(infoQuery{
		Context:   ctx,
		Namespace: "encrypt",
		Type:      "get",
		To:        types.ServerJID,
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
	respData := make(map[types.JID]preKeyResp)
	for _, child := range list.GetChildren() {
		if child.Tag != "user" {
			continue
		}
		jid := child.AttrGetter().JID("jid")
		bundle, err := nodeToPreKeyBundle(uint32(jid.Device), child)
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

func nodeToPreKeyBundle(deviceID uint32, node waBinary.Node) (*prekey.Bundle, error) {
	errorNode, ok := node.GetOptionalChildByTag("error")
	if ok && errorNode.Tag == "error" {
		return nil, fmt.Errorf("got error getting prekeys: %s", errorNode.XMLString())
	}

	registrationBytes, ok := node.GetChildByTag("registration").Content.([]byte)
	if !ok || len(registrationBytes) != 4 {
		return nil, fmt.Errorf("invalid registration ID in prekey response")
	}
	registrationID := binary.BigEndian.Uint32(registrationBytes)

	keysNode, ok := node.GetOptionalChildByTag("keys")
	if !ok {
		keysNode = node
	}

	identityKeyRaw, ok := keysNode.GetChildByTag("identity").Content.([]byte)
	if !ok || len(identityKeyRaw) != 32 {
		return nil, fmt.Errorf("invalid identity key in prekey response")
	}
	identityKeyPub := *(*[32]byte)(identityKeyRaw)

	preKey, err := nodeToPreKey(keysNode.GetChildByTag("key"))
	if err != nil {
		return nil, fmt.Errorf("invalid prekey in prekey response: %w", err)
	}
	signedPreKey, err := nodeToPreKey(keysNode.GetChildByTag("skey"))
	if err != nil {
		return nil, fmt.Errorf("invalid signed prekey in prekey response: %w", err)
	}

	return prekey.NewBundle(registrationID, deviceID,
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
