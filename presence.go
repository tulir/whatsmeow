// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"sync/atomic"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func (cli *Client) handleChatState(node *waBinary.Node) {
	source, err := cli.parseMessageSource(node)
	if err != nil {
		cli.Log.Warnf("Failed to parse chat state update: %v", err)
	} else if len(node.GetChildren()) != 1 {
		cli.Log.Warnf("Failed to parse chat state update: unexpected number of children in element (%d)", len(node.GetChildren()))
	} else {
		child := node.GetChildren()[0]
		presence := types.ChatPresence(child.Tag)
		if presence != types.ChatPresenceComposing && presence != types.ChatPresenceRecording && presence != types.ChatPresencePaused {
			cli.Log.Warnf("Unrecognized chat presence state %s", child.Tag)
		}
		cli.dispatchEvent(&events.ChatPresence{
			MessageSource: source,
			State:         presence,
		})
	}
}

func (cli *Client) handlePresence(node *waBinary.Node) {
	var evt events.Presence
	ag := node.AttrGetter()
	evt.From = ag.JID("from")
	presenceType := ag.OptionalString("type")
	if presenceType == "unavailable" {
		evt.Unavailable = true
	} else if presenceType != "" {
		cli.Log.Debugf("Unrecognized presence type '%s' in presence event from %s", presenceType, evt.From)
	}
	lastSeen := ag.OptionalString("last")
	if lastSeen != "" && lastSeen != "deny" {
		evt.LastSeen = time.Unix(ag.Int64("last"), 0)
	}
	if !ag.OK() {
		cli.Log.Warnf("Error parsing presence event: %+v", ag.Errors)
	} else {
		cli.dispatchEvent(&evt)
	}
}

// SendPresence updates the user's presence status on WhatsApp.
//
// You should call this at least once after connecting so that the server has your pushname.
// Otherwise, other users will see "-" as the name.
func (cli *Client) SendPresence(state types.Presence) error {
	if len(cli.Store.PushName) == 0 {
		return ErrNoPushName
	}
	if state == types.PresenceAvailable {
		atomic.CompareAndSwapUint32(&cli.sendActiveReceipts, 0, 1)
	} else {
		atomic.CompareAndSwapUint32(&cli.sendActiveReceipts, 1, 0)
	}
	return cli.sendNode(waBinary.Node{
		Tag: "presence",
		Attrs: waBinary.Attrs{
			"name": cli.Store.PushName,
			"type": string(state),
		},
	})
}

// SubscribePresence asks the WhatsApp servers to send presence updates of a specific user to this client.
//
// After subscribing to this event, you should start receiving *events.Presence for that user in normal event handlers.
//
// Also, it seems that the WhatsApp servers require you to be online to receive presence status from other users,
// so you should mark yourself as online before trying to use this function:
//     cli.SendPresence(types.PresenceAvailable)
func (cli *Client) SubscribePresence(jid types.JID) error {
	return cli.sendNode(waBinary.Node{
		Tag: "presence",
		Attrs: waBinary.Attrs{
			"type": "subscribe",
			"to":   jid,
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
