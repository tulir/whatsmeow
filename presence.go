// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"fmt"
	"sync/atomic"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const (
	chatStateUpdateParseErrorMsg = "Failed to parse chat state update"
	unexpectedChildrenErrorMsg   = "Failed to parse chat state update: unexpected number of children in element"
	unrecognizedPresenceStateMsg = "Unrecognized chat presence state"
)

// handleChatState handles incoming chat state updates.
func (cli *Client) handleChatState(node *waBinary.Node) {
	// Parse the message source
	source, err := cli.parseMessageSource(node, true)
	if err != nil {
		cli.Log.Warnf("%s: %v", chatStateUpdateParseErrorMsg, err)
		return
	}

	// Verify the number of children nodes in the received node.
	children := node.GetChildren()
	if len(children) != 1 {
		cli.Log.Warnf("%s (%d)", unexpectedChildrenErrorMsg, len(node.GetChildren()))
		return
	}

	// Parse the child node and verify the presence state.
	child := children[0]
	presence := types.ChatPresence(child.Tag)
	if presence != types.ChatPresenceComposing && presence != types.ChatPresencePaused {
		cli.Log.Warnf("%s %s", unrecognizedPresenceStateMsg, child.Tag)
	}

	media := types.ChatPresenceMedia(child.AttrGetter().OptionalString("media"))
	cli.dispatchEvent(&events.ChatPresence{
		MessageSource: source,
		State:         presence,
		Media:         media,
	})
}

func (cli *Client) handlePresence(node *waBinary.Node) {
	// Create a new Presence event instance
	var evt events.Presence
	// Extract 'from' attribute as JID
	ag := node.AttrGetter()
	evt.From = ag.JID("from")

	// Extract 'type' attribute as string
	presenceType := ag.OptionalString("type")

	// Check if the presence type is 'unavailable'
	if presenceType == "unavailable" {
		evt.Unavailable = true
	} else if presenceType != "" {
		cli.Log.Debugf("Unrecognized presence type '%s' in presence event from %s", presenceType, evt.From)
	}

	lastSeen := ag.OptionalString("last")
	if lastSeen != "" && lastSeen != "deny" {
		// Parse 'last' attribute as Unix time if available and not 'deny'
		evt.LastSeen = ag.UnixTime("last")
	}

	// Check if there were any errors during attribute extraction
	if !ag.OK() {
		cli.Log.Warnf("Error parsing presence event: %+v", ag.Errors)
	} else {
		// Dispatch the presence event
		cli.dispatchEvent(&evt)
	}
}

// SendPresence updates the user's presence status on WhatsApp.
//
// You should call this at least once after connecting so that the server has your pushname.
// Otherwise, other users will see "-" as the name.
func (cli *Client) SendPresence(state types.Presence) error {
	// Check if the pushName is set. If not, return an error.
	if len(cli.Store.PushName) == 0 {
		return ErrNoPushName
	}

	// Set the sendActiveReceipts flag to 1 if the state is available, otherwise 0.
	if state == types.PresenceAvailable {
		atomic.CompareAndSwapUint32(&cli.sendActiveReceipts, 0, 1)
	} else {
		atomic.CompareAndSwapUint32(&cli.sendActiveReceipts, 1, 0)
	}

	// Send the presence update to the WhatsApp servers.
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
//
//	cli.SendPresence(types.PresenceAvailable)
func (cli *Client) SubscribePresence(jid types.JID) error {
	// Retrieve privacy token for the specified JID
	privacyToken, err := cli.Store.PrivacyTokens.GetPrivacyToken(jid)
	if err != nil {
		return fmt.Errorf("failed to get privacy token: %w", err)
	}

	// Check if a privacy token is available

	if privacyToken == nil {
		// No privacy token available
		if cli.ErrorOnSubscribePresenceWithoutToken {
			return fmt.Errorf("%w for %v", ErrNoPrivacyToken, jid.ToNonAD())
		}

		// Log a debug message if configured not to return an error
		cli.Log.Debugf("Trying to subscribe to presence of %s without privacy token", jid)
	}

	// Prepare the presence subscription request node
	req := waBinary.Node{
		Tag: "presence",
		Attrs: waBinary.Attrs{
			"type": "subscribe",
			"to":   jid,
		},
	}

	// Add privacy token information if available
	if privacyToken != nil {
		req.Content = []waBinary.Node{{
			Tag:     "tctoken",
			Content: privacyToken.Token,
		}}
	}

	// Send the presence subscription request
	return cli.sendNode(req)
}

// SendChatPresence updates the user's typing status in a specific chat.
//
// The media parameter can be set to indicate the user is recording media (like a voice message) rather than typing a text message.
func (cli *Client) SendChatPresence(jid types.JID, state types.ChatPresence, media types.ChatPresenceMedia) error {
	// Check if the client is logged in
	ownID := cli.getOwnID()
	if ownID.IsEmpty() {
		return ErrNotLoggedIn
	}

	// Prepare the content node based on the chat presence state and media
	content := []waBinary.Node{{Tag: string(state)}}
	// Add media attribute if the presence state is composing and a media is specified
	if state == types.ChatPresenceComposing && len(media) > 0 {
		content[0].Attrs = waBinary.Attrs{
			"media": string(media),
		}
	}

	// Send the chat presence update to the specified JID.
	return cli.sendNode(waBinary.Node{
		Tag: "chatstate",
		Attrs: waBinary.Attrs{
			"from": ownID,
			"to":   jid,
		},
		Content: content,
	})
}
