// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package events contains all the events that whatsmeow.Client emits to functions registered with AddEventHandler.
package events

import (
	"fmt"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/structs"
)

// QR is emitted after connecting when there's no session data in the device store.
//
// The QR codes are available in the Codes slice. You should render the strings as QR codes one by
// one, switching to the next one whenever the duration specified in the Timeout field has passed.
//
// When the QR code has been scanned and pairing is complete, PairSuccess will be emitted. If you
// run out of codes before scanning, the server will close the websocket, and you will have to
// reconnect to get more codes.
type QR struct {
	Codes   []string
	Timeout time.Duration
}

// PairSuccess is emitted after the QR code has been scanned with the phone and the handshake has
// been completed. Note that this is generally followed by a websocket reconnection, so you should
// wait for the Connected before trying to send anything.
type PairSuccess struct {
	ID           waBinary.JID
	BusinessName string
	Platform     string
}

// Connected is emitted when the client has successfully connected to the WhatsApp servers
// and is authenticated. The user who the client is authenticated as will be in the device store
// at this point, which is why this event doesn't contain any data.
type Connected struct{}

// LoggedOut is emitted when the client has been unpaired from the phone.
type LoggedOut struct{}

// HistorySync is emitted when the phone has sent a blob of historical messages.
type HistorySync struct {
	Data *waProto.HistorySync
}

// Message is emitted when receiving a new message.
type Message struct {
	Info        structs.MessageInfo // Information about the message like the chat and sender IDs
	Message     *waProto.Message    // The actual message struct
	IsEphemeral bool
	IsViewOnce  bool

	// The raw message struct. This is the raw unwrapped data, which means the actual message might
	// be wrapped in DeviceSentMessage, EphemeralMessage or ViewOnceMessage.
	RawMessage *waProto.Message
}

type ReceiptType string

const (
	ReceiptTypeDelivered ReceiptType = ""
	ReceiptTypeRead      ReceiptType = "read"
)

func (rt ReceiptType) GoString() string {
	switch rt {
	case ReceiptTypeRead:
		return "ReceiptTypeRead"
	case ReceiptTypeDelivered:
		return "ReceiptTypeDelivered"
	default:
		return fmt.Sprintf("ReceiptType(%#v)", string(rt))
	}
}

// Receipt is emitted when an outgoing message is delivered to or read by another user, or when another device reads an incoming message.
type Receipt struct {
	structs.MessageSource
	MessageID   string
	Timestamp   time.Time
	Type        ReceiptType
	PreviousIDs []string // Additional message IDs that were read. Only present for read receipts.
}

// GroupInfo is emitted when the metadata of a group changes.
type GroupInfo struct {
	JID       waBinary.JID  // The group ID in question
	Notify    string        // Seems like a top-level type for the invite
	Sender    *waBinary.JID // The user who made the change. Doesn't seem to be present when notify=invite
	Timestamp time.Time     // The time when the change occurred

	Name     *structs.GroupName     // Group name change
	Topic    *structs.GroupTopic    // Group topic (description) change
	Locked   *structs.GroupLocked   // Group locked status change (can only admins edit group info?)
	Announce *structs.GroupAnnounce // Group announce status change (can only admins send messages?)

	PrevParticipantVersionID string
	ParticipantVersionID     string

	JoinReason string // This will be invite if the user joined via invite link

	Join  []structs.GroupParticipant // Users who joined or were added the group
	Leave []structs.GroupParticipant // Users who left or were removed from the group

	UnknownChanges []*waBinary.Node
}
