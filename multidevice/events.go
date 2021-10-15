// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	waBinary "go.mau.fi/whatsmeow/binary"
)

type nodeHandler func(node *waBinary.Node) bool

type LoggedOutEvent struct{}

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
	case "401":
		conflict, ok := node.GetOptionalChildByTag("conflict")
		if ok && conflict.AttrGetter().String("type") == "device_removed" {
			go cli.dispatchEvent(&LoggedOutEvent{})
			err := cli.Store.Delete()
			if err != nil {
				cli.Log.Warnln("Failed to delete store after device_removed error:", err)
			}
		}
	}
	return true
}

func (cli *Client) handleEncryptNotification(node *waBinary.Node) {
	count := node.GetChildByTag("count")
	ag := count.AttrGetter()
	otksLeft := ag.Int("value")
	if !ag.OK() {
		cli.Log.Warnln("Didn't get number of OTKs left in encryption notification")
		return
	}
	cli.Log.Infoln("Server said we have", otksLeft, "one-time keys left")
	cli.uploadPreKeys(otksLeft)
}

func (cli *Client) handleNotification(node *waBinary.Node) bool {
	if node.Tag != "notification" {
		return false
	}
	ag := node.AttrGetter()
	notifType := ag.String("type")
	if !ag.OK() {
		return false
	}
	cli.Log.Debugln("Received", notifType, "update")
	go cli.sendAck(node)
	switch notifType {
	case "encrypt":
		go cli.handleEncryptNotification(node)
	}
	// TODO dispatch group info changes as events
	return true
}

type ConnectedEvent struct{}

func (cli *Client) handleConnectSuccess(node *waBinary.Node) bool {
	if node.Tag != "success" {
		return false
	}
	cli.Log.Infoln("Successfully authenticated")
	go func() {
		count, err := cli.Store.PreKeys.UploadedPreKeyCount()
		if err != nil {
			cli.Log.Errorln("Failed to get number of prekeys on server:", err)
		} else if count < WantedPreKeyCount {
			cli.uploadPreKeys(count)
		}
		err = cli.sendPassiveIQ(false)
		if err != nil {
			cli.Log.Warnln("Failed to send post-connect passive IQ:", err)
		}
		cli.dispatchEvent(&ConnectedEvent{})
	}()
	return true
}

func (cli *Client) sendPassiveIQ(passive bool) error {
	tag := "active"
	if passive {
		tag = "passive"
	}
	_, err := cli.sendIQ(InfoQuery{
		Namespace: "passive",
		Type:      "set",
		To:        waBinary.ServerJID,
		Content:   []waBinary.Node{{Tag: tag}},
	})
	if err != nil {
		return err
	}
	return nil
}
