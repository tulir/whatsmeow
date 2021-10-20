// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

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
	Timestamp   time.Time
}

// GroupInfoEvent is emitted when the metadata of a group changes.
type GroupInfoEvent struct {
	JID       waBinary.JID  // The group ID in question
	Notify    string        // Seems like a top-level type for the invite
	Sender    *waBinary.JID // The user who made the change. Doesn't seem to be present when notify=invite
	Timestamp time.Time     // The time when the change occurred

	Name     *GroupName     // Group name change
	Topic    *GroupTopic    // Group topic (description) change
	Locked   *GroupLocked   // Group locked status change (can only admins edit group info?)
	Announce *GroupAnnounce // Group announce status change (can only admins send messages?)

	PrevParticipantVersionID string
	ParticipantVersionID     string

	JoinReason string // This will be invite if the user joined via invite link

	Join  []GroupParticipant // Users who joined or were added the group
	Leave []GroupParticipant // Users who left or were removed from the group

	UnknownChanges []*waBinary.Node
}

// ContactEvent is emitted when an entry in the user's contact list is modified from another device.
type ContactEvent struct {
	JID       waBinary.JID // The contact who was modified.
	Timestamp time.Time    // The time when the modification happened.'

	Action *waProto.ContactAction // The new contact info.
}

// PinEvent is emitted when a chat is pinned or unpinned from another device.
type PinEvent struct {
	JID       waBinary.JID // The chat which was pinned or unpinned.
	Timestamp time.Time    // The time when the (un)pinning happened.

	Action *waProto.PinAction // Whether the chat is now pinned or not.
}

// StarEvent is emitted when a message is starred or unstarred from another device.
type StarEvent struct {
	ChatJID   waBinary.JID // The chat where the message was pinned.
	SenderJID waBinary.JID // In group chats, the user who sent the message (except if the message was sent by the user).
	IsFromMe  bool         // Whether the message was sent by the user.
	MessageID string       // The message which was starred or unstarred.
	Timestamp time.Time    // The time when the (un)starring happened.

	Action *waProto.StarAction // Whether the message is now starred or not.
}

// DeleteForMeEvent is emitted when a message is deleted (for the current user only) from another device.
type DeleteForMeEvent struct {
	ChatJID   waBinary.JID // The chat where the message was deleted.
	SenderJID waBinary.JID // In group chats, the user who sent the message (except if the message was sent by the user).
	IsFromMe  bool         // Whether the message was sent by the user.
	MessageID string       // The message which was deleted.
	Timestamp time.Time    // The time when the deletion happened.

	Action *waProto.DeleteMessageForMeAction // Additional information for the deletion.
}

// MuteEvent is emitted when a chat is muted or unmuted from another device.
type MuteEvent struct {
	JID       waBinary.JID // The chat which was muted or unmuted.
	Timestamp time.Time    // The time when the (un)muting happened.

	Action *waProto.MuteAction // The current mute status of the chat.
}

// ArchiveEvent is emitted when a chat is archived or unarchived from another device.
type ArchiveEvent struct {
	JID       waBinary.JID // The chat which was archived or unarchived.
	Timestamp time.Time    // The time when the (un)archiving happened.

	Action *waProto.ArchiveChatAction // The current archival status of the chat.
}

// PushNameEvent is emitted when the user's push name is changed from another device.
type PushNameEvent struct {
	Timestamp time.Time // The time when the push name was changed.

	Action *waProto.PushNameSetting // The new push name for the user.
}

// AppStateEvent is emitted directly for new data received from app state syncing.
// You should generally use the higher-level events like ContactEvent and MuteEvent.
type AppStateEvent struct {
	Index []string
	*waProto.SyncActionValue
}
