// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types

import "time"

type BasicCallMeta struct {
	From           JID
	Timestamp      time.Time
	CallCreator    JID
	CallCreatorAlt JID
	CallID         string
	GroupJID       JID
	Participants   []CallParticipant
}

// CallParticipant describes a participant reported in a group call event.
type CallParticipant struct {
	JID         JID
	PhoneNumber JID // The server supplied alternative phone number, when available.
	Username    string
	State       string
}

type CallRemoteMeta struct {
	RemotePlatform string // The platform of the caller's WhatsApp client
	RemoteVersion  string // Version of the caller's WhatsApp client
}
