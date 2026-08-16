// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package voip

import (
	"bytes"
	"strconv"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func TestParseGroupUpdateVideoRosterAndRelay(t *testing.T) {
	creator := types.JID{User: "156535032389744", Device: 14, Server: types.HiddenUserServer}
	group := types.NewJID("120363411251996986", types.GroupServer)
	android := types.NewJID("74170125783269", types.HiddenUserServer)
	iphone := types.NewJID("242653052539031", types.HiddenUserServer)
	web := creator
	node := waBinary.Node{
		Tag: "group_update",
		Attrs: waBinary.Attrs{
			"call-creator": creator,
			"call-id":      "00789FDA2E6B8842B8EFE01ABD5F5B3A",
		},
		Content: []waBinary.Node{
			{
				Tag: "group_info",
				Attrs: waBinary.Attrs{
					"group-jid":       group,
					"call-creator":    creator,
					"call-id":         "00789FDA2E6B8842B8EFE01ABD5F5B3A",
					"transaction-id":  "22",
					"media":           "video",
					"connected-limit": "32",
				},
				Content: []waBinary.Node{
					groupUser(android, "connected", "android", 2),
					groupUser(iphone, "connected", "iphone", 1),
					groupUser(web, "connected", "web", 0),
				},
			},
			{Tag: "av_upgrade", Attrs: waBinary.Attrs{"av-upgradable": "1"}},
			{
				Tag: "relay",
				Attrs: waBinary.Attrs{
					"attribute_padding": "1",
					"transaction-id":    "3",
					"warp_mi_tag_len":   "4",
					"self_pid":          "0",
					"uuid":              "relay-call-uuid",
					"participant_uuid":  "self-participant-uuid",
				},
				Content: []waBinary.Node{
					{Tag: "token", Attrs: waBinary.Attrs{"id": "0"}, Content: []byte("token-zero")},
					{Tag: "token", Attrs: waBinary.Attrs{"id": "2"}, Content: []byte("token-two")},
					{Tag: "auth_token", Attrs: waBinary.Attrs{"id": "0"}, Content: []byte("auth-zero")},
					{Tag: "key", Content: []byte("relay-key")},
					{
						Tag: "te2",
						Attrs: waBinary.Attrs{
							"auth_token_id": "0",
							"domain_name":   "edgeray-zrh1-1.wt.whatsapp.com",
							"relay_name":    "zrh1c01",
							"relay_id":      "0",
							"token_id":      "2",
						},
						Content: []byte{157, 240, 17, 133, 13, 150},
					},
					{Tag: "hbh_key", Content: []byte("hop-by-hop-key")},
				},
			},
		},
	}

	update, err := ParseGroupUpdate(&node)
	if err != nil {
		t.Fatalf("ParseGroupUpdate: %v", err)
	}
	if update.CallID != "00789FDA2E6B8842B8EFE01ABD5F5B3A" {
		t.Errorf("CallID = %q", update.CallID)
	}
	if update.CallCreator != creator || update.GroupJID != group {
		t.Errorf("call identity = %s / %s, want %s / %s", update.CallCreator, update.GroupJID, creator, group)
	}
	if update.TransactionID != 22 || update.Media != "video" || update.ConnectedLimit != 32 {
		t.Errorf("group state = transaction %d, media %q, limit %d", update.TransactionID, update.Media, update.ConnectedLimit)
	}
	if !update.AVUpgradable {
		t.Error("AVUpgradable = false, want true")
	}
	if len(update.Participants) != 3 {
		t.Fatalf("len(Participants) = %d, want 3", len(update.Participants))
	}
	for i, wantPID := range []uint32{2, 1, 0} {
		devices := update.Participants[i].Devices
		if len(devices) != 1 {
			t.Fatalf("participant %d has %d devices, want 1", i, len(devices))
		}
		if !devices[0].HasPID || devices[0].PID != wantPID {
			t.Errorf("participant %d PID = %d (present %t), want %d", i, devices[0].PID, devices[0].HasPID, wantPID)
		}
		if devices[0].Platform == "" {
			t.Errorf("participant %d platform is empty", i)
		}
		if !bytes.Equal(devices[0].Capability, []byte{1, 5, 247, 9, 224, 250, 19}) {
			t.Errorf("participant %d capability = %x", i, devices[0].Capability)
		}
	}
	if update.Relay == nil {
		t.Fatal("Relay = nil")
	}
	if update.Relay.TransactionID != 3 || update.Relay.SelfPID != 0 || !update.Relay.HasSelfPID {
		t.Errorf("relay transaction/self PID = %d/%d (present %t)", update.Relay.TransactionID, update.Relay.SelfPID, update.Relay.HasSelfPID)
	}
	if update.Relay.WarpMITagLength != 4 || !update.Relay.HasWarpMITagLength {
		t.Errorf("relay WARP MI tag length = %d (present %t)", update.Relay.WarpMITagLength, update.Relay.HasWarpMITagLength)
	}
	if update.Relay.UUID != "relay-call-uuid" || update.Relay.ParticipantUUID != "self-participant-uuid" {
		t.Errorf("relay UUIDs = %q / %q", update.Relay.UUID, update.Relay.ParticipantUUID)
	}
	if !bytes.Equal(update.Relay.Key, []byte("relay-key")) || !bytes.Equal(update.Relay.HBHKey, []byte("hop-by-hop-key")) {
		t.Error("relay key material was not preserved")
	}
	if len(update.Relay.Tokens) != 3 || !bytes.Equal(update.Relay.Tokens[2], []byte("token-two")) {
		t.Errorf("relay tokens = %#v", update.Relay.Tokens)
	}
	if len(update.Relay.AuthTokens) != 1 || !bytes.Equal(update.Relay.AuthTokens[0], []byte("auth-zero")) {
		t.Errorf("relay auth tokens = %#v", update.Relay.AuthTokens)
	}
	if len(update.Relay.Endpoints) != 1 {
		t.Fatalf("len(Relay.Endpoints) = %d, want 1", len(update.Relay.Endpoints))
	}
	endpoint := update.Relay.Endpoints[0]
	if endpoint.RelayName != "zrh1c01" || endpoint.TokenID != 2 || !bytes.Equal(endpoint.Address, []byte{157, 240, 17, 133, 13, 150}) {
		t.Errorf("relay endpoint = %+v", endpoint)
	}
}

func TestParseGroupUpdateAllowsAdHocGroupWithoutGroupJID(t *testing.T) {
	creator := types.JID{User: "156535032389744", Device: 14, Server: types.HiddenUserServer}
	peer := types.NewJID("242653052539031", types.HiddenUserServer)
	node := waBinary.Node{
		Tag: "group_update",
		Attrs: waBinary.Attrs{
			"call-creator": creator,
			"call-id":      "00DD63A26643DC3496FCBD161E6E2AB1",
		},
		Content: []waBinary.Node{{
			Tag: "group_info",
			Attrs: waBinary.Attrs{
				"call-creator":    creator,
				"call-id":         "00DD63A26643DC3496FCBD161E6E2AB1",
				"transaction-id":  "21",
				"media":           "audio",
				"connected-limit": "32",
			},
			Content: []waBinary.Node{groupUser(peer, "connected", "iphone", 1)},
		}},
	}

	update, err := ParseGroupUpdate(&node)
	if err != nil {
		t.Fatalf("ParseGroupUpdate: %v", err)
	}
	if !update.GroupJID.IsEmpty() {
		t.Errorf("GroupJID = %s, want empty for ad-hoc group", update.GroupJID)
	}
	if update.TransactionID != 21 || len(update.Participants) != 1 {
		t.Errorf("group state = transaction %d with %d participants", update.TransactionID, len(update.Participants))
	}
}

func TestParseGroupUpdateRejectsOverflowingTransactionID(t *testing.T) {
	creator := types.JID{User: "156535032389744", Device: 14, Server: types.HiddenUserServer}
	node := waBinary.Node{
		Tag:   "group_update",
		Attrs: waBinary.Attrs{"call-creator": creator, "call-id": "call-id"},
		Content: []waBinary.Node{{
			Tag: "group_info",
			Attrs: waBinary.Attrs{
				"group-jid":       types.NewJID("120363411251996986", types.GroupServer),
				"transaction-id":  "4294967296",
				"media":           "audio",
				"connected-limit": "32",
			},
		}},
	}

	if _, err := ParseGroupUpdate(&node); err == nil {
		t.Fatal("ParseGroupUpdate accepted a transaction ID larger than uint32")
	}
}

func groupUser(jid types.JID, state, platform string, pid uint32) waBinary.Node {
	return waBinary.Node{
		Tag: "user",
		Attrs: waBinary.Attrs{
			"jid":   jid,
			"state": state,
		},
		Content: []waBinary.Node{{
			Tag: "device",
			Attrs: waBinary.Attrs{
				"jid":      jid,
				"platform": platform,
				"pid":      strconv.FormatUint(uint64(pid), 10),
			},
			Content: []waBinary.Node{{
				Tag:     "capability",
				Attrs:   waBinary.Attrs{"ver": "1"},
				Content: []byte{1, 5, 247, 9, 224, 250, 19},
			}},
		}},
	}
}
