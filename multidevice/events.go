// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	waBinary "github.com/Rhymen/go-whatsapp/binary"
)

type EventHandler func(cli *Client, node *waBinary.Node) bool

var eventHandlers = [...]EventHandler{
	handlePairDevice,
}

func handlePairDevice(cli *Client, node *waBinary.Node) bool {
	if node.Tag != "iq" || len(node.GetChildren()) != 1 || node.GetChildren()[0].Tag != "pair-device" || node.Attrs["from"] != waBinary.ServerJID {
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

	return true
}
