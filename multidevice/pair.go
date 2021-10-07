// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	"encoding/base64"
	"strings"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
)

type QREvent struct {
	Codes   []string
	Timeout time.Duration
}

const QRScanTimeout = 30 * time.Second

func handlePairDevice(cli *Client, node *waBinary.Node) bool {
	if node.Tag != "iq" || len(node.GetChildren()) != 1 || node.Attrs["from"] != waBinary.ServerJID {
		return false
	}

	pairDevice := node.GetChildren()[0]
	if pairDevice.Tag != "pair-device" {
		return false
	}

	err := cli.sendNode(waBinary.Node{
		Tag: "iq",
		Attrs: map[string]interface{}{
			"to":   node.Attrs["from"],
			"id":   node.Attrs["id"],
			"type": "result",
		},
	})
	if err != nil {
		cli.Log.Warnln("Failed to send acknowledgement for pair-device request:", err)
	}

	evt := &QREvent{
		Codes:   make([]string, 0, len(pairDevice.GetChildren())),
		Timeout: QRScanTimeout,
	}
	for i, child := range pairDevice.GetChildren() {
		if child.Tag != "ref" {
			cli.Log.Warnfln("pair-device node contains unexpected child tag %s at index %d", child.Tag, i)
			continue
		}
		content, ok := child.Content.([]byte)
		if !ok {
			cli.Log.Warnfln("pair-device node contains unexpected child content type %T at index %d", child, i)
			continue
		}
		evt.Codes = append(evt.Codes, cli.makeQRData(string(content)))
	}

	cli.dispatchEvent(evt)

	return true
}

func (cli *Client) makeQRData(ref string) string {
	noise := base64.StdEncoding.EncodeToString(cli.Session.NoiseKey.Pub[:])
	identity := base64.StdEncoding.EncodeToString(cli.Session.SignedIdentityKey.Pub[:])
	adv := base64.StdEncoding.EncodeToString(cli.Session.AdvSecretKey)
	return strings.Join([]string{ref, noise, identity, adv}, ",")
}
