// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types

import (
	"fmt"
	"time"
)

// MessageSource contains basic sender and chat information about a message.
type MessageSource struct {
	Chat     JID  // The chat where the message was sent.
	Sender   JID  // The user who sent the message.
	IsFromMe bool // Whether the message was sent by the current user instead of someone else.
	IsGroup  bool // Whether the chat is a group chat or broadcast list.

	// When sending a read receipt to a broadcast list message, the Chat is the broadcast list
	// and Sender is you, so this field contains the recipient of the read receipt.
	BroadcastListOwner JID
}

// IsIncomingBroadcast returns true if the message was sent to a broadcast list instead of directly to the user.
//
// If this is true, it means the message shows up in the direct chat with the Sender.
func (ms *MessageSource) IsIncomingBroadcast() bool {
	return (!ms.IsFromMe || !ms.BroadcastListOwner.IsEmpty()) && ms.Chat.IsBroadcastList()
}

// DeviceSentMeta contains metadata from messages sent by another one of the user's own devices.
type DeviceSentMeta struct {
	DestinationJID string // The destination user. This should match the MessageInfo.Recipient field.
	Phash          string
}

type EditAttribute string

const (
	EditAttributeEmpty        EditAttribute = ""
	EditAttributeMessageEdit  EditAttribute = "1"
	EditAttributePinInChat    EditAttribute = "2"
	EditAttributeAdminEdit    EditAttribute = "3" // only used in newsletters
	EditAttributeSenderRevoke EditAttribute = "7"
	EditAttributeAdminRevoke  EditAttribute = "8"
)

// MessageInfo contains metadata about an incoming message.
type MessageInfo struct {
	MessageSource
	ID        MessageID
	ServerID  MessageServerID
	Type      string
	PushName  string
	Timestamp time.Time
	Category  string
	Multicast bool
	MediaType string
	Edit      EditAttribute

	VerifiedName   *VerifiedName
	DeviceSentMeta *DeviceSentMeta // Metadata for direct messages sent from another one of the user's own devices.
}

// SourceString returns a log-friendly representation of who sent the message and where.
func (ms *MessageSource) SourceString() string {
	if ms.Sender != ms.Chat {
		return fmt.Sprintf("%s in %s", ms.Sender, ms.Chat)
	} else {
		return ms.Chat.String()
	}
}
