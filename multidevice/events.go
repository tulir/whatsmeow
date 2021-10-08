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

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/multidevice/keys"
)

type nodeHandler func(node *waBinary.Node) bool

func (cli *Client) handleStreamError(node *waBinary.Node) bool {
	if node.Tag != "stream:error" {
		return false
	}
	code, _ := node.Attrs["code"].(string)
	switch code {
	case "515":
		cli.Log.Debugln("Got 515 code, reconnecting")
		go func() {
			cli.Disconnect()
			err := cli.Connect()
			if err != nil {
				cli.Log.Errorln("Failed to reconnect after 515 code:", err)
			}
		}()
	}
	return true
}

type ConnectedEvent struct {}

func (cli *Client) handleConnectSuccess(node *waBinary.Node) bool {
	if node.Tag != "success" {
		return false
	}
	cli.Log.Infoln("Successfully authenticated")
	cli.dispatchEvent(&ConnectedEvent{})

	if !cli.Session.ServerHasPreKeys() {
		cli.uploadPreKeys()
	}
	//err := cli.sendPassiveIQ(false)
	//if err != nil {
	//	cli.Log.Warnln("Failed to send post-connect passive IQ:", err)
	//}
	return true
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
			Content: key.Signature,
		})
	}
	return node
}

func preKeysToNodes(prekeys []*keys.PreKey) []waBinary.Node {
	nodes := make([]waBinary.Node, len(prekeys))
	for i, key := range prekeys {
		nodes[i] = preKeyToNode(key)
	}
	return nodes
}

func (cli *Client) uploadPreKeys() {
	var registrationIDBytes [4]byte
	binary.BigEndian.PutUint16(registrationIDBytes[2:], cli.Session.RegistrationID)
	preKeys := cli.Session.GetOrGenPreKeys(30)
	resChan, err := cli.sendRequest(waBinary.Node{
		Tag: "iq",
		Attrs: map[string]interface{}{
			"xmlns": "encrypt",
			"type":  "set",
			"to":    waBinary.ServerJID,
		},
		Content: []waBinary.Node{
			{Tag: "registration", Content: registrationIDBytes[:]},
			{Tag: "type", Content: []byte{ecc.DjbType}},
			{Tag: "identity", Content: cli.Session.IdentityKey.Pub},
			{Tag: "list", Content: preKeysToNodes(preKeys)},
			preKeyToNode(cli.Session.SignedPreKey),
		},
	})
	if err != nil {
		cli.Log.Errorln("Failed to send request to upload prekeys:", err)
		return
	}
	<-resChan
	cli.Log.Debugln("Got response to uploading prekeys")
	cli.Session.MarkPreKeysAsUploaded(preKeys[len(preKeys)-1].KeyID)
}

func (cli *Client) sendPassiveIQ(passive bool) error {
	tag := "active"
	if passive {
		tag = "passive"
	}
	res, err := cli.sendRequest(waBinary.Node{
		Tag: "iq",
		Attrs: map[string]interface{}{
			"to":    waBinary.ServerJID,
			"xmlns": "passive",
			"type":  "set",
		},
		Content: []waBinary.Node{{Tag: tag}},
	})
	if err != nil {
		return err
	}
	fmt.Println("passive iq response:", <-res)
	return nil
}
