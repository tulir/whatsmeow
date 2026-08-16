// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func rejectCallTestClient() *Client {
	self := callStateSelfJID()
	return &Client{
		calls: map[string]*callState{},
		Log:   waLog.Noop,
		Store: &store.Device{
			ID:            &self,
			PrivacyTokens: &store.NoopStore{},
		},
	}
}

func TestRejectCallSendFailureLeavesStateRegistered(t *testing.T) {
	cli := rejectCallTestClient()
	cli.putCall("CID", &callState{to: callStatePeerJID(), creator: callStateSelfJID()})

	err := cli.RejectCall(context.Background(), callStatePeerJID(), "CID")
	if err == nil || !strings.Contains(err.Error(), ErrNotConnected.Error()) {
		t.Fatalf("RejectCall send-failure error = %v, want it to wrap %v", err, ErrNotConnected)
	}
	if cs := cli.getCall("CID"); cs == nil {
		t.Error("call state was dropped despite the reject send failing")
	}
}

func TestRejectCallNotLoggedIn(t *testing.T) {
	cli := &Client{calls: map[string]*callState{}, Log: waLog.Noop, Store: &store.Device{}}
	if err := cli.RejectCall(context.Background(), callStatePeerJID(), "CID"); err != ErrNotLoggedIn {
		t.Fatalf("RejectCall(not logged in) = %v, want %v", err, ErrNotLoggedIn)
	}
}

func TestRejectCallEmitsCallMediaStop(t *testing.T) {
	cli := rejectCallTestClient()
	ce := &capturedEvents{}
	cli.AddEventHandler(ce.add)
	peer := callStatePeerJID()
	cli.putCall("CID", &callState{
		meta:    types.BasicCallMeta{CallID: "CID", From: peer, CallCreator: peer},
		to:      peer,
		creator: peer,
	})

	_ = cli.RejectCall(context.Background(), peer, "CID")

	stops := ce.filter(isCallMediaStop)
	if len(stops) != 1 {
		t.Fatalf("CallMediaStop dispatch count = %d, want 1", len(stops))
	}
	stop, ok := stops[0].(*events.CallMediaStop)
	if !ok {
		t.Fatal("captured event is not *events.CallMediaStop")
	}
	if stop.Reason != "rejected" {
		t.Errorf("Reason = %q, want rejected", stop.Reason)
	}
}

func TestRejectCallUnknownCallEmitsNoMediaStop(t *testing.T) {
	cli := rejectCallTestClient()
	ce := &capturedEvents{}
	cli.AddEventHandler(ce.add)

	_ = cli.RejectCall(context.Background(), callStatePeerJID(), "NOPE")

	if n := len(ce.filter(isCallMediaStop)); n != 0 {
		t.Errorf("CallMediaStop dispatch count = %d, want 0 for an unknown call", n)
	}
}
