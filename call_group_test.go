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
	"strconv"
	"strings"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type groupUpdateCorpus struct {
	Schema string            `json:"schema"`
	Cases  []groupUpdateCase `json:"cases"`
}

type groupUpdateCase struct {
	Name      string               `json:"name"`
	Outer     groupUpdateOuter     `json:"outer"`
	GroupInfo groupUpdateGroupInfo `json:"group_info"`
}

type groupUpdateOuter struct {
	From        string `json:"from"`
	CallID      string `json:"call_id"`
	CallCreator string `json:"call_creator"`
}

type groupUpdateGroupInfo struct {
	GroupJID       string                   `json:"group_jid"`
	TransactionID  uint32                   `json:"transaction_id"`
	Media          string                   `json:"media"`
	ConnectedLimit uint32                   `json:"connected_limit"`
	Participants   []groupUpdateParticipant `json:"participants"`
}

type groupUpdateParticipant struct {
	JID     string              `json:"jid"`
	State   string              `json:"state"`
	Devices []groupUpdateDevice `json:"devices"`
}

type groupUpdateDevice struct {
	JID      string  `json:"jid"`
	PID      *uint32 `json:"pid"`
	Platform string  `json:"platform"`
}

func TestGroupUpdateIngestionCorpus(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/48c2391ce9f7dcc2b3f223f72f1b5f0c627ad943/datasheets/voip-group-update-ingest.md#L112-L148
	corpus := loadGroupUpdateCorpus(t)
	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			cli, log, captured := routerTestClient()
			cli.SynchronousAck = true
			callCreator := mustParseGroupUpdateJID(t, tc.Outer.CallCreator)
			cli.putCall(tc.Outer.CallID, &callState{
				meta: types.BasicCallMeta{
					From:        mustParseGroupUpdateJID(t, tc.Outer.From),
					CallCreator: callCreator,
					CallID:      tc.Outer.CallID,
				},
				to:      callCreator,
				creator: callCreator,
			})

			node := groupUpdateNodeFromCorpus(t, tc, 0)
			cli.handleCallEvent(t.Context(), &node)

			updates := captured.filter(isCallGroupUpdate)
			if len(updates) != 1 {
				t.Fatalf("CallGroupUpdate dispatch count = %d, want 1", len(updates))
			}
			event := updates[0].(*events.CallGroupUpdate)
			assertGroupUpdateEvent(t, tc, event)
			if n := len(captured.filter(isUnknownCallEvent)); n != 0 {
				t.Fatalf("UnknownCallEvent dispatch count = %d, want 0", n)
			}
			cs := cli.getCall(tc.Outer.CallID)
			if cs == nil || cs.group == nil || !reflect.DeepEqual(cs.group.snapshot, event.Update) {
				t.Fatalf("stored group snapshot = %+v, want event update %+v", cs, event.Update)
			}

			cli.handleCallEvent(t.Context(), &node)
			if n := len(captured.filter(isCallGroupUpdate)); n != 1 {
				t.Fatalf("duplicate update dispatch count = %d, want 1 total", n)
			}

			cli.dropCall(tc.Outer.CallID)
			late := groupUpdateNodeFromCorpus(t, tc, 100)
			cli.handleCallEvent(t.Context(), &late)
			if n := len(captured.filter(isCallGroupUpdate)); n != 1 {
				t.Fatalf("post-terminate update dispatch count = %d, want 1 total", n)
			}
			if cs := cli.getCall(tc.Outer.CallID); cs != nil {
				t.Fatalf("post-terminate update recreated call state: %+v", cs)
			}
			if n := log.warnCount("Failed to send acknowledgement for call"); n != 3 {
				t.Fatalf("deferred ACK attempts = %d, want 3", n)
			}
		})
	}
}

func TestGroupUpdateMalformedDispatchesUnknownAndStillAcks(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/48c2391ce9f7dcc2b3f223f72f1b5f0c627ad943/datasheets/voip-group-update-ingest.md#L145-L148
	cli, log, captured := routerTestClient()
	cli.SynchronousAck = true
	creator := callStatePeerJID()
	cli.putCall("MALFORMED", &callState{meta: types.BasicCallMeta{CallID: "MALFORMED"}})
	node := waBinary.Node{
		Tag: "call",
		Attrs: waBinary.Attrs{
			"from": creator,
			"id":   "malformed-group-update",
			"t":    "1721730000",
		},
		Content: []waBinary.Node{{
			Tag: "group_update",
			Attrs: waBinary.Attrs{
				"call-id":      "MALFORMED",
				"call-creator": creator,
			},
		}},
	}

	cli.handleCallEvent(t.Context(), &node)

	if n := len(captured.filter(isCallGroupUpdate)); n != 0 {
		t.Fatalf("CallGroupUpdate dispatch count = %d, want 0", n)
	}
	unknown := captured.filter(isUnknownCallEvent)
	if len(unknown) != 1 {
		t.Fatalf("UnknownCallEvent dispatch count = %d, want 1", len(unknown))
	}
	if event := unknown[0].(*events.UnknownCallEvent); event.Node == nil || event.Node.Tag != "group_update" {
		t.Fatalf("UnknownCallEvent node = %+v, want group_update child", event.Node)
	}
	if !log.hasWarn("Failed to parse group call update") {
		t.Fatal("missing sanitized malformed-update warning")
	}
	if n := log.warnCount("Failed to send acknowledgement for call"); n != 1 {
		t.Fatalf("deferred ACK attempts = %d, want 1", n)
	}
	if cs := cli.getCall("MALFORMED"); cs == nil || cs.group != nil {
		t.Fatalf("malformed update mutated call state: %+v", cs)
	}
}

func loadGroupUpdateCorpus(t *testing.T) groupUpdateCorpus {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/48c2391ce9f7dcc2b3f223f72f1b5f0c627ad943/datasheets/voip-group-update-ingest.md#L17-L88
	t.Helper()
	vector, err := os.ReadFile("testdata/group_update_corpus.json")
	if err != nil {
		t.Fatalf("read group-update corpus: %v", err)
	}
	var corpus groupUpdateCorpus
	if err = json.Unmarshal(vector, &corpus); err != nil {
		t.Fatalf("decode group-update corpus: %v", err)
	}
	if corpus.Schema != "whatsmeow.group-update-corpus.v1" {
		t.Fatalf("corpus schema = %q", corpus.Schema)
	}
	if len(corpus.Cases) != 4 {
		t.Fatalf("corpus cases = %d, want 4", len(corpus.Cases))
	}
	return corpus
}

func groupUpdateNodeFromCorpus(t *testing.T, tc groupUpdateCase, transactionOffset uint32) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/48c2391ce9f7dcc2b3f223f72f1b5f0c627ad943/datasheets/voip-group-update-ingest.md#L36-L88
	t.Helper()
	callCreator := mustParseGroupUpdateJID(t, tc.Outer.CallCreator)
	groupAttrs := waBinary.Attrs{
		"call-id":         tc.Outer.CallID,
		"call-creator":    callCreator,
		"transaction-id":  strconv.FormatUint(uint64(tc.GroupInfo.TransactionID+transactionOffset), 10),
		"media":           tc.GroupInfo.Media,
		"connected-limit": strconv.FormatUint(uint64(tc.GroupInfo.ConnectedLimit), 10),
	}
	if tc.GroupInfo.GroupJID != "" {
		groupAttrs["group-jid"] = mustParseGroupUpdateJID(t, tc.GroupInfo.GroupJID)
	}
	users := make([]waBinary.Node, 0, len(tc.GroupInfo.Participants))
	for _, participant := range tc.GroupInfo.Participants {
		userJID := mustParseGroupUpdateJID(t, participant.JID)
		devices := make([]waBinary.Node, 0, len(participant.Devices))
		for _, device := range participant.Devices {
			attrs := waBinary.Attrs{"jid": mustParseGroupUpdateJID(t, device.JID)}
			if device.PID != nil {
				attrs["pid"] = strconv.FormatUint(uint64(*device.PID), 10)
			}
			if device.Platform != "" {
				attrs["platform"] = device.Platform
			}
			devices = append(devices, waBinary.Node{Tag: "device", Attrs: attrs})
		}
		users = append(users, waBinary.Node{
			Tag:     "user",
			Attrs:   waBinary.Attrs{"jid": userJID, "state": participant.State},
			Content: devices,
		})
	}
	return waBinary.Node{
		Tag: "call",
		Attrs: waBinary.Attrs{
			"from": mustParseGroupUpdateJID(t, tc.Outer.From),
			"id":   "group-update-" + tc.Name,
			"t":    "1721730000",
		},
		Content: []waBinary.Node{{
			Tag: "group_update",
			Attrs: waBinary.Attrs{
				"call-id":      tc.Outer.CallID,
				"call-creator": callCreator,
			},
			Content: []waBinary.Node{{
				Tag:     "group_info",
				Attrs:   groupAttrs,
				Content: users,
			}},
		}},
	}
}

func assertGroupUpdateEvent(t *testing.T, tc groupUpdateCase, event *events.CallGroupUpdate) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/48c2391ce9f7dcc2b3f223f72f1b5f0c627ad943/datasheets/voip-group-update-ingest.md#L36-L118
	t.Helper()
	if event.CallID != tc.Outer.CallID {
		t.Errorf("CallID = %q, want %q", event.CallID, tc.Outer.CallID)
	}
	wantFrom := mustParseGroupUpdateJID(t, tc.Outer.From)
	if event.From != wantFrom {
		t.Errorf("From = %s, want %s", event.From, wantFrom)
	}
	wantCreator := mustParseGroupUpdateJID(t, tc.Outer.CallCreator)
	if event.CallCreator != wantCreator || event.Update.CallCreator != wantCreator {
		t.Errorf("CallCreator = %s / %s, want %s", event.CallCreator, event.Update.CallCreator, wantCreator)
	}
	if event.Update.CallID != tc.Outer.CallID {
		t.Errorf("Update.CallID = %q, want %q", event.Update.CallID, tc.Outer.CallID)
	}
	wantGroupJID := mustParseGroupUpdateJID(t, tc.GroupInfo.GroupJID)
	if event.GroupJID != wantGroupJID || event.Update.GroupJID != wantGroupJID {
		t.Errorf("GroupJID = %s / %s, want %s", event.GroupJID, event.Update.GroupJID, wantGroupJID)
	}
	if event.Update.TransactionID != tc.GroupInfo.TransactionID ||
		event.Update.Media != tc.GroupInfo.Media ||
		event.Update.ConnectedLimit != tc.GroupInfo.ConnectedLimit {
		t.Errorf("group info = transaction %d, media %q, limit %d",
			event.Update.TransactionID, event.Update.Media, event.Update.ConnectedLimit)
	}
	if len(event.Update.Participants) != len(tc.GroupInfo.Participants) {
		t.Fatalf("participants = %d, want %d", len(event.Update.Participants), len(tc.GroupInfo.Participants))
	}
	for i, wantParticipant := range tc.GroupInfo.Participants {
		gotParticipant := event.Update.Participants[i]
		if gotParticipant.JID != mustParseGroupUpdateJID(t, wantParticipant.JID) ||
			gotParticipant.State != wantParticipant.State {
			t.Errorf("participant %d = %s/%s, want %s/%s",
				i, gotParticipant.JID, gotParticipant.State, wantParticipant.JID, wantParticipant.State)
		}
		if len(gotParticipant.Devices) != len(wantParticipant.Devices) {
			t.Fatalf("participant %d devices = %d, want %d",
				i, len(gotParticipant.Devices), len(wantParticipant.Devices))
		}
		for j, wantDevice := range wantParticipant.Devices {
			gotDevice := gotParticipant.Devices[j]
			if gotDevice.JID != mustParseGroupUpdateJID(t, wantDevice.JID) ||
				gotDevice.Platform != wantDevice.Platform {
				t.Errorf("participant %d device %d = %s/%s, want %s/%s",
					i, j, gotDevice.JID, gotDevice.Platform, wantDevice.JID, wantDevice.Platform)
			}
			if wantDevice.PID == nil {
				if gotDevice.HasPID {
					t.Errorf("participant %d device %d unexpectedly has PID %d", i, j, gotDevice.PID)
				}
			} else if !gotDevice.HasPID || gotDevice.PID != *wantDevice.PID {
				t.Errorf("participant %d device %d PID = %d (present %t), want %d",
					i, j, gotDevice.PID, gotDevice.HasPID, *wantDevice.PID)
			}
		}
	}
	if event.Data == nil || event.Data.Tag != "group_update" {
		t.Fatalf("Data = %+v, want group_update child", event.Data)
	}
}

func mustParseGroupUpdateJID(t *testing.T, raw string) types.JID {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/48c2391ce9f7dcc2b3f223f72f1b5f0c627ad943/datasheets/voip-group-update-ingest.md#L36-L88
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

func isCallGroupUpdate(event any) bool {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/48c2391ce9f7dcc2b3f223f72f1b5f0c627ad943/datasheets/voip-group-update-ingest.md#L93-L99
	_, ok := event.(*events.CallGroupUpdate)
	return ok
}

func isUnknownCallEvent(event any) bool {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/48c2391ce9f7dcc2b3f223f72f1b5f0c627ad943/datasheets/voip-group-update-ingest.md#L145-L148
	_, ok := event.(*events.UnknownCallEvent)
	return ok
}

func (l *capturedLog) warnCount(substr string) int {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/48c2391ce9f7dcc2b3f223f72f1b5f0c627ad943/datasheets/voip-group-update-ingest.md#L127-L129
	l.mu.Lock()
	defer l.mu.Unlock()
	count := 0
	for _, warning := range l.warns {
		if strings.Contains(warning, substr) {
			count++
		}
	}
	return count
}
