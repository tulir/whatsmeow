// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"errors"
	"time"

	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types/events"
)

func (cli *Client) handleEncryptNotification(node *waBinary.Node) {
	cli.Log.Infof("Got encryption notification from server: %s", node.XMLString())
	count := node.GetChildByTag("count")
	ag := count.AttrGetter()
	otksLeft := ag.Int("value")
	if !ag.OK() {
		cli.Log.Warnf("Didn't get number of OTKs left in encryption notification")
		return
	}
	if otksLeft < MinPreKeyCount {
		cli.uploadPreKeys()
	}
}

func (cli *Client) handleAppStateNotification(node *waBinary.Node) {
	for _, collection := range node.GetChildrenByTag("collection") {
		ag := collection.AttrGetter()
		name := appstate.WAPatchName(ag.String("name"))
		version := ag.Uint64("version")
		cli.Log.Debugf("Got server sync notification that app state %s has updated to version %d", name, version)
		err := cli.FetchAppState(name, false, false)
		if errors.Is(err, ErrIQDisconnected) || errors.Is(err, ErrNotConnected) {
			// There are some app state changes right before a remote logout, so stop syncing if we're disconnected.
			cli.Log.Debugf("Failed to sync app state after notification: %v, not trying to sync other states", err)
			return
		} else if err != nil {
			cli.Log.Errorf("Failed to sync app state after notification: %v", err)
		}
	}
}

func (cli *Client) handlePictureNotification(node *waBinary.Node) {
	ts := time.Unix(node.AttrGetter().Int64("t"), 0)
	for _, child := range node.GetChildren() {
		ag := child.AttrGetter()
		var evt events.Picture
		evt.Timestamp = ts
		evt.JID = ag.JID("jid")
		evt.Author = ag.JID("author")
		if child.Tag == "remove" {
			evt.Remove = true
		} else if child.Tag == "add" {
			evt.PictureID = ag.String("id")
		} else {
			continue
		}
		cli.dispatchEvent(&evt)
	}
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
	case "server_sync":
		go cli.handleAppStateNotification(node)
	case "account_sync":
		// If we start storing device lists locally, then this should update that store
	case "devices":
		// This is probably other users' devices
	case "w:gp2":
		evt, err := parseGroupChange(node)
		if err != nil {
			cli.Log.Errorf("Failed to parse group info change: %v", err)
		} else {
			go cli.dispatchEvent(evt)
		}
	case "picture":
		go cli.handlePictureNotification(node)
	}
}
