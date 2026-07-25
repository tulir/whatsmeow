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

	"go.mau.fi/whatsmeow/types"
)

type groupInviteOfferCorpus struct {
	Schema string                 `json:"schema"`
	Cases  []groupInviteOfferCase `json:"cases"`
}

type groupInviteOfferCase struct {
	Name          string                              `json:"name"`
	CallID        string                              `json:"call_id"`
	To            string                              `json:"to"`
	CallCreator   string                              `json:"call_creator"`
	TargetDevices []string                            `json:"target_devices"`
	Participants  []groupInviteOfferVectorParticipant `json:"participants"`
}

type groupInviteOfferVectorParticipant struct {
	JID     string                         `json:"jid"`
	State   string                         `json:"state"`
	Devices []groupInviteOfferVectorDevice `json:"devices"`
}

type groupInviteOfferVectorDevice struct {
	JID               string `json:"jid"`
	CapabilityVersion uint32 `json:"capability_version"`
	CapabilityHex     string `json:"capability_hex"`
}

func TestBuildGroupInviteOfferCorpus(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/25eda415afb0f926112ca375c5892b95b4bd6f60/datasheets/voip-group-invite-offer.md#L36-L76
	vector, err := os.ReadFile("../testdata/group_invite_offer_corpus.json")
	if err != nil {
		t.Fatalf("read group invite offer corpus: %v", err)
	}
	var corpus groupInviteOfferCorpus
	if err = json.Unmarshal(vector, &corpus); err != nil {
		t.Fatalf("decode group invite offer corpus: %v", err)
	}
	if corpus.Schema != "whatsmeow.group-invite-offer-corpus.v1" {
		t.Fatalf("corpus schema = %q", corpus.Schema)
	}
	if len(corpus.Cases) != 2 {
		t.Fatalf("corpus cases = %d, want 2", len(corpus.Cases))
	}

	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			params := GroupInviteOfferParams{
				CallID:      tc.CallID,
				To:          mustGroupInviteJID(t, tc.To),
				CallCreator: mustGroupInviteJID(t, tc.CallCreator),
			}
			for _, raw := range tc.TargetDevices {
				params.TargetDevices = append(params.TargetDevices, mustGroupInviteJID(t, raw))
			}
			for _, vectorParticipant := range tc.Participants {
				participant := types.GroupCallParticipant{
					JID:   mustGroupInviteJID(t, vectorParticipant.JID),
					State: vectorParticipant.State,
				}
				for _, vectorDevice := range vectorParticipant.Devices {
					capability, decodeErr := hex.DecodeString(vectorDevice.CapabilityHex)
					if decodeErr != nil {
						t.Fatalf("decode capability: %v", decodeErr)
					}
					participant.Devices = append(participant.Devices, types.GroupCallDevice{
						JID:               mustGroupInviteJID(t, vectorDevice.JID),
						CapabilityVersion: vectorDevice.CapabilityVersion,
						Capability:        capability,
					})
				}
				params.Participants = append(params.Participants, participant)
			}

			call, buildErr := BuildGroupInviteOffer(params)
			if buildErr != nil {
				t.Fatalf("BuildGroupInviteOffer: %v", buildErr)
			}
			if call.Tag != "call" {
				t.Fatalf("outer tag = %q, want call", call.Tag)
			}
			if got, ok := call.Attrs["to"].(types.JID); !ok || got != params.To {
				t.Fatalf("outer to = %#v, want %s", call.Attrs["to"], params.To)
			}
			if _, ok := call.Attrs["id"]; ok {
				t.Fatal("low-level builder unexpectedly stamped a stanza ID")
			}
			if len(call.Attrs) != 1 {
				t.Errorf("outer attrs = %#v, want only to", call.Attrs)
			}

			offer := stanzaContentNodes(t, call)[0]
			if offer.Tag != "offer" {
				t.Fatalf("action tag = %q, want offer", offer.Tag)
			}
			if got, _ := stanzaAttrString(offer, "call-id"); got != params.CallID {
				t.Errorf("call-id = %q, want %q", got, params.CallID)
			}
			if got, ok := offer.Attrs["call-creator"].(types.JID); !ok || got != params.CallCreator {
				t.Errorf("call-creator = %#v, want %s", offer.Attrs["call-creator"], params.CallCreator)
			}
			if len(offer.Attrs) != 2 {
				t.Errorf("offer attrs = %#v, want only call-id and call-creator", offer.Attrs)
			}
			if got := stanzaChildTags(t, call); !stanzaEqTags(got, []string{"audio", "net", "destination", "group_info"}) {
				t.Fatalf("offer child tags = %v", got)
			}

			children := stanzaContentNodes(t, offer)
			if got, _ := stanzaAttrString(children[0], "enc"); got != "opus" {
				t.Errorf("audio enc = %q, want opus", got)
			}
			if got, _ := stanzaAttrString(children[0], "rate"); got != "16000" {
				t.Errorf("audio rate = %q, want 16000", got)
			}
			if got, _ := stanzaAttrString(children[1], "medium"); got != "2" {
				t.Errorf("net medium = %q, want 2", got)
			}
			if len(children[0].Attrs) != 2 || len(children[1].Attrs) != 1 {
				t.Errorf("media attrs = audio %#v, net %#v", children[0].Attrs, children[1].Attrs)
			}
			if len(children[2].Attrs) != 0 || len(children[3].Attrs) != 0 {
				t.Errorf("container attrs = destination %#v, group_info %#v", children[2].Attrs, children[3].Attrs)
			}

			destinations := stanzaContentNodes(t, children[2])
			if len(destinations) != len(params.TargetDevices) {
				t.Fatalf("destination count = %d, want %d", len(destinations), len(params.TargetDevices))
			}
			for i, destination := range destinations {
				if destination.Tag != "to" {
					t.Errorf("destination %d tag = %q, want to", i, destination.Tag)
				}
				if got, ok := destination.Attrs["jid"].(types.JID); !ok || got != params.TargetDevices[i] {
					t.Errorf("destination %d JID = %#v, want %s", i, destination.Attrs["jid"], params.TargetDevices[i])
				}
				if len(destination.Attrs) != 1 {
					t.Errorf("destination %d attrs = %#v, want only jid", i, destination.Attrs)
				}
			}

			users := stanzaContentNodes(t, children[3])
			if len(users) != len(params.Participants) {
				t.Fatalf("roster count = %d, want %d", len(users), len(params.Participants))
			}
			for i, user := range users {
				wantParticipant := params.Participants[i]
				if got, ok := user.Attrs["jid"].(types.JID); !ok || got != wantParticipant.JID {
					t.Errorf("participant %d JID = %#v, want %s", i, user.Attrs["jid"], wantParticipant.JID)
				}
				gotState, hasState := stanzaAttrString(user, "state")
				if wantParticipant.State == "" {
					if hasState {
						t.Errorf("participant %d state = %q, want absent", i, gotState)
					}
				} else if !hasState || gotState != wantParticipant.State {
					t.Errorf("participant %d state = %q (present %t), want %q", i, gotState, hasState, wantParticipant.State)
				}
				wantUserAttrs := 1
				if wantParticipant.State != "" {
					wantUserAttrs = 2
				}
				if len(user.Attrs) != wantUserAttrs {
					t.Errorf("participant %d attrs = %#v", i, user.Attrs)
				}

				devices := stanzaContentNodes(t, user)
				if len(devices) != len(wantParticipant.Devices) {
					t.Fatalf("participant %d device count = %d, want %d", i, len(devices), len(wantParticipant.Devices))
				}
				for j, device := range devices {
					wantDevice := wantParticipant.Devices[j]
					if got, ok := device.Attrs["jid"].(types.JID); !ok || got != wantDevice.JID {
						t.Errorf("participant %d device %d JID = %#v, want %s", i, j, device.Attrs["jid"], wantDevice.JID)
					}
					if len(device.Attrs) != 1 {
						t.Errorf("participant %d device %d attrs = %#v, want only jid", i, j, device.Attrs)
					}
					deviceChildren := stanzaContentNodes(t, device)
					if len(deviceChildren) != 1 {
						t.Fatalf("participant %d device %d child count = %d, want 1", i, j, len(deviceChildren))
					}
					capability := deviceChildren[0]
					if capability.Tag != "capability" {
						t.Errorf("participant %d device %d child = %q, want capability", i, j, capability.Tag)
					}
					if got, _ := stanzaAttrString(capability, "ver"); got != strconv.FormatUint(uint64(wantDevice.CapabilityVersion), 10) {
						t.Errorf("participant %d device %d capability version = %q", i, j, got)
					}
					gotCapability, ok := capability.Content.([]byte)
					if !ok || !bytes.Equal(gotCapability, wantDevice.Capability) {
						t.Errorf("participant %d device %d capability mismatch", i, j)
					}
					if len(capability.Attrs) != 1 {
						t.Errorf("participant %d device %d capability attrs = %#v, want only ver", i, j, capability.Attrs)
					}
				}
			}
		})
	}
}

func TestBuildGroupInviteOfferRejectsMissingRequiredFields(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/25eda415afb0f926112ca375c5892b95b4bd6f60/datasheets/voip-group-invite-offer.md#L128-L136
	peer := mustGroupInviteJID(t, "200003@lid")
	creator := mustGroupInviteJID(t, "100001:14@lid")
	participant := types.GroupCallParticipant{
		JID: peer,
		Devices: []types.GroupCallDevice{{
			JID:               peer,
			CapabilityVersion: 1,
			Capability:        []byte{1, 5, 247, 9, 224, 250, 27},
		}},
	}
	base := GroupInviteOfferParams{
		CallID:        "0063F48A8B4CA7D1DAF665F1CC8EB545",
		To:            peer,
		CallCreator:   creator,
		TargetDevices: []types.JID{peer},
		Participants:  []types.GroupCallParticipant{participant},
	}
	cases := []struct {
		name string
		edit func(*GroupInviteOfferParams)
		want string
	}{
		{name: "call ID", edit: func(params *GroupInviteOfferParams) { params.CallID = "" }, want: "call ID is required"},
		{name: "target", edit: func(params *GroupInviteOfferParams) { params.To = types.EmptyJID }, want: "target is required"},
		{name: "creator", edit: func(params *GroupInviteOfferParams) { params.CallCreator = types.EmptyJID }, want: "call creator is required"},
		{name: "target devices", edit: func(params *GroupInviteOfferParams) { params.TargetDevices = nil }, want: "target devices are required"},
		{name: "participants", edit: func(params *GroupInviteOfferParams) { params.Participants = nil }, want: "participants are required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := base
			tc.edit(&params)
			_, err := BuildGroupInviteOffer(params)
			if err == nil || err.Error() != "whatsmeow: build group invite offer: "+tc.want {
				t.Fatalf("error = %v, want %q", err, "whatsmeow: build group invite offer: "+tc.want)
			}
		})
	}
}

func TestBuildGroupInviteOfferCarriesVideoCapability(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/36d54857c74e45ccb08f6444a32d2afa13f20be9/datasheets/group-video-reactions.md#L21-L30
	peer := mustGroupInviteJID(t, "200003@lid")
	creator := mustGroupInviteJID(t, "100001:14@lid")
	call, err := BuildGroupInviteOffer(GroupInviteOfferParams{
		CallID:        "0063F48A8B4CA7D1DAF665F1CC8EB545",
		To:            peer,
		CallCreator:   creator,
		TargetDevices: []types.JID{peer},
		Participants: []types.GroupCallParticipant{{
			JID: peer,
			Devices: []types.GroupCallDevice{{
				JID:               peer,
				CapabilityVersion: 1,
				Capability:        []byte{1, 5, 247, 9, 224, 250, 27},
			}},
		}},
		Video: true,
	})
	if err != nil {
		t.Fatalf("BuildGroupInviteOffer: %v", err)
	}
	if got := stanzaChildTags(t, call); !stanzaEqTags(got, []string{"audio", "video", "net", "destination", "group_info"}) {
		t.Fatalf("video invite child tags = %v", got)
	}
	video := stanzaContentNodes(t, stanzaContentNodes(t, call)[0])[1]
	if got, _ := stanzaAttrString(video, "enc"); got != "h.264" {
		t.Fatalf("video enc = %q, want h.264", got)
	}
}

func mustGroupInviteJID(t *testing.T, raw string) types.JID {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/25eda415afb0f926112ca375c5892b95b4bd6f60/datasheets/voip-group-invite-offer.md#L36-L76
	t.Helper()
	jid, err := types.ParseJID(raw)
	if err != nil {
		t.Fatalf("parse JID %q: %v", raw, err)
	}
	return jid
}
