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
}

// DeviceSentMeta contains metadata from messages sent by another one of the user's own devices.
type DeviceSentMeta struct {
	DestinationJID string // The destination user. This should match the MessageInfo.Recipient field.
	Phash          string
}

// MessageInfo contains metadata about an incoming message.
type MessageInfo struct {
	MessageSource
	ID        string
	Type      string
	PushName  string
	Timestamp time.Time
	Category  string

	DeviceSentMeta *DeviceSentMeta // Metadata for direct messages sent from another one of the user's own devices.
}

// SourceString returns a log-friendly representation of who sent the message and where.
func (mi *MessageInfo) SourceString() string {
	if mi.IsGroup {
		return fmt.Sprintf("%s in %s", mi.Sender, mi.Chat)
	} else if mi.Sender != mi.Chat {
		return fmt.Sprintf("%s to %s", mi.Sender, mi.Chat)
	} else {
		return mi.Chat.String()
	}
}
