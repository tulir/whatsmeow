// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsapp

import (
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
)

// QREvent is emitted after connecting when there's no session data in the device store.
//
// The QR codes are available in the Codes slice. You should render the strings as QR codes one by
// one, switching to the next one whenever the duration specified in the Timeout field has passed.
//
// When the QR code has been scanned and pairing is complete, PairSuccessEvent will be emitted. If
// you run out of codes before scanning, the server will close the websocket, and you will have to
// reconnect to get more codes.
type QREvent struct {
	Codes   []string
	Timeout time.Duration
}

// PairSuccessEvent is emitted after the QR code has been scanned with the phone and the handshake
// has been completed. Note that this is generally followed by a websocket reconnection, so you
// should wait for the ConnectedEvent before trying to send anything.
type PairSuccessEvent struct {
	ID           waBinary.JID
	BusinessName string
	Platform     string
}

// ConnectedEvent is emitted when the client has successfully connected to the WhatsApp servers
// and is authenticated. The user who the client is authenticated as will be in the device store
// at this point, which is why this event doesn't contain any data.
type ConnectedEvent struct{}

// LoggedOutEvent is emitted when the client has been unpaired from the phone.
type LoggedOutEvent struct{}

// HistorySyncEvent is emitted when the phone has sent a blob of historical messages.
type HistorySyncEvent struct {
	Data *waProto.HistorySync
}

// DeviceSentMeta contains metadata from messages sent by another one of the user's own devices.
type DeviceSentMeta struct {
	DestinationJID string // The destination user. This should match the MessageInfo.Recipient field.
	Phash          string
}

// MessageEvent is emitted when receiving a new message.
type MessageEvent struct {
	Info           *MessageInfo     // Information about the message like the chat and sender IDs
	Message        *waProto.Message // The actual message struct
	DeviceSentMeta *DeviceSentMeta  // Metadata for direct messages sent from another one of the user's own devices.
	IsEphemeral    bool
	IsViewOnce     bool

	// The raw message struct. This is the raw unwrapped data, which means the actual message might
	// be wrapped in DeviceSentMessage, EphemeralMessage or ViewOnceMessage.
	RawMessage *waProto.Message
}

// ReadReceiptEvent is emitted when someone reads a message sent by the user.
type ReadReceiptEvent struct {
	From        waBinary.JID
	Chat        *waBinary.JID
	Recipient   *waBinary.JID
	MessageID   string
	PreviousIDs []string
	Timestamp   int64
}
