// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func membershipApprovalModeNode(state string) waBinary.Node {
	return waBinary.Node{
		Tag: "membership_approval_mode",
		Content: []waBinary.Node{{
			Tag:   "group_join",
			Attrs: waBinary.Attrs{"state": state},
		}},
	}
}

func TestParseGroupNodeMembershipApprovalMode(t *testing.T) {
	cli := &Client{}
	for _, state := range []string{"on", "off"} {
		t.Run(state, func(t *testing.T) {
			node := waBinary.Node{
				Tag:     "group",
				Attrs:   waBinary.Attrs{"id": "123"},
				Content: []waBinary.Node{membershipApprovalModeNode(state)},
			}
			group, err := cli.parseGroupNode(&node)
			if err != nil {
				t.Fatalf("failed to parse group node: %v", err)
			}
			want := state == "on"
			if group.IsJoinApprovalRequired != want {
				t.Errorf("IsJoinApprovalRequired = %v, want %v", group.IsJoinApprovalRequired, want)
			}
		})
	}
}

func TestParseGroupChangeMembershipApprovalMode(t *testing.T) {
	cli := &Client{}
	for _, state := range []string{"on", "off"} {
		t.Run(state, func(t *testing.T) {
			node := waBinary.Node{
				Tag: "notification",
				Attrs: waBinary.Attrs{
					"from": types.NewJID("123", types.GroupServer),
					"t":    "0",
				},
				Content: []waBinary.Node{membershipApprovalModeNode(state)},
			}
			evt, _, err := cli.parseGroupChange(&node)
			if err != nil {
				t.Fatalf("failed to parse group change: %v", err)
			}
			if evt.MembershipApprovalMode == nil {
				t.Fatal("MembershipApprovalMode is nil")
			}
			want := state == "on"
			if evt.MembershipApprovalMode.IsJoinApprovalRequired != want {
				t.Errorf("IsJoinApprovalRequired = %v, want %v", evt.MembershipApprovalMode.IsJoinApprovalRequired, want)
			}
		})
	}
}
