// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package events

import (
	"time"

	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/types"
)

// Contact is emitted when an entry in the user's contact list is modified from another device.
type Contact struct {
	JID       types.JID // The contact who was modified.
	Timestamp time.Time // The time when the modification happened.'

	Action       *waSyncAction.ContactAction // The new contact info.
	FromFullSync bool                        // Whether the action is emitted because of a fullSync
}

// PushName is emitted when a message is received with a different push name than the previous value cached for the same user.
type PushName struct {
	JID         types.JID // The user whose push name changed.
	JIDAlt      types.JID
	Message     *types.MessageInfo // The message where this change was first noticed.
	OldPushName string             // The previous push name from the local cache.
	NewPushName string             // The new push name that was included in the message.
}

// BusinessName is emitted when a message is received with a different verified business name than the previous value cached for the same user.
type BusinessName struct {
	JID             types.JID
	Message         *types.MessageInfo // This is only present if the change was detected in a message.
	OldBusinessName string
	NewBusinessName string
}

// Pin is emitted when a chat is pinned or unpinned from another device.
type Pin struct {
	JID       types.JID // The chat which was pinned or unpinned.
	Timestamp time.Time // The time when the (un)pinning happened.

	Action       *waSyncAction.PinAction // Whether the chat is now pinned or not.
	FromFullSync bool                    // Whether the action is emitted because of a fullSync
}

// Star is emitted when a message is starred or unstarred from another device.
type Star struct {
	ChatJID   types.JID // The chat where the message was pinned.
	SenderJID types.JID // In group chats, the user who sent the message (except if the message was sent by the user).
	IsFromMe  bool      // Whether the message was sent by the user.
	MessageID string    // The message which was starred or unstarred.
	Timestamp time.Time // The time when the (un)starring happened.

	Action       *waSyncAction.StarAction // Whether the message is now starred or not.
	FromFullSync bool                     // Whether the action is emitted because of a fullSync
}

// DeleteForMe is emitted when a message is deleted (for the current user only) from another device.
type DeleteForMe struct {
	ChatJID   types.JID // The chat where the message was deleted.
	SenderJID types.JID // In group chats, the user who sent the message (except if the message was sent by the user).
	IsFromMe  bool      // Whether the message was sent by the user.
	MessageID string    // The message which was deleted.
	Timestamp time.Time // The time when the deletion happened.

	Action       *waSyncAction.DeleteMessageForMeAction // Additional information for the deletion.
	FromFullSync bool                                   // Whether the action is emitted because of a fullSync
}

// Mute is emitted when a chat is muted or unmuted from another device.
type Mute struct {
	JID       types.JID // The chat which was muted or unmuted.
	Timestamp time.Time // The time when the (un)muting happened.

	Action       *waSyncAction.MuteAction // The current mute status of the chat.
	FromFullSync bool                     // Whether the action is emitted because of a fullSync
}

// Archive is emitted when a chat is archived or unarchived from another device.
type Archive struct {
	JID       types.JID // The chat which was archived or unarchived.
	Timestamp time.Time // The time when the (un)archiving happened.

	Action       *waSyncAction.ArchiveChatAction // The current archival status of the chat.
	FromFullSync bool                            // Whether the action is emitted because of a fullSync
}

// MarkChatAsRead is emitted when a whole chat is marked as read or unread from another device.
type MarkChatAsRead struct {
	JID       types.JID // The chat which was marked as read or unread.
	Timestamp time.Time // The time when the marking happened.

	Action       *waSyncAction.MarkChatAsReadAction // Whether the chat was marked as read or unread, and info about the most recent messages.
	FromFullSync bool                               // Whether the action is emitted because of a fullSync
}

// ClearChat is emitted when a chat is cleared on another device. This is different from DeleteChat.
type ClearChat struct {
	JID       types.JID // The chat which was cleared.
	Timestamp time.Time // The time when the clear happened.

	Action       *waSyncAction.ClearChatAction // Information about the clear.
	FromFullSync bool                          // Whether the action is emitted because of a fullSync
	DeleteMedia  bool
}

// DeleteChat is emitted when a chat is deleted on another device.
type DeleteChat struct {
	JID       types.JID // The chat which was deleted.
	Timestamp time.Time // The time when the deletion happened.

	Action       *waSyncAction.DeleteChatAction // Information about the deletion.
	FromFullSync bool                           // Whether the action is emitted because of a fullSync
	DeleteMedia  bool
}

// PushNameSetting is emitted when the user's push name is changed from another device.
type PushNameSetting struct {
	Timestamp time.Time // The time when the push name was changed.

	Action       *waSyncAction.PushNameSetting // The new push name for the user.
	FromFullSync bool                          // Whether the action is emitted because of a fullSync
}

// UnarchiveChatsSetting is emitted when the user changes the "Keep chats archived" setting from another device.
type UnarchiveChatsSetting struct {
	Timestamp time.Time // The time when the setting was changed.

	Action       *waSyncAction.UnarchiveChatsSetting // The new settings.
	FromFullSync bool                                // Whether the action is emitted because of a fullSync
}

// UserStatusMute is emitted when the user mutes or unmutes another user's status updates.
type UserStatusMute struct {
	JID       types.JID // The user who was muted or unmuted
	Timestamp time.Time // The timestamp when the action happened

	Action       *waSyncAction.UserStatusMuteAction // The new mute status
	FromFullSync bool                               // Whether the action is emitted because of a fullSync
}

// LabelEdit is emitted when a label is edited from any device.
type LabelEdit struct {
	Timestamp time.Time // The time when the label was edited.
	LabelID   string    // The label id which was edited.

	Action       *waSyncAction.LabelEditAction // The new label info.
	FromFullSync bool                          // Whether the action is emitted because of a fullSync
}

// LabelAssociationChat is emitted when a chat is labeled or unlabeled from any device.
type LabelAssociationChat struct {
	JID       types.JID // The chat which was labeled or unlabeled.
	Timestamp time.Time // The time when the (un)labeling happened.
	LabelID   string    // The label id which was added or removed.

	Action       *waSyncAction.LabelAssociationAction // The current label status of the chat.
	FromFullSync bool                                 // Whether the action is emitted because of a fullSync
}

// LabelAssociationMessage is emitted when a message is labeled or unlabeled from any device.
type LabelAssociationMessage struct {
	JID       types.JID // The chat which was labeled or unlabeled.
	Timestamp time.Time // The time when the (un)labeling happened.
	LabelID   string    // The label id which was added or removed.
	MessageID string    // The message id which was labeled or unlabeled.

	Action       *waSyncAction.LabelAssociationAction // The current label status of the message.
	FromFullSync bool                                 // Whether the action is emitted because of a fullSync
}

// AppState is emitted directly for new data received from app state syncing.
// You should generally use the higher-level events like events.Contact and events.Mute.
type AppState struct {
	Index []string
	*waSyncAction.SyncActionValue
}

// AppStateSyncComplete is emitted when app state is resynced.
type AppStateSyncComplete struct {
	Name     appstate.WAPatchName
	Version  uint64
	Recovery bool
}

type AppStateSyncError struct {
	Name     appstate.WAPatchName
	Error    error
	FullSync bool
}
