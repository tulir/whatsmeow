// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package voip

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

type groupInviteAcceptCorpus struct {
	Schema string                      `json:"schema"`
	Cases  []groupInviteAcceptTestCase `json:"cases"`
}

type groupInviteAcceptTestCase struct {
	Name         string                      `json:"name"`
	CallID       string                      `json:"call_id"`
	CallCreator  string                      `json:"call_creator"`
	Joinable     bool                        `json:"joinable"`
	GroupInfo    *groupInviteAcceptGroupInfo `json:"group_info"`
	WantSnapshot bool                        `json:"want_snapshot"`
}

type groupInviteAcceptGroupInfo struct {
	TransactionID  uint32                         `json:"transaction_id"`
	Media          string                         `json:"media"`
	ConnectedLimit uint32                         `json:"connected_limit"`
	Participants   []groupInviteAcceptParticipant `json:"participants"`
}

type groupInviteAcceptParticipant struct {
	JID     string                    `json:"jid"`
	State   string                    `json:"state"`
	Devices []groupInviteAcceptDevice `json:"devices"`
}

type groupInviteAcceptDevice struct {
	JID      string `json:"jid"`
	Platform string `json:"platform"`
}

func TestParseGroupInviteSnapshotCorpus(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L21-L40
	data, err := os.ReadFile("../testdata/group_invite_accept_corpus.json")
	if err != nil {
		t.Fatalf("read group invite accept corpus: %v", err)
	}
	var corpus groupInviteAcceptCorpus
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("decode group invite accept corpus: %v", err)
	}
	if corpus.Schema != "whatsmeow.group-invite-accept-corpus.v1" {
		t.Fatalf("corpus schema = %q", corpus.Schema)
	}
	if len(corpus.Cases) != 2 {
		t.Fatalf("corpus cases = %d, want 2", len(corpus.Cases))
	}

	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L28-L40
			offer := groupInviteAcceptOffer(t, tc)
			snapshot, ok, err := ParseGroupInviteSnapshot(&offer)
			if err != nil {
				t.Fatalf("ParseGroupInviteSnapshot: %v", err)
			}
			if ok != tc.WantSnapshot {
				t.Fatalf("snapshot present = %t, want %t", ok, tc.WantSnapshot)
			}
			if !tc.WantSnapshot {
				if snapshot != nil {
					t.Fatalf("ordinary offer snapshot = %+v, want nil", snapshot)
				}
				return
			}
			if snapshot == nil {
				t.Fatal("active group offer snapshot is nil")
			}
			if snapshot.CallID != tc.CallID || snapshot.CallCreator != mustGroupInviteAcceptJID(t, tc.CallCreator) {
				t.Errorf("call identity = %q/%s, want %q/%s", snapshot.CallID, snapshot.CallCreator, tc.CallID, tc.CallCreator)
			}
			if snapshot.TransactionID != tc.GroupInfo.TransactionID ||
				snapshot.Media != tc.GroupInfo.Media ||
				snapshot.ConnectedLimit != tc.GroupInfo.ConnectedLimit {
				t.Errorf("group state = transaction %d, media %q, limit %d",
					snapshot.TransactionID, snapshot.Media, snapshot.ConnectedLimit)
			}
			if snapshot.Joinable != tc.Joinable {
				t.Errorf("Joinable = %t, want %t", snapshot.Joinable, tc.Joinable)
			}
			if !snapshot.GroupJID.IsEmpty() {
				t.Errorf("GroupJID = %s, want empty ad-hoc group", snapshot.GroupJID)
			}
			if len(snapshot.Participants) != len(tc.GroupInfo.Participants) {
				t.Fatalf("participants = %d, want %d", len(snapshot.Participants), len(tc.GroupInfo.Participants))
			}
			for participantIndex, wantParticipant := range tc.GroupInfo.Participants {
				gotParticipant := snapshot.Participants[participantIndex]
				if gotParticipant.JID != mustGroupInviteAcceptJID(t, wantParticipant.JID) ||
					gotParticipant.State != wantParticipant.State {
					t.Errorf("participant %d = %s/%s, want %s/%s",
						participantIndex, gotParticipant.JID, gotParticipant.State, wantParticipant.JID, wantParticipant.State)
				}
				if len(gotParticipant.Devices) != len(wantParticipant.Devices) {
					t.Fatalf("participant %d devices = %d, want %d",
						participantIndex, len(gotParticipant.Devices), len(wantParticipant.Devices))
				}
			}
		})
	}
}

func TestParseGroupInviteSnapshotRejectsConflictingIdentity(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L73-L77
	data, err := os.ReadFile("../testdata/group_invite_accept_corpus.json")
	if err != nil {
		t.Fatalf("read group invite accept corpus: %v", err)
	}
	var corpus groupInviteAcceptCorpus
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("decode group invite accept corpus: %v", err)
	}
	active := corpus.Cases[1]

	for _, field := range []string{"call-id", "call-creator"} {
		t.Run(field, func(t *testing.T) {
			// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L73-L77
			offer := groupInviteAcceptOffer(t, active)
			groupInfo, ok := offer.GetOptionalChildByTag("group_info")
			if !ok {
				t.Fatal("fixture missing group_info")
			}
			if field == "call-id" {
				groupInfo.Attrs[field] = "OTHER-CALL"
			} else {
				groupInfo.Attrs[field] = mustGroupInviteAcceptJID(t, "900009:9@lid")
			}
			offer.Content = []waBinary.Node{groupInfo}
			if _, _, err := ParseGroupInviteSnapshot(&offer); err == nil {
				t.Fatalf("ParseGroupInviteSnapshot accepted conflicting %s", field)
			}
		})
	}
}

func groupInviteAcceptOffer(t *testing.T, tc groupInviteAcceptTestCase) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L21-L40
	t.Helper()
	creator := mustGroupInviteAcceptJID(t, tc.CallCreator)
	attrs := waBinary.Attrs{
		"call-id":      tc.CallID,
		"call-creator": creator,
	}
	if tc.Joinable {
		attrs["joinable"] = "1"
	}
	offer := waBinary.Node{Tag: "offer", Attrs: attrs}
	if tc.GroupInfo == nil {
		return offer
	}

	users := make([]waBinary.Node, len(tc.GroupInfo.Participants))
	for participantIndex, participant := range tc.GroupInfo.Participants {
		devices := make([]waBinary.Node, len(participant.Devices))
		for deviceIndex, device := range participant.Devices {
			devices[deviceIndex] = waBinary.Node{
				Tag: "device",
				Attrs: waBinary.Attrs{
					"jid":      mustGroupInviteAcceptJID(t, device.JID),
					"platform": device.Platform,
				},
			}
		}
		users[participantIndex] = waBinary.Node{
			Tag: "user",
			Attrs: waBinary.Attrs{
				"jid":   mustGroupInviteAcceptJID(t, participant.JID),
				"state": participant.State,
			},
			Content: devices,
		}
	}
	offer.Content = []waBinary.Node{{
		Tag: "group_info",
		Attrs: waBinary.Attrs{
			"call-id":         tc.CallID,
			"call-creator":    creator,
			"transaction-id":  strconv.FormatUint(uint64(tc.GroupInfo.TransactionID), 10),
			"media":           tc.GroupInfo.Media,
			"connected-limit": strconv.FormatUint(uint64(tc.GroupInfo.ConnectedLimit), 10),
		},
		Content: users,
	}}
	return offer
}

func mustGroupInviteAcceptJID(t *testing.T, raw string) types.JID {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L21-L40
	t.Helper()
	jid, err := types.ParseJID(raw)
	if err != nil {
		t.Fatalf("parse JID %q: %v", raw, err)
	}
	return jid
}
