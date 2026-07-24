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

type CallOffer struct {
	types.BasicCallMeta
	types.CallRemoteMeta

	Data  *waBinary.Node
	Video bool
}

type CallAccept struct {
	types.BasicCallMeta
	types.CallRemoteMeta

	Data    *waBinary.Node
	PeerLID types.JID
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

type CallOfferNotice struct {
	types.BasicCallMeta

	Media string
	Type  string

	Data *waBinary.Node
}

type CallRelayLatency struct {
	types.BasicCallMeta
	Data *waBinary.Node
}

type CallTerminate struct {
	types.BasicCallMeta
	Reason string
	Data   *waBinary.Node
}

type CallReject struct {
	types.BasicCallMeta
	Data *waBinary.Node
}

type UnknownCallEvent struct {
	Node *waBinary.Node
}

type CallMediaReady struct {
	types.BasicCallMeta

	SelfLID types.JID
	PeerLID types.JID

	CallKey   []byte
	Relay     types.RelayEndpoint
	Codec     types.CallCodec
	Direction types.CallDirection
	Video     bool
}

type CallMediaStop struct {
	types.BasicCallMeta

	Reason string
}

type CallMute struct {
	types.BasicCallMeta

	Muted bool
}

type CallAck struct {
	types.BasicCallMeta

	Data *waBinary.Node
}

// CallVideo describes one independent remote video-flow transition.
type CallVideo struct {
	types.BasicCallMeta

	State          types.CallVideoState
	Orientation    int
	HasOrientation bool
	Data           *waBinary.Node
}
