// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package events

import (
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

// CallOffer is emitted when the user receives a call on WhatsApp.
type CallOffer struct {
	types.BasicCallMeta
	types.CallRemoteMeta

	Data *waBinary.Node // The call offer data
}

// CallAccept is emitted when a call is accepted on WhatsApp.
type CallAccept struct {
	types.BasicCallMeta
	types.CallRemoteMeta

	Data *waBinary.Node
}

type CallPreAccept struct {
	types.BasicCallMeta
	types.CallRemoteMeta

	Data *waBinary.Node
}

type CallTransport struct {
	types.BasicCallMeta
	types.CallRemoteMeta

	Data *waBinary.Node
}

// CallOfferNotice is emitted when the user receives a notice of a call on WhatsApp.
// This seems to be primarily for group calls (whereas CallOffer is for 1:1 calls).
type CallOfferNotice struct {
	types.BasicCallMeta

	Media string // "audio" or "video" depending on call type
	Type  string // "group" when it's a group call

	Data *waBinary.Node
}

// CallRelayLatency is emitted slightly after the user receives a call on WhatsApp.
type CallRelayLatency struct {
	types.BasicCallMeta
	Data *waBinary.Node
}

// CallTerminate is emitted when the other party terminates a call on WhatsApp.
type CallTerminate struct {
	types.BasicCallMeta
	Reason string
	Data   *waBinary.Node
}

// UnknownCallEvent is emitted when a call element with unknown content is received.
type UnknownCallEvent struct {
	Node *waBinary.Node
}
