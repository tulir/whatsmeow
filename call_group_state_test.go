// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

type groupCallStateCorpus struct {
	Schema string               `json:"schema"`
	Cases  []groupCallStateCase `json:"cases"`
}

type groupCallStateCase struct {
	Name         string                         `json:"name"`
	Registered   bool                           `json:"registered"`
	CallID       string                         `json:"call_id"`
	DirectTarget string                         `json:"direct_target"`
	Updates      []groupCallStateVectorSnapshot `json:"updates"`
	WantAccepted []bool                         `json:"want_accepted"`
	WantSnapshot int                            `json:"want_snapshot"`
	WantTarget   string                         `json:"want_target"`
}

type groupCallStateVectorSnapshot struct {
	TransactionID    uint32                            `json:"transaction_id"`
	GroupJID         string                            `json:"group_jid"`
	Media            string                            `json:"media"`
	Participants     []groupCallStateVectorParticipant `json:"participants"`
	RelayTransaction uint32                            `json:"relay_transaction"`
	RelaySelfPID     uint32                            `json:"relay_self_pid"`
	HasRelaySelfPID  bool                              `json:"has_relay_self_pid"`
}

type groupCallStateVectorParticipant struct {
	JID   string  `json:"jid"`
	State string  `json:"state"`
	PID   *uint32 `json:"pid"`
}

func loadGroupCallStateCorpus(t *testing.T) groupCallStateCorpus {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/699185f41519da3177c17ea6a10f9d4aa48b6941/datasheets/voip-group-call-state.md#L31-L44
	t.Helper()
	vector, err := os.ReadFile("testdata/group_call_state_corpus.json")
	if err != nil {
		t.Fatalf("read group-call state corpus: %v", err)
	}
	var corpus groupCallStateCorpus
	if err = json.Unmarshal(vector, &corpus); err != nil {
		t.Fatalf("decode group-call state corpus: %v", err)
	}
	if corpus.Schema != "whatsmeow.group-call-state-corpus.v1" {
		t.Fatalf("corpus schema = %q", corpus.Schema)
	}
	if len(corpus.Cases) != 6 {
		t.Fatalf("corpus cases = %d, want 6", len(corpus.Cases))
	}
	return corpus
}

func TestApplyGroupUpdateCorpus(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/699185f41519da3177c17ea6a10f9d4aa48b6941/datasheets/voip-group-call-state.md#L31-L44
	corpus := loadGroupCallStateCorpus(t)
	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			cli := &Client{calls: map[string]*callState{}}
			if tc.Registered {
				cli.putCall(tc.CallID, &callState{
					meta: types.BasicCallMeta{CallID: tc.CallID},
					to:   mustParseGroupStateJID(t, tc.DirectTarget),
				})
			}

			gotAccepted := make([]bool, len(tc.Updates))
			for i, update := range tc.Updates {
				gotAccepted[i] = cli.applyGroupUpdate(groupUpdateFromVector(t, tc.CallID, update))
			}
			if !reflect.DeepEqual(gotAccepted, tc.WantAccepted) {
				t.Fatalf("accepted updates = %v, want %v", gotAccepted, tc.WantAccepted)
			}

			cs := cli.getCall(tc.CallID)
			if !tc.Registered {
				if cs != nil {
					t.Fatal("late update recreated a removed call")
				}
				return
			}
			if cs == nil {
				t.Fatal("registered call disappeared")
			}
			if tc.WantSnapshot < 0 {
				if cs.group != nil {
					t.Fatalf("group state = %+v, want nil", cs.group)
				}
			} else {
				want := groupUpdateFromVector(t, tc.CallID, tc.Updates[tc.WantSnapshot])
				if cs.group == nil || !reflect.DeepEqual(cs.group.snapshot, want) {
					t.Fatalf("group snapshot = %+v, want %+v", cs.group, want)
				}
			}
		})
	}
}

func TestGroupCallSignalingTargetCorpus(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/699185f41519da3177c17ea6a10f9d4aa48b6941/datasheets/voip-group-call-state.md#L41-L43
	corpus := loadGroupCallStateCorpus(t)

	for _, tc := range corpus.Cases {
		if !tc.Registered {
			continue
		}
		t.Run(tc.Name, func(t *testing.T) {
			cli := &Client{calls: map[string]*callState{}}
			cli.putCall(tc.CallID, &callState{
				meta: types.BasicCallMeta{CallID: tc.CallID},
				to:   mustParseGroupStateJID(t, tc.DirectTarget),
			})
			for _, update := range tc.Updates {
				cli.applyGroupUpdate(groupUpdateFromVector(t, tc.CallID, update))
			}
			cs := cli.getCall(tc.CallID)
			if got, want := cs.signalingTarget(), mustParseGroupStateJID(t, tc.WantTarget); got != want {
				t.Fatalf("signaling target = %s, want %s", got, want)
			}
		})
	}
}

func TestGroupCallControlsUseCallScopedSignalingTarget(t *testing.T) {
	callID := "0032CD59A427AD3B9B48F33A71C85FE8"
	creator := mustParseGroupStateJID(t, "74170125783269:43@lid")
	peer := mustParseGroupStateJID(t, "242653052539031@lid")
	cs := &callState{
		to:      peer,
		creator: creator,
		meta:    types.BasicCallMeta{CallID: callID},
		group: &groupCallState{snapshot: types.GroupCallUpdate{
			CallID: callID,
		}},
	}
	want := types.NewJID(callID, "call")

	nodes := []waBinary.Node{
		buildCallTerminate(cs, callID, "terminate-id"),
		buildCallMute(cs, callID, "mute-id", "1"),
		buildCallVideoState(cs, callID, "video-id", types.CallVideoStateEnabled, nil),
	}
	for _, node := range nodes {
		action := node.GetChildren()[0]
		got, ok := node.Attrs["to"].(types.JID)
		if !ok {
			t.Fatalf("%s to attribute is %T, want types.JID", action.Tag, node.Attrs["to"])
		}
		if got != want {
			t.Errorf("%s target = %s, want %s", action.Tag, got, want)
		}
	}
}

func TestDirectCallControlsKeepPeerSignalingTarget(t *testing.T) {
	callID := "DIRECT"
	creator := mustParseGroupStateJID(t, "74170125783269:43@lid")
	peer := mustParseGroupStateJID(t, "242653052539031@lid")
	cs := &callState{to: peer, creator: creator}

	nodes := []waBinary.Node{
		buildCallTerminate(cs, callID, "terminate-id"),
		buildCallMute(cs, callID, "mute-id", "0"),
		buildCallVideoState(cs, callID, "video-id", types.CallVideoStateDisabled, nil),
	}
	for _, node := range nodes {
		action := node.GetChildren()[0]
		got, ok := node.Attrs["to"].(types.JID)
		if !ok {
			t.Fatalf("%s to attribute is %T, want types.JID", action.Tag, node.Attrs["to"])
		}
		if got != peer {
			t.Errorf("%s target = %s, want %s", action.Tag, got, peer)
		}
	}
}

func groupUpdateFromVector(t *testing.T, callID string, vector groupCallStateVectorSnapshot) types.GroupCallUpdate {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/699185f41519da3177c17ea6a10f9d4aa48b6941/datasheets/voip-group-call-state.md#L31-L44
	t.Helper()
	update := types.GroupCallUpdate{
		CallID:        callID,
		GroupJID:      mustParseGroupStateJID(t, vector.GroupJID),
		TransactionID: vector.TransactionID,
		Media:         vector.Media,
		Relay: &types.GroupCallRelay{
			TransactionID: vector.RelayTransaction,
			SelfPID:       vector.RelaySelfPID,
			HasSelfPID:    vector.HasRelaySelfPID,
		},
	}
	for _, participant := range vector.Participants {
		device := types.GroupCallDevice{JID: mustParseGroupStateJID(t, participant.JID)}
		if participant.PID != nil {
			device.PID = *participant.PID
			device.HasPID = true
		}
		update.Participants = append(update.Participants, types.GroupCallParticipant{
			JID:     mustParseGroupStateJID(t, participant.JID),
			State:   participant.State,
			Devices: []types.GroupCallDevice{device},
		})
	}
	return update
}

func mustParseGroupStateJID(t *testing.T, raw string) types.JID {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/699185f41519da3177c17ea6a10f9d4aa48b6941/datasheets/voip-group-call-state.md#L31-L44
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
