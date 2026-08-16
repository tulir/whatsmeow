// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package voip

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

type initialGroupCallCorpus struct {
	Schema string                         `json:"schema"`
	Offer  initialGroupCallOfferVector    `json:"offer"`
	Ack    initialGroupCallSnapshotVector `json:"ack"`
	Ready  initialGroupCallSnapshotVector `json:"ready"`
}

type initialGroupCallOfferVector struct {
	CallID        string                              `json:"call_id"`
	CallCreator   string                              `json:"call_creator"`
	GroupBoundJID string                              `json:"group_bound_jid"`
	Participants  []initialGroupCallParticipantVector `json:"participants"`
}

type initialGroupCallSnapshotVector struct {
	TransactionID  uint32                              `json:"transaction_id"`
	Media          string                              `json:"media"`
	ConnectedLimit uint32                              `json:"connected_limit"`
	Joinable       bool                                `json:"joinable"`
	Participants   []initialGroupCallParticipantVector `json:"participants"`
	Relay          initialGroupCallRelayVector         `json:"relay"`
}

type initialGroupCallParticipantVector struct {
	JID     string                         `json:"jid"`
	State   string                         `json:"state"`
	Devices []initialGroupCallDeviceVector `json:"devices"`
}

type initialGroupCallDeviceVector struct {
	JID               string  `json:"jid"`
	PID               *uint32 `json:"pid"`
	CapabilityVersion uint32  `json:"capability_version"`
	CapabilityHex     string  `json:"capability_hex"`
}

type initialGroupCallRelayVector struct {
	TransactionID    uint32                           `json:"transaction_id"`
	SelfPID          uint32                           `json:"self_pid"`
	KeyLength        int                              `json:"key_length"`
	KeyFill          byte                             `json:"key_fill"`
	TokenLengths     []int                            `json:"token_lengths"`
	TokenFills       []byte                           `json:"token_fills"`
	AuthTokenLengths []int                            `json:"auth_token_lengths"`
	AuthTokenFills   []byte                           `json:"auth_token_fills"`
	Endpoints        []initialGroupCallEndpointVector `json:"endpoints"`
}

type initialGroupCallEndpointVector struct {
	RelayID     uint32 `json:"relay_id"`
	TokenID     uint32 `json:"token_id"`
	AuthTokenID uint32 `json:"auth_token_id"`
	RelayName   string `json:"relay_name"`
	RTT         uint32 `json:"rtt"`
	IsFNA       bool   `json:"is_fna"`
	AddressHex  string `json:"address_hex"`
}

func loadInitialGroupCallCorpus(t *testing.T) initialGroupCallCorpus {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L47-L111
	t.Helper()
	raw, err := os.ReadFile("../testdata/initial_group_call_corpus.json")
	if err != nil {
		t.Fatalf("read initial group call corpus: %v", err)
	}
	var corpus initialGroupCallCorpus
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("decode initial group call corpus: %v", err)
	}
	if corpus.Schema != "whatsmeow.initial-group-call-corpus.v1" {
		t.Fatalf("corpus schema = %q", corpus.Schema)
	}
	return corpus
}

func TestBuildInitialGroupOfferCorpus(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L63-L101
	corpus := loadInitialGroupCallCorpus(t)
	participants := initialGroupParticipantsFromVector(t, corpus.Offer.Participants)
	cases := []struct {
		name     string
		groupJID types.JID
	}{
		{name: "ad_hoc"},
		{name: "group_bound", groupJID: mustInitialGroupCallJID(t, corpus.Offer.GroupBoundJID)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call, err := BuildInitialGroupOffer(InitialGroupOfferParams{
				CallID:       corpus.Offer.CallID,
				CallCreator:  mustInitialGroupCallJID(t, corpus.Offer.CallCreator),
				GroupJID:     tc.groupJID,
				Participants: participants,
			})
			if err != nil {
				t.Fatalf("BuildInitialGroupOffer: %v", err)
			}
			if call.Tag != "call" {
				t.Fatalf("outer tag = %q, want call", call.Tag)
			}
			wantTarget := types.NewJID(corpus.Offer.CallID, "call")
			if got, ok := call.Attrs["to"].(types.JID); !ok || got != wantTarget {
				t.Fatalf("outer target = %#v, want %s", call.Attrs["to"], wantTarget)
			}
			if len(call.Attrs) != 1 {
				t.Errorf("outer attrs = %#v, want only to", call.Attrs)
			}

			offer := stanzaContentNodes(t, call)[0]
			if offer.Tag != "offer" {
				t.Fatalf("action tag = %q, want offer", offer.Tag)
			}
			if got, _ := stanzaAttrString(offer, "call-id"); got != corpus.Offer.CallID {
				t.Errorf("call ID = %q, want %q", got, corpus.Offer.CallID)
			}
			if got, ok := offer.Attrs["call-creator"].(types.JID); !ok ||
				got != mustInitialGroupCallJID(t, corpus.Offer.CallCreator) {
				t.Errorf("call creator = %#v", offer.Attrs["call-creator"])
			}
			gotGroupJID, hasGroupJID := offer.Attrs["group-jid"].(types.JID)
			if tc.groupJID.IsEmpty() {
				if hasGroupJID {
					t.Errorf("ad-hoc offer group JID = %s, want absent", gotGroupJID)
				}
				if len(offer.Attrs) != 2 {
					t.Errorf("ad-hoc offer attrs = %#v", offer.Attrs)
				}
			} else {
				if !hasGroupJID || gotGroupJID != tc.groupJID {
					t.Errorf("group-bound offer group JID = %#v, want %s", offer.Attrs["group-jid"], tc.groupJID)
				}
				if len(offer.Attrs) != 3 {
					t.Errorf("group-bound offer attrs = %#v", offer.Attrs)
				}
			}

			if got := stanzaChildTags(t, call); !stanzaEqTags(got, []string{"audio", "audio", "net", "group_info"}) {
				t.Fatalf("offer child tags = %v", got)
			}
			children := stanzaContentNodes(t, offer)
			for i, rate := range []string{"8000", "16000"} {
				if got, _ := stanzaAttrString(children[i], "enc"); got != "opus" {
					t.Errorf("audio %d encoding = %q, want opus", i, got)
				}
				if got, _ := stanzaAttrString(children[i], "rate"); got != rate {
					t.Errorf("audio %d rate = %q, want %s", i, got, rate)
				}
			}
			if got, _ := stanzaAttrString(children[2], "medium"); got != "3" {
				t.Errorf("network medium = %q, want 3", got)
			}
			assertInitialGroupRoster(t, children[3], participants)
		})
	}
}

func TestBuildInitialGroupOfferRejectsIncompleteEnvelope(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L164-L176
	corpus := loadInitialGroupCallCorpus(t)
	base := InitialGroupOfferParams{
		CallID:       corpus.Offer.CallID,
		CallCreator:  mustInitialGroupCallJID(t, corpus.Offer.CallCreator),
		Participants: initialGroupParticipantsFromVector(t, corpus.Offer.Participants),
	}
	cases := []struct {
		name string
		edit func(*InitialGroupOfferParams)
	}{
		{name: "call ID", edit: func(params *InitialGroupOfferParams) { params.CallID = "" }},
		{name: "call creator", edit: func(params *InitialGroupOfferParams) { params.CallCreator = types.EmptyJID }},
		{name: "fewer than self plus two remotes", edit: func(params *InitialGroupOfferParams) {
			params.Participants = params.Participants[:2]
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := base
			tc.edit(&params)
			if _, err := BuildInitialGroupOffer(params); err == nil {
				t.Fatal("BuildInitialGroupOffer accepted incomplete envelope")
			}
		})
	}
}

func TestBuildInitialGroupOfferCarriesVideoCapability(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/36d54857c74e45ccb08f6444a32d2afa13f20be9/datasheets/group-video-reactions.md#L21-L30
	self := mustInitialGroupCallJID(t, "100001:14@lid")
	first := mustInitialGroupCallJID(t, "200002@lid")
	second := mustInitialGroupCallJID(t, "200003@lid")
	call, err := BuildInitialGroupOffer(InitialGroupOfferParams{
		CallID:      "0063F48A8B4CA7D1DAF665F1CC8EB545",
		CallCreator: self,
		Video:       true,
		Participants: []types.GroupCallParticipant{
			{JID: self, Devices: []types.GroupCallDevice{{
				JID: self, CapabilityVersion: 1, Capability: bytes.Clone(CapabilityOffer),
			}}},
			{JID: first, Devices: []types.GroupCallDevice{{JID: first}}},
			{JID: second, Devices: []types.GroupCallDevice{{JID: second}}},
		},
	})
	if err != nil {
		t.Fatalf("BuildInitialGroupOffer: %v", err)
	}
	if got := stanzaChildTags(t, call); !stanzaEqTags(got, []string{"audio", "audio", "video", "net", "group_info"}) {
		t.Fatalf("video group offer child tags = %v", got)
	}
	children := stanzaContentNodes(t, stanzaContentNodes(t, call)[0])
	if got, _ := stanzaAttrString(children[2], "enc"); got != "h.264" {
		t.Fatalf("video enc = %q, want h.264", got)
	}
	groupInfo := children[4]
	selfDevice := stanzaContentNodes(t, stanzaContentNodes(t, groupInfo)[0])[0]
	capability := stanzaContentNodes(t, selfDevice)[0]
	if got, ok := capability.Content.([]byte); !ok || bytes.Equal(got, CapabilityOffer) {
		t.Fatalf("video group offer retained audio-only capability: %x", got)
	}
}

func TestParseInitialGroupCallAckCorpus(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L27-L33
	corpus := loadInitialGroupCallCorpus(t)
	node := initialGroupAckNodeFromVector(t, corpus)
	update, isGroup, err := ParseInitialGroupCallAck(&node)
	if err != nil {
		t.Fatalf("ParseInitialGroupCallAck: %v", err)
	}
	if !isGroup {
		t.Fatal("capture ACK was not recognized as an initial group call")
	}
	if update.CallID != corpus.Offer.CallID ||
		update.CallCreator != mustInitialGroupCallJID(t, corpus.Offer.CallCreator) ||
		update.TransactionID != 11 || update.Media != "audio" ||
		update.ConnectedLimit != 32 || !update.Joinable {
		t.Fatalf("parsed ACK identity/state = %+v", update)
	}
	wantParticipants := initialGroupParticipantsFromVector(t, corpus.Ack.Participants)
	if len(update.Participants) != len(wantParticipants) {
		t.Fatalf("participants = %d, want %d", len(update.Participants), len(wantParticipants))
	}
	for i := range wantParticipants {
		if update.Participants[i].JID != wantParticipants[i].JID ||
			update.Participants[i].State != wantParticipants[i].State ||
			len(update.Participants[i].Devices) != len(wantParticipants[i].Devices) {
			t.Errorf("participant %d = %+v, want %+v", i, update.Participants[i], wantParticipants[i])
		}
	}
	if update.Relay == nil || update.Relay.TransactionID != 0 ||
		!update.Relay.HasSelfPID || update.Relay.SelfPID != 0 ||
		len(update.Relay.Key) != 24 || len(update.Relay.Endpoints) != 2 {
		t.Fatalf("parsed ACK relay = %+v", update.Relay)
	}
}

func TestParseInitialGroupCallAckLeavesDirectAckUntyped(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L27-L33
	update, isGroup, err := ParseInitialGroupCallAck(&waBinary.Node{Tag: "ack"})
	if err != nil {
		t.Fatalf("ParseInitialGroupCallAck(direct): %v", err)
	}
	if isGroup || update != nil {
		t.Fatalf("direct ACK parsed as group: update=%+v is_group=%t", update, isGroup)
	}
}

func assertInitialGroupRoster(t *testing.T, groupInfo waBinary.Node, want []types.GroupCallParticipant) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L63-L101
	t.Helper()
	if groupInfo.Tag != "group_info" || len(groupInfo.Attrs) != 0 {
		t.Fatalf("group_info envelope = %+v", groupInfo)
	}
	users := stanzaContentNodes(t, groupInfo)
	if len(users) != len(want) {
		t.Fatalf("roster users = %d, want %d", len(users), len(want))
	}
	for i, user := range users {
		if got, ok := user.Attrs["jid"].(types.JID); !ok || got != want[i].JID {
			t.Errorf("user %d JID = %#v, want %s", i, user.Attrs["jid"], want[i].JID)
		}
		if len(user.Attrs) != 1 {
			t.Errorf("user %d attrs = %#v, want only jid", i, user.Attrs)
		}
		devices := stanzaContentNodes(t, user)
		if len(devices) != len(want[i].Devices) {
			t.Fatalf("user %d devices = %d, want %d", i, len(devices), len(want[i].Devices))
		}
		for j, device := range devices {
			wantDevice := want[i].Devices[j]
			if got, ok := device.Attrs["jid"].(types.JID); !ok || got != wantDevice.JID {
				t.Errorf("user %d device %d JID = %#v, want %s", i, j, device.Attrs["jid"], wantDevice.JID)
			}
			deviceChildren := stanzaContentNodes(t, device)
			if wantDevice.Capability == nil {
				if len(deviceChildren) != 0 {
					t.Errorf("remote user %d device %d unexpectedly advertises capability", i, j)
				}
				continue
			}
			if len(deviceChildren) != 1 || deviceChildren[0].Tag != "capability" {
				t.Fatalf("user %d device %d capability children = %+v", i, j, deviceChildren)
			}
			if got, _ := stanzaAttrString(deviceChildren[0], "ver"); got != strconv.FormatUint(uint64(wantDevice.CapabilityVersion), 10) {
				t.Errorf("user %d device %d capability version = %q", i, j, got)
			}
			gotCapability, ok := deviceChildren[0].Content.([]byte)
			if !ok || !bytes.Equal(gotCapability, wantDevice.Capability) {
				t.Errorf("user %d device %d capability mismatch", i, j)
			}
		}
	}
}

func initialGroupParticipantsFromVector(
	t *testing.T,
	vectors []initialGroupCallParticipantVector,
) []types.GroupCallParticipant {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L47-L111
	t.Helper()
	participants := make([]types.GroupCallParticipant, len(vectors))
	for i, vector := range vectors {
		participants[i] = types.GroupCallParticipant{
			JID:   mustInitialGroupCallJID(t, vector.JID),
			State: vector.State,
		}
		for _, deviceVector := range vector.Devices {
			var capability []byte
			if deviceVector.CapabilityHex != "" {
				var err error
				capability, err = hex.DecodeString(deviceVector.CapabilityHex)
				if err != nil {
					t.Fatalf("decode capability: %v", err)
				}
			}
			device := types.GroupCallDevice{
				JID:               mustInitialGroupCallJID(t, deviceVector.JID),
				CapabilityVersion: deviceVector.CapabilityVersion,
				Capability:        capability,
			}
			if deviceVector.PID != nil {
				device.PID = *deviceVector.PID
				device.HasPID = true
			}
			participants[i].Devices = append(participants[i].Devices, device)
		}
	}
	return participants
}

func initialGroupAckNodeFromVector(t *testing.T, corpus initialGroupCallCorpus) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L27-L33
	t.Helper()
	groupInfo := waBinary.Node{
		Tag: "group_info",
		Attrs: waBinary.Attrs{
			"call-id":         corpus.Offer.CallID,
			"call-creator":    mustInitialGroupCallJID(t, corpus.Offer.CallCreator),
			"transaction-id":  strconv.FormatUint(uint64(corpus.Ack.TransactionID), 10),
			"media":           corpus.Ack.Media,
			"connected-limit": strconv.FormatUint(uint64(corpus.Ack.ConnectedLimit), 10),
			"joinable":        "1",
		},
	}
	for _, participant := range initialGroupParticipantsFromVector(t, corpus.Ack.Participants) {
		devices := make([]waBinary.Node, len(participant.Devices))
		for i, device := range participant.Devices {
			devices[i] = waBinary.Node{Tag: "device", Attrs: waBinary.Attrs{"jid": device.JID}}
		}
		groupInfo.Content = append(groupInfo.GetChildren(), waBinary.Node{
			Tag:     "user",
			Attrs:   waBinary.Attrs{"jid": participant.JID, "state": participant.State},
			Content: devices,
		})
	}
	relay := initialGroupRelayNodeFromVector(t, corpus.Ack.Relay)
	return waBinary.Node{
		Tag:     "ack",
		Attrs:   waBinary.Attrs{"class": "call", "type": "offer"},
		Content: []waBinary.Node{groupInfo, relay},
	}
}

func initialGroupRelayNodeFromVector(t *testing.T, vector initialGroupCallRelayVector) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L103-L111
	t.Helper()
	children := []waBinary.Node{{Tag: "key", Content: bytes.Repeat([]byte{vector.KeyFill}, vector.KeyLength)}}
	for i, length := range vector.TokenLengths {
		children = append(children, waBinary.Node{
			Tag:     "token",
			Attrs:   waBinary.Attrs{"id": strconv.Itoa(i)},
			Content: bytes.Repeat([]byte{vector.TokenFills[i]}, length),
		})
	}
	for i, length := range vector.AuthTokenLengths {
		children = append(children, waBinary.Node{
			Tag:     "auth_token",
			Attrs:   waBinary.Attrs{"id": strconv.Itoa(i)},
			Content: bytes.Repeat([]byte{vector.AuthTokenFills[i]}, length),
		})
	}
	for _, endpointVector := range vector.Endpoints {
		address, err := hex.DecodeString(endpointVector.AddressHex)
		if err != nil {
			t.Fatalf("decode relay endpoint: %v", err)
		}
		attrs := waBinary.Attrs{
			"relay_id":      strconv.FormatUint(uint64(endpointVector.RelayID), 10),
			"token_id":      strconv.FormatUint(uint64(endpointVector.TokenID), 10),
			"auth_token_id": strconv.FormatUint(uint64(endpointVector.AuthTokenID), 10),
			"relay_name":    endpointVector.RelayName,
		}
		if endpointVector.RTT != 0 {
			attrs["c2r_rtt"] = strconv.FormatUint(uint64(endpointVector.RTT), 10)
		}
		if endpointVector.IsFNA {
			attrs["is_fna"] = "1"
		}
		children = append(children, waBinary.Node{Tag: "te2", Attrs: attrs, Content: address})
	}
	return waBinary.Node{
		Tag: "relay",
		Attrs: waBinary.Attrs{
			"transaction-id":   strconv.FormatUint(uint64(vector.TransactionID), 10),
			"self_pid":         strconv.FormatUint(uint64(vector.SelfPID), 10),
			"uuid":             "capture-relay",
			"participant_uuid": "capture-self",
		},
		Content: children,
	}
}

func mustInitialGroupCallJID(t *testing.T, raw string) types.JID {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L47-L111
	t.Helper()
	if raw == "" {
		return types.EmptyJID
	}
	jid, err := types.ParseJID(raw)
	if err != nil {
		t.Fatalf("parse JID %q: %v", raw, err)
	}
	return jid
}
