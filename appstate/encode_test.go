// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package appstate_test

import (
	"reflect"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/types"
)

func TestBuildClearChat(t *testing.T) {
	jid := types.NewJID("1234567890", types.DefaultUserServer)
	now := time.Unix(1700000000, 0)
	key := &waCommon.MessageKey{
		RemoteJID: proto.String(jid.String()),
		FromMe:    proto.Bool(true),
		ID:        proto.String("TEST_MSG_ID"),
	}

	t.Run("without delete media", func(t *testing.T) {
		patch := appstate.BuildClearChat(jid, now, key, false)
		if patch.Type != appstate.WAPatchRegularHigh {
			t.Errorf("expected patch type %v, got %v", appstate.WAPatchRegularHigh, patch.Type)
		}
		if len(patch.Mutations) != 1 {
			t.Fatalf("expected 1 mutation, got %d", len(patch.Mutations))
		}
		mut := patch.Mutations[0]
		expectedIndex := []string{appstate.IndexClearChat, jid.String(), "0", "0"}
		if !reflect.DeepEqual(mut.Index, expectedIndex) {
			t.Errorf("expected index %v, got %v", expectedIndex, mut.Index)
		}
		action := mut.Value.GetClearChatAction()
		if action == nil {
			t.Fatal("expected ClearChatAction, got nil")
		}
		if action.GetMessageRange().GetLastMessageTimestamp() != now.Unix() {
			t.Errorf("expected timestamp %d, got %d", now.Unix(), action.GetMessageRange().GetLastMessageTimestamp())
		}
		if len(action.GetMessageRange().GetMessages()) != 1 {
			t.Fatalf("expected 1 message in range, got %d", len(action.GetMessageRange().GetMessages()))
		}
		if action.GetMessageRange().GetMessages()[0].GetKey().GetID() != "TEST_MSG_ID" {
			t.Errorf("expected message ID TEST_MSG_ID, got %s", action.GetMessageRange().GetMessages()[0].GetKey().GetID())
		}
	})

	t.Run("with delete media", func(t *testing.T) {
		patch := appstate.BuildClearChat(jid, now, nil, true)
		if len(patch.Mutations) != 1 {
			t.Fatalf("expected 1 mutation, got %d", len(patch.Mutations))
		}
		mut := patch.Mutations[0]
		expectedIndex := []string{appstate.IndexClearChat, jid.String(), "0", "1"}
		if !reflect.DeepEqual(mut.Index, expectedIndex) {
			t.Errorf("expected index %v, got %v", expectedIndex, mut.Index)
		}
		action := mut.Value.GetClearChatAction()
		if action == nil {
			t.Fatal("expected ClearChatAction, got nil")
		}
		if len(action.GetMessageRange().GetMessages()) != 0 {
			t.Errorf("expected 0 messages in range, got %d", len(action.GetMessageRange().GetMessages()))
		}
	})
}
