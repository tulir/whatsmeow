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
}

type CallRemoteMeta struct {
	RemotePlatform string
	RemoteVersion  string
}

// CallLinkMedia identifies the media mode encoded in a WhatsApp call link.
type CallLinkMedia string

const (
	CallLinkMediaAudio CallLinkMedia = "audio"
	CallLinkMediaVideo CallLinkMedia = "video"
)

// CallLink is a reusable token returned by the call-link service.
type CallLink struct {
	Token string
	Media CallLinkMedia
}

// CallLinkPreview is the service metadata returned before joining a link.
type CallLinkPreview struct {
	Token              string
	Media              CallLinkMedia
	Creator            JID
	CreatorPN          JID
	WaitingRoomEnabled bool
	IsAdmin            bool
}

// CallLinkJoin is the result of joining a reusable link.
type CallLinkJoin struct {
	Token              string
	Media              CallLinkMedia
	CallID             string
	CallCreator        JID
	WaitingRoomEnabled bool
	InWaitingRoom      bool
	IsAdmin            bool
	Group              *GroupCallUpdate
}

// CallLinkWaitingRoom is one authoritative waiting-room snapshot.
type CallLinkWaitingRoom struct {
	CallID        string
	CallCreator   JID
	LinkToken     string
	Media         CallLinkMedia
	Enabled       bool
	IsAdmin       bool
	TransactionID uint32
	Users         []CallLinkWaitingRoomUser
}

// CallLinkWaitingRoomUser is one user in a waiting-room snapshot.
type CallLinkWaitingRoomUser struct {
	JID   JID
	PN    JID
	State string
}

// CallDirection identifies which side originated a 1:1 call.
type CallDirection uint8

const (
	CallDirectionIncoming CallDirection = iota
	CallDirectionOutgoing
)

// CallVideoState is the state value carried by an in-call video stanza. Local and
// remote video flows are independent: stopping one does not stop the other.
type CallVideoState int

const (
	CallVideoStateDisabled         CallVideoState = 0
	CallVideoStateEnabled          CallVideoState = 1
	CallVideoStateUpgradeRequest   CallVideoState = 3
	CallVideoStateUpgradeAccept    CallVideoState = 4
	CallVideoStateUpgradeReject    CallVideoState = 5
	CallVideoStateStopped          CallVideoState = 6
	CallVideoStateUpgradeCancel    CallVideoState = 8
	CallVideoStateUpgradeRequestV2 CallVideoState = 11
)

// CallScreenShareState identifies one independent screen-share transition.
type CallScreenShareState int

const (
	CallScreenShareStateStarted CallScreenShareState = 1
	CallScreenShareStateStopped CallScreenShareState = 2
)

// CallScreenShare is the parsed state of one participant's screen-share stream.
type CallScreenShare struct {
	State            CallScreenShareState
	Version          uint32
	ScreenShareID    uint32
	HasScreenShareID bool
}

// CallCodec identifies which media codec a 1:1 call negotiated.
type CallCodec uint8

const (
	CallCodecMLow CallCodec = iota
	CallCodecOpus
)

// String returns a human-readable name for the codec.
func (c CallCodec) String() string {
	switch c {
	case CallCodecMLow:
		return "mlow"
	case CallCodecOpus:
		return "opus"
	default:
		return "unknown"
	}
}

// RelayEndpoint is the elected media relay for a 1:1 call.
type RelayEndpoint struct {
	RelayID     uint32
	TokenID     uint32
	AuthTokenID uint32
	RelayName   string
	IsFNA       bool
	IPv4        string
	Port        uint16

	Key       []byte
	Token     []byte
	AuthToken []byte
}

// GroupCallUpdate is a server snapshot of group-call membership and transport state.
type GroupCallUpdate struct {
	CallID         string
	CallCreator    JID
	GroupJID       JID
	TransactionID  uint32
	Media          string
	ConnectedLimit uint32
	Joinable       bool
	AVUpgradable   bool
	RekeyRequested bool
	Participants   []GroupCallParticipant
	Relay          *GroupCallRelay
}

// GroupCallEncRekey is one encrypted shared key epoch for a group-call transaction.
type GroupCallEncRekey struct {
	CallID            string
	CallCreator       JID
	TransactionID     uint32
	KeyGeneration     uint32
	EncryptionType    string
	EncryptionVersion uint32
	Ciphertext        []byte
}

// GroupCallParticipant is one user in a group-call roster snapshot.
type GroupCallParticipant struct {
	JID     JID
	PN      JID
	State   string
	Type    string
	Devices []GroupCallDevice
}

// GroupCallDevice is one participant device advertised in a group-call roster.
type GroupCallDevice struct {
	JID               JID
	Platform          string
	PID               uint32
	HasPID            bool
	CapabilityVersion uint32
	Capability        []byte
}

// GroupCallRelay is the relay allocation attached to a group-call update.
type GroupCallRelay struct {
	TransactionID      uint32
	SelfPID            uint32
	HasSelfPID         bool
	UUID               string
	ParticipantUUID    string
	AttributePadding   bool
	WarpMITagLength    uint32
	HasWarpMITagLength bool
	Key                []byte
	HBHKey             []byte
	Tokens             [][]byte
	AuthTokens         [][]byte
	Endpoints          []GroupCallRelayEndpoint
}

// GroupCallRelayEndpoint is one address record in a group-call relay allocation.
type GroupCallRelayEndpoint struct {
	RelayID     uint32
	TokenID     uint32
	AuthTokenID uint32
	RelayName   string
	DomainName  string
	RTT         uint32
	IsFNA       bool
	Address     []byte
}
