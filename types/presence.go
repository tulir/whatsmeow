// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types

import (
	"fmt"
)

type Presence string

const (
	PresenceAvailable   Presence = "available"
	PresenceUnavailable Presence = "unavailable"
)

type ChatPresence string

const (
	ChatPresenceComposing ChatPresence = "composing"
	ChatPresencePaused    ChatPresence = "paused"
)

type ChatPresenceMedia string

const (
	ChatPresenceMediaText  ChatPresenceMedia = ""
	ChatPresenceMediaAudio ChatPresenceMedia = "audio"
)

// ReceiptType represents the type of a Receipt event.
type ReceiptType string

const (
	// ReceiptTypeDelivered means the message was delivered to the device (but the user might not have noticed).
	ReceiptTypeDelivered ReceiptType = ""
	// ReceiptTypeSender is sent by your other devices when a message you sent is delivered to them.
	ReceiptTypeSender ReceiptType = "sender"
	// ReceiptTypeRetry means the message was delivered to the device, but decrypting the message failed.
	ReceiptTypeRetry ReceiptType = "retry"
	// ReceiptTypeRead means the user opened the chat and saw the message.
	ReceiptTypeRead ReceiptType = "read"
	// ReceiptTypeReadSelf means the current user read a message from a different device, and has read receipts disabled in privacy settings.
	ReceiptTypeReadSelf ReceiptType = "read-self"
	// ReceiptTypePlayed means the user opened a view-once media message.
	//
	// This is dispatched for both incoming and outgoing messages when played. If the current user opened the media,
	// it means the media should be removed from all devices. If a recipient opened the media, it's just a notification
	// for the sender that the media was viewed.
	ReceiptTypePlayed ReceiptType = "played"
	// ReceiptTypePlayedSelf probably means the current user opened a view-once media message from a different device,
	// and has read receipts disabled in privacy settings.
	ReceiptTypePlayedSelf ReceiptType = "played-self"

	ReceiptTypeServerError ReceiptType = "server-error"
	ReceiptTypeInactive    ReceiptType = "inactive"
	ReceiptTypePeerMsg     ReceiptType = "peer_msg"
	ReceiptTypeHistorySync ReceiptType = "hist_sync"
)

// GoString returns the name of the Go constant for the ReceiptType value.
func (rt ReceiptType) GoString() string {
	switch rt {
	case ReceiptTypeRead:
		return "types.ReceiptTypeRead"
	case ReceiptTypeReadSelf:
		return "types.ReceiptTypeReadSelf"
	case ReceiptTypeDelivered:
		return "types.ReceiptTypeDelivered"
	case ReceiptTypePlayed:
		return "types.ReceiptTypePlayed"
	default:
		return fmt.Sprintf("types.ReceiptType(%#v)", string(rt))
	}
}
