// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package events

import (
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	waBinary "go.mau.fi/whatsmeow/types"
)

// Contact is emitted when an entry in the user's contact list is modified from another device.
type Contact struct {
	JID       waBinary.JID // The contact who was modified.
	Timestamp time.Time    // The time when the modification happened.'

	Action *waProto.ContactAction // The new contact info.
}

// Pin is emitted when a chat is pinned or unpinned from another device.
type Pin struct {
	JID       waBinary.JID // The chat which was pinned or unpinned.
	Timestamp time.Time    // The time when the (un)pinning happened.

	Action *waProto.PinAction // Whether the chat is now pinned or not.
}

// Star is emitted when a message is starred or unstarred from another device.
type Star struct {
	ChatJID   waBinary.JID // The chat where the message was pinned.
	SenderJID waBinary.JID // In group chats, the user who sent the message (except if the message was sent by the user).
	IsFromMe  bool         // Whether the message was sent by the user.
	MessageID string       // The message which was starred or unstarred.
	Timestamp time.Time    // The time when the (un)starring happened.

	Action *waProto.StarAction // Whether the message is now starred or not.
}

// DeleteForMe is emitted when a message is deleted (for the current user only) from another device.
type DeleteForMe struct {
	ChatJID   waBinary.JID // The chat where the message was deleted.
	SenderJID waBinary.JID // In group chats, the user who sent the message (except if the message was sent by the user).
	IsFromMe  bool         // Whether the message was sent by the user.
	MessageID string       // The message which was deleted.
	Timestamp time.Time    // The time when the deletion happened.

	Action *waProto.DeleteMessageForMeAction // Additional information for the deletion.
}

// Mute is emitted when a chat is muted or unmuted from another device.
type Mute struct {
	JID       waBinary.JID // The chat which was muted or unmuted.
	Timestamp time.Time    // The time when the (un)muting happened.

	Action *waProto.MuteAction // The current mute status of the chat.
}

// Archive is emitted when a chat is archived or unarchived from another device.
type Archive struct {
	JID       waBinary.JID // The chat which was archived or unarchived.
	Timestamp time.Time    // The time when the (un)archiving happened.

	Action *waProto.ArchiveChatAction // The current archival status of the chat.
}

// PushName is emitted when the user's push name is changed from another device.
type PushName struct {
	Timestamp time.Time // The time when the push name was changed.

	Action *waProto.PushNameSetting // The new push name for the user.
}

// AppState is emitted directly for new data received from app state syncing.
// You should generally use the higher-level events like events.Contact and events.Mute.
type AppState struct {
	Index []string
	*waProto.SyncActionValue
}
