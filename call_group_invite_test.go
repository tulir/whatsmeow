// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.mau.fi/whatsmeow/voip"
)

type groupParticipantInviteCorpus struct {
	Schema          string                                 `json:"schema"`
	CapabilityCases []groupParticipantInviteCapabilityCase `json:"capability_cases"`
	RosterCases     []groupParticipantInviteRosterCase     `json:"roster_cases"`
}

type groupParticipantInviteCapabilityCase struct {
	Name              string `json:"name"`
	Device            string `json:"device"`
	CapabilityVersion uint32 `json:"capability_version"`
	CapabilityHex     string `json:"capability_hex"`
}

type groupParticipantInviteRosterCase struct {
	Name              string                                    `json:"name"`
	CallID            string                                    `json:"call_id"`
	Creator           string                                    `json:"creator"`
	Connected         bool                                      `json:"connected"`
	SelfLID           string                                    `json:"self_lid"`
	PeerLID           string                                    `json:"peer_lid"`
	SelfDevice        groupParticipantInviteVectorDevice        `json:"self_device"`
	PeerDevice        groupParticipantInviteVectorDevice        `json:"peer_device"`
	GroupParticipants []groupParticipantInviteVectorParticipant `json:"group_participants"`
	WantParticipants  []groupParticipantInviteVectorParticipant `json:"want_participants"`
}

type groupParticipantInviteVectorParticipant struct {
	JID     string                               `json:"jid"`
	State   string                               `json:"state"`
	Devices []groupParticipantInviteVectorDevice `json:"devices"`
}

type groupParticipantInviteVectorDevice struct {
	JID               string `json:"jid"`
	CapabilityVersion uint32 `json:"capability_version"`
	CapabilityHex     string `json:"capability_hex"`
}

func TestParseCallInviteDeviceCorpus(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L64
	corpus := loadGroupParticipantInviteCorpus(t)
	for _, tc := range corpus.CapabilityCases {
		t.Run(tc.Name, func(t *testing.T) {
			capability := mustInviteCapability(t, tc.CapabilityHex)
			node := waBinary.Node{
				Tag: "preaccept",
				Content: []waBinary.Node{{
					Tag:     "capability",
					Attrs:   waBinary.Attrs{"ver": "1"},
					Content: capability,
				}},
			}
			got, err := parseCallInviteDevice(mustParticipantInviteJID(t, tc.Device), &node)
			if err != nil {
				t.Fatalf("parseCallInviteDevice: %v", err)
			}
			if got.JID != mustParticipantInviteJID(t, tc.Device) {
				t.Errorf("device JID = %s, want %s", got.JID, tc.Device)
			}
			if got.CapabilityVersion != tc.CapabilityVersion {
				t.Errorf("capability version = %d, want %d", got.CapabilityVersion, tc.CapabilityVersion)
			}
			if !bytes.Equal(got.Capability, capability) {
				t.Errorf("capability mismatch")
			}
			capability[0] ^= 0xff
			if bytes.Equal(got.Capability, capability) {
				t.Error("parsed capability aliases the inbound node buffer")
			}
		})
	}
}

func TestGroupInviteRosterCorpus(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	corpus := loadGroupParticipantInviteCorpus(t)
	for _, tc := range corpus.RosterCases {
		t.Run(tc.Name, func(t *testing.T) {
			cs := &callState{
				meta:             types.BasicCallMeta{CallID: tc.CallID},
				creator:          mustParticipantInviteJID(t, tc.Creator),
				connected:        tc.Connected,
				selfLID:          mustParticipantInviteJID(t, tc.SelfLID),
				peerLID:          mustParticipantInviteJID(t, tc.PeerLID),
				inviteSelfDevice: participantInviteDeviceFromVector(t, tc.SelfDevice),
				invitePeerDevice: participantInviteDeviceFromVector(t, tc.PeerDevice),
			}
			if len(tc.GroupParticipants) > 0 {
				cs.group = &groupCallState{snapshot: types.GroupCallUpdate{
					Participants: participantInviteRosterFromVector(t, tc.GroupParticipants),
				}}
			}
			cli := &Client{calls: map[string]*callState{tc.CallID: cs}}

			creator, got, _, err := cli.groupInviteRoster(tc.CallID)
			if err != nil {
				t.Fatalf("groupInviteRoster: %v", err)
			}
			if creator != mustParticipantInviteJID(t, tc.Creator) {
				t.Errorf("creator = %s, want %s", creator, tc.Creator)
			}
			want := participantInviteRosterFromVector(t, tc.WantParticipants)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("roster = %#v, want %#v", got, want)
			}

			got[0].Devices[0].Capability[0] ^= 0xff
			if cs.group != nil && bytes.Equal(got[0].Devices[0].Capability, cs.group.snapshot.Participants[0].Devices[0].Capability) {
				t.Error("returned roster aliases canonical group state")
			}
			if cs.group == nil && bytes.Equal(got[0].Devices[0].Capability, cs.inviteSelfDevice.Capability) {
				t.Error("returned roster aliases direct invite state")
			}
		})
	}
}

func TestGroupInviteRosterRejectsInvalidCallState(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L117-L132
	self := mustParticipantInviteJID(t, "100001:14@lid")
	peer := mustParticipantInviteJID(t, "200002@lid")
	validDevice := func(jid types.JID) types.GroupCallDevice {
		return types.GroupCallDevice{
			JID:               jid,
			CapabilityVersion: 1,
			Capability:        []byte{1, 5, 247, 9, 224, 187, 83},
		}
	}
	valid := func() *callState {
		return &callState{
			creator:          self,
			connected:        true,
			selfLID:          self,
			peerLID:          peer,
			inviteSelfDevice: validDevice(self),
			invitePeerDevice: validDevice(peer),
		}
	}
	cases := []struct {
		name string
		edit func(*callState)
		want string
	}{
		{name: "not connected", edit: func(cs *callState) { cs.connected = false }, want: "call is not connected"},
		{name: "missing local device", edit: func(cs *callState) { cs.inviteSelfDevice = types.GroupCallDevice{} }, want: "local active device capability is unavailable"},
		{name: "missing peer device", edit: func(cs *callState) { cs.invitePeerDevice = types.GroupCallDevice{} }, want: "peer active device capability is unavailable"},
		{name: "empty group roster", edit: func(cs *callState) { cs.group = &groupCallState{} }, want: "group roster is empty"},
	}

	cli := &Client{calls: map[string]*callState{}}
	if _, _, _, err := cli.groupInviteRoster("UNKNOWN"); err == nil || !strings.Contains(err.Error(), "unknown call") {
		t.Fatalf("unknown-call error = %v", err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs := valid()
			tc.edit(cs)
			cli := &Client{calls: map[string]*callState{"CID": cs}}
			_, _, _, err := cli.groupInviteRoster("CID")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestInviteCallParticipantAllowsActiveVideoCall(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/36d54857c74e45ccb08f6444a32d2afa13f20be9/datasheets/group-video-reactions.md#L21-L30
	self := mustParticipantInviteJID(t, "100001:14@lid")
	peer := mustParticipantInviteJID(t, "200002@lid")
	target := mustParticipantInviteJID(t, "200003@lid")
	cs := &callState{
		meta:        types.BasicCallMeta{CallID: "CID"},
		creator:     self,
		connected:   true,
		localVideo:  true,
		remoteVideo: true,
		selfLID:     self,
		peerLID:     peer,
		inviteSelfDevice: types.GroupCallDevice{
			JID: self, CapabilityVersion: 1, Capability: []byte{1, 5, 247, 9, 224, 250, 19},
		},
		invitePeerDevice: types.GroupCallDevice{
			JID: peer, CapabilityVersion: 1, Capability: []byte{1, 5, 247, 9, 224, 250, 19},
		},
	}
	cli := &Client{
		calls: map[string]*callState{"CID": cs},
		userDevicesCache: map[types.JID]deviceCache{
			target: {devices: []types.JID{target}},
		},
		Log: waLog.Noop,
	}
	err := cli.InviteCallParticipant(context.Background(), "CID", target)
	if err == nil || !strings.Contains(err.Error(), ErrNotConnected.Error()) {
		t.Fatalf("video participant invite error = %v, want send attempt wrapping %v", err, ErrNotConnected)
	}
}

func TestInviteCallParticipantReachesSendWithoutOptimisticMutation(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	self := mustParticipantInviteJID(t, "100001:14@lid")
	peer := mustParticipantInviteJID(t, "200002@lid")
	target := mustParticipantInviteJID(t, "200003@lid")
	targetDevices := []types.JID{
		mustParticipantInviteJID(t, "200003@lid"),
		mustParticipantInviteJID(t, "200003:44@lid"),
		mustParticipantInviteJID(t, "200003:45@lid"),
		mustParticipantInviteJID(t, "200003:43@lid"),
	}
	cs := &callState{
		meta:      types.BasicCallMeta{CallID: "CID"},
		creator:   self,
		connected: true,
		selfLID:   self,
		peerLID:   peer,
		inviteSelfDevice: types.GroupCallDevice{
			JID: self, CapabilityVersion: 1, Capability: []byte{1, 5, 247, 9, 224, 187, 83},
		},
		invitePeerDevice: types.GroupCallDevice{
			JID: peer, CapabilityVersion: 1, Capability: []byte{1, 5, 255, 9, 224, 250, 27},
		},
	}
	cli := &Client{
		calls:            map[string]*callState{"CID": cs},
		userDevicesCache: map[types.JID]deviceCache{target: {devices: targetDevices}},
		Log:              waLog.Noop,
	}

	err := cli.InviteCallParticipant(context.Background(), "CID", target)
	if err == nil || !strings.Contains(err.Error(), ErrNotConnected.Error()) {
		t.Fatalf("InviteCallParticipant error = %v, want send error wrapping %v", err, ErrNotConnected)
	}
	if cs.group != nil {
		t.Fatal("send failure created optimistic group state")
	}
}

func TestInviteCallParticipantRejectsInvalidInputAndExistingParticipant(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L117-L137
	var nilClient *Client
	target := mustParticipantInviteJID(t, "200003@lid")
	if err := nilClient.InviteCallParticipant(context.Background(), "CID", target); err != ErrClientIsNil {
		t.Fatalf("nil-client error = %v, want %v", err, ErrClientIsNil)
	}

	self := mustParticipantInviteJID(t, "100001:14@lid")
	peer := mustParticipantInviteJID(t, "200002@lid")
	cs := &callState{
		creator:   self,
		connected: true,
		selfLID:   self,
		peerLID:   peer,
		inviteSelfDevice: types.GroupCallDevice{
			JID: self, CapabilityVersion: 1, Capability: []byte{1},
		},
		invitePeerDevice: types.GroupCallDevice{
			JID: peer, CapabilityVersion: 1, Capability: []byte{2},
		},
	}
	cli := &Client{calls: map[string]*callState{"CID": cs}, Log: waLog.Noop}
	cases := []struct {
		name   string
		callID string
		target types.JID
		want   string
	}{
		{name: "empty call ID", target: target, want: "call ID is required"},
		{name: "empty target", callID: "CID", want: "target is required"},
		{name: "unknown call", callID: "UNKNOWN", target: target, want: "unknown call"},
		{name: "self target", callID: "CID", target: self.ToNonAD(), want: "already belongs to the call"},
		{name: "peer target", callID: "CID", target: peer, want: "already belongs to the call"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := cli.InviteCallParticipant(context.Background(), tc.callID, tc.target)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestRingCallParticipantsSelectsDisconnectedTargetAndConnectedRoster(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/25eda415afb0f926112ca375c5892b95b4bd6f60/datasheets/voip-group-invite-offer.md#L81-L106
	// Capture group-unanswered-ringing-v2-20260726-035236 covers the
	// inviteToCall/group-update sequence.
	self := mustParticipantInviteJID(t, "156535032389744@lid")
	peer := mustParticipantInviteJID(t, "242653052539031@lid")
	target := mustParticipantInviteJID(t, "74170125783269@lid")
	participants := []types.GroupCallParticipant{
		{JID: self, State: "connected"},
		{JID: peer, State: "connected"},
		{JID: target, State: "invited"},
	}

	got, err := ringCallParticipants(participants, target)
	if err != nil {
		t.Fatalf("ringCallParticipants: %v", err)
	}
	if len(got) != 2 || got[0].JID != self || got[1].JID != peer {
		t.Fatalf("ring group_info participants = %+v, want connected self and peer", got)
	}
}

func TestRingCallParticipantsRejectsMissingAndConnectedTargets(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L117-L137
	target := mustParticipantInviteJID(t, "74170125783269@lid")
	other := mustParticipantInviteJID(t, "242653052539031@lid")
	if _, err := ringCallParticipants([]types.GroupCallParticipant{{
		JID: other, State: "connected",
	}}, target); err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("missing-target error = %v", err)
	}
	if _, err := ringCallParticipants([]types.GroupCallParticipant{{
		JID: target, State: "connected",
	}}, target); err == nil || !strings.Contains(err.Error(), "already connected") {
		t.Fatalf("connected-target error = %v", err)
	}
}

func TestRingCallParticipantReachesSendForInvitedRosterTarget(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	self := mustParticipantInviteJID(t, "156535032389744:15@lid")
	peer := mustParticipantInviteJID(t, "242653052539031@lid")
	target := mustParticipantInviteJID(t, "74170125783269@lid")
	targetDevice := mustParticipantInviteJID(t, "74170125783269:43@lid")
	cs := &callState{
		meta:      types.BasicCallMeta{CallID: "CID"},
		creator:   self,
		connected: true,
		group: &groupCallState{snapshot: types.GroupCallUpdate{
			Participants: []types.GroupCallParticipant{
				{JID: self.ToNonAD(), State: "connected"},
				{JID: peer, State: "connected"},
				{JID: target, State: "invited"},
			},
		}},
	}
	cli := &Client{
		calls: map[string]*callState{"CID": cs},
		userDevicesCache: map[types.JID]deviceCache{
			target: {devices: []types.JID{targetDevice}},
		},
		Log: waLog.Noop,
	}

	err := cli.RingCallParticipant(context.Background(), "CID", target)
	if err == nil || !strings.Contains(err.Error(), ErrNotConnected.Error()) {
		t.Fatalf("RingCallParticipant error = %v, want send attempt wrapping %v", err, ErrNotConnected)
	}
}

func TestParseCallInviteDeviceRejectsMalformedCapability(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L117-L121
	device := mustParticipantInviteJID(t, "200002@lid")
	cases := []struct {
		name string
		jid  types.JID
		node *waBinary.Node
		want string
	}{
		{name: "empty device", node: &waBinary.Node{}, want: "device is required"},
		{name: "nil node", jid: device, want: "nil node"},
		{name: "missing capability", jid: device, node: &waBinary.Node{Tag: "preaccept"}, want: "missing capability"},
		{
			name: "invalid version", jid: device,
			node: &waBinary.Node{Tag: "preaccept", Content: []waBinary.Node{{
				Tag: "capability", Attrs: waBinary.Attrs{"ver": "invalid"}, Content: []byte{1},
			}}},
			want: "parse call invite device capability",
		},
		{
			name: "overflowing version", jid: device,
			node: &waBinary.Node{Tag: "preaccept", Content: []waBinary.Node{{
				Tag: "capability", Attrs: waBinary.Attrs{"ver": "4294967296"}, Content: []byte{1},
			}}},
			want: "invalid capability version",
		},
		{
			name: "empty capability", jid: device,
			node: &waBinary.Node{Tag: "preaccept", Content: []waBinary.Node{{
				Tag: "capability", Attrs: waBinary.Attrs{"ver": "1"},
			}}},
			want: "empty capability",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseCallInviteDevice(tc.jid, tc.node)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestCapturePeerInviteDeviceStoresOwnedCapability(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	peer := mustParticipantInviteJID(t, "200002@lid")
	capability := []byte{1, 5, 255, 9, 224, 250, 27}
	node := waBinary.Node{
		Tag: "preaccept",
		Content: []waBinary.Node{{
			Tag:     "capability",
			Attrs:   waBinary.Attrs{"ver": "1"},
			Content: capability,
		}},
	}
	cs := &callState{}
	cli := &Client{calls: map[string]*callState{"CID": cs}}

	if err := cli.capturePeerInviteDevice("CID", peer, &node); err != nil {
		t.Fatalf("capturePeerInviteDevice: %v", err)
	}
	if cs.invitePeerDevice.JID != peer || cs.invitePeerDevice.CapabilityVersion != 1 {
		t.Fatalf("captured device = %+v", cs.invitePeerDevice)
	}
	if !bytes.Equal(cs.invitePeerDevice.Capability, capability) {
		t.Fatal("captured capability mismatch")
	}
	capability[0] ^= 0xff
	if bytes.Equal(cs.invitePeerDevice.Capability, capability) {
		t.Fatal("stored capability aliases the inbound node")
	}

	if err := cli.capturePeerInviteDevice("UNKNOWN", peer, &node); err == nil || !strings.Contains(err.Error(), "unknown call") {
		t.Fatalf("unknown-call error = %v", err)
	}
}

func TestPreacceptCapturesSelectedPeerInviteDevice(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	peerDevice := mustParticipantInviteJID(t, "200002:1@lid")
	creator := mustParticipantInviteJID(t, "100001:14@lid")
	cli, log, _ := routerTestClient()
	cs := &callState{}
	cli.putCall("CID", cs)
	node := waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"from": peerDevice, "t": "1", "platform": "iphone", "version": "2.26.15"},
		Content: []waBinary.Node{{
			Tag:   "preaccept",
			Attrs: waBinary.Attrs{"call-id": "CID", "call-creator": creator},
			Content: []waBinary.Node{{
				Tag:     "capability",
				Attrs:   waBinary.Attrs{"ver": "1"},
				Content: []byte{1, 5, 255, 9, 224, 250, 27},
			}},
		}},
	}

	cli.handleCallEvent(context.Background(), &node)

	if cs.invitePeerDevice.JID != peerDevice {
		t.Fatalf("captured peer device = %s, want %s", cs.invitePeerDevice.JID, peerDevice)
	}
	if cs.invitePeerDevice.CapabilityVersion != 1 || !bytes.Equal(cs.invitePeerDevice.Capability, []byte{1, 5, 255, 9, 224, 250, 27}) {
		t.Fatalf("captured peer capability = %+v", cs.invitePeerDevice)
	}
	if log.hasWarn("capture peer invite device") {
		t.Fatal("valid peer capability emitted a capture warning")
	}
}

func TestAcceptInboundOfferSeedsBothInviteDevices(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	self := mustParticipantInviteJID(t, "100001:14@lid")
	peer := mustParticipantInviteJID(t, "200002:1@lid")
	cli, _, _ := routerTestClient()
	cli.Store = &store.Device{LID: self}
	offer := waBinary.Node{
		Tag: "offer",
		Content: []waBinary.Node{{
			Tag:     "capability",
			Attrs:   waBinary.Attrs{"ver": "1"},
			Content: []byte{1, 5, 255, 9, 224, 250, 27},
		}},
	}
	meta := types.BasicCallMeta{
		From:        peer,
		CallCreator: peer,
		CallID:      "CID",
	}

	cs := cli.acceptInboundOffer(context.Background(), &offer, meta, types.CallRemoteMeta{}, []byte{1, 2, 3})

	if cs.inviteSelfDevice.JID != self || cs.inviteSelfDevice.CapabilityVersion != 1 {
		t.Fatalf("incoming self invite device = %+v", cs.inviteSelfDevice)
	}
	if !bytes.Equal(cs.inviteSelfDevice.Capability, voip.CapabilityOffer) {
		t.Fatalf("incoming self capability = %x, want %x", cs.inviteSelfDevice.Capability, voip.CapabilityOffer)
	}
	if cs.invitePeerDevice.JID != peer || cs.invitePeerDevice.CapabilityVersion != 1 {
		t.Fatalf("incoming peer invite device = %+v", cs.invitePeerDevice)
	}
	if !bytes.Equal(cs.invitePeerDevice.Capability, []byte{1, 5, 255, 9, 224, 250, 27}) {
		t.Fatalf("incoming peer capability = %x", cs.invitePeerDevice.Capability)
	}
}

func TestCallAcceptMarksOutgoingCallConnected(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	peerDevice := mustParticipantInviteJID(t, "200002:1@lid")
	cli, _, captured := routerTestClient()
	cs := &callState{}
	cli.putCall("CID", cs)
	meta := types.BasicCallMeta{CallID: "CID", From: peerDevice}

	cli.onCallAccept(meta, types.CallRemoteMeta{}, &waBinary.Node{Tag: "accept"})

	if !cs.connected {
		t.Fatal("accepted outgoing call did not open the participant-invite gate")
	}
	if cs.peerLID != peerDevice || cs.to != peerDevice {
		t.Fatalf("accepted peer routing = %s / %s, want %s", cs.peerLID, cs.to, peerDevice)
	}
	accepted := captured.filter(func(event any) bool {
		_, ok := event.(*events.CallAccept)
		return ok
	})
	if len(accepted) != 1 {
		t.Fatalf("CallAccept event count = %d, want 1", len(accepted))
	}
}

func TestMarkCallConnectedRequiresCurrentCallState(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	cli, _, _ := routerTestClient()
	current := &callState{}
	stale := &callState{}
	cli.putCall("CID", current)

	if cli.markCallConnected("CID", stale) {
		t.Fatal("stale call state opened the participant-invite gate")
	}
	if current.connected || stale.connected {
		t.Fatal("stale transition mutated call state")
	}
	if !cli.markCallConnected("CID", current) {
		t.Fatal("current call state did not open the participant-invite gate")
	}
	if !current.connected {
		t.Fatal("current call state remains disconnected")
	}
}

func loadGroupParticipantInviteCorpus(t *testing.T) groupParticipantInviteCorpus {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L20-L72
	t.Helper()
	raw, err := os.ReadFile("testdata/group_participant_invite_corpus.json")
	if err != nil {
		t.Fatalf("read group participant invite corpus: %v", err)
	}
	var corpus groupParticipantInviteCorpus
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("decode group participant invite corpus: %v", err)
	}
	if corpus.Schema != "whatsmeow.group-participant-invite-corpus.v1" {
		t.Fatalf("corpus schema = %q", corpus.Schema)
	}
	if len(corpus.CapabilityCases) != 2 || len(corpus.RosterCases) != 2 {
		t.Fatalf("corpus case counts = capability %d, roster %d", len(corpus.CapabilityCases), len(corpus.RosterCases))
	}
	return corpus
}

func participantInviteRosterFromVector(t *testing.T, vectors []groupParticipantInviteVectorParticipant) []types.GroupCallParticipant {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	t.Helper()
	participants := make([]types.GroupCallParticipant, len(vectors))
	for i, vector := range vectors {
		devices := make([]types.GroupCallDevice, len(vector.Devices))
		for j, device := range vector.Devices {
			devices[j] = participantInviteDeviceFromVector(t, device)
		}
		participants[i] = types.GroupCallParticipant{
			JID:     mustParticipantInviteJID(t, vector.JID),
			State:   vector.State,
			Devices: devices,
		}
	}
	return participants
}

func participantInviteDeviceFromVector(t *testing.T, vector groupParticipantInviteVectorDevice) types.GroupCallDevice {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	t.Helper()
	return types.GroupCallDevice{
		JID:               mustParticipantInviteJID(t, vector.JID),
		CapabilityVersion: vector.CapabilityVersion,
		Capability:        mustInviteCapability(t, vector.CapabilityHex),
	}
}

func mustInviteCapability(t *testing.T, raw string) []byte {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L64
	t.Helper()
	value, err := hex.DecodeString(raw)
	if err != nil {
		t.Fatalf("decode capability %q: %v", raw, err)
	}
	return value
}

func mustParticipantInviteJID(t *testing.T, raw string) types.JID {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	t.Helper()
	jid, err := types.ParseJID(raw)
	if err != nil {
		t.Fatalf("parse JID %q: %v", raw, err)
	}
	return jid
}
