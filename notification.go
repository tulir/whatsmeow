// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import waBinary "go.mau.fi/whatsmeow/binary"

func (cli *Client) handleEncryptNotification(node *waBinary.Node) {
	count := node.GetChildByTag("count")
	ag := count.AttrGetter()
	otksLeft := ag.Int("value")
	if !ag.OK() {
		cli.Log.Warnf("Didn't get number of OTKs left in encryption notification")
		return
	}
	cli.Log.Infof("Server said we have %d one-time keys left", otksLeft)
	cli.uploadPreKeys(otksLeft)
}

func (cli *Client) handleNotification(node *waBinary.Node) {
	ag := node.AttrGetter()
	notifType := ag.String("type")
	if !ag.OK() {
		return
	}
	cli.Log.Debugf("Received %s update", notifType)
	go cli.sendAck(node)
	switch notifType {
	case "encrypt":
		go cli.handleEncryptNotification(node)
	}
	// TODO dispatch group info changes as events
}
