// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type nodeHandler func(node *waBinary.Node)

func (cli *Client) handleStreamError(node *waBinary.Node) {
	cli.IsLoggedIn = false
	code, _ := node.Attrs["code"].(string)
	switch code {
	case "515":
		cli.Log.Debugf("Got 515 code, reconnecting")
		go func() {
			cli.Disconnect()
			err := cli.Connect()
			if err != nil {
				cli.Log.Errorf("Failed to reconnect after 515 code:", err)
			}
		}()
	case "401":
		conflict, ok := node.GetOptionalChildByTag("conflict")
		if ok && conflict.AttrGetter().String("type") == "device_removed" {
			go cli.dispatchEvent(&events.LoggedOut{})
			err := cli.Store.Delete()
			if err != nil {
				cli.Log.Warnf("Failed to delete store after device_removed error:", err)
			}
		}
	}
}

func (cli *Client) handleConnectSuccess(node *waBinary.Node) {
	cli.Log.Infof("Successfully authenticated")
	cli.IsLoggedIn = true
	go func() {
		count, err := cli.Store.PreKeys.UploadedPreKeyCount()
		if err != nil {
			cli.Log.Errorf("Failed to get number of prekeys on server: %v", err)
		} else if count < WantedPreKeyCount {
			cli.uploadPreKeys(count)
		}
		err = cli.SetPassive(false)
		if err != nil {
			cli.Log.Warnf("Failed to send post-connect passive IQ: %v", err)
		}
		cli.dispatchEvent(&events.Connected{})
	}()
}

// SetPassive tells the WhatsApp server whether this device is passive or not.
func (cli *Client) SetPassive(passive bool) error {
	tag := "active"
	if passive {
		tag = "passive"
	}
	_, err := cli.sendIQ(infoQuery{
		Namespace: "passive",
		Type:      "set",
		To:        types.ServerJID,
		Content:   []waBinary.Node{{Tag: tag}},
	})
	if err != nil {
		return err
	}
	return nil
}

// SendPresence updates the user's presence status on WhatsApp.
//
// You should call this at least once after connecting so that the server has your pushname.
// Otherwise, other users will see "-" as the name.
func (cli *Client) SendPresence(state types.Presence) error {
	return cli.sendNode(waBinary.Node{
		Tag: "presence",
		Attrs: waBinary.Attrs{
			"name": cli.Store.PushName,
			"type": string(state),
		},
	})
}

// SendChatPresence updates the user's typing status in a specific chat.
func (cli *Client) SendChatPresence(state types.ChatPresence, jid types.JID) error {
	return cli.sendNode(waBinary.Node{
		Tag: "chatstate",
		Attrs: waBinary.Attrs{
			"from": *cli.Store.ID,
			"to":   jid,
		},
		Content: []waBinary.Node{{Tag: string(state)}},
	})
}
