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

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.mau.fi/whatsmeow/voip"
)

func callStatePeerJID() types.JID {
	return types.JID{User: "214482127208608", Server: types.HiddenUserServer}
}

func callStateSelfJID() types.JID {
	return types.JID{User: "111111111111111", Server: types.HiddenUserServer}
}

func TestCallStateGetPutDrop(t *testing.T) {
	cli := &Client{calls: map[string]*callState{}}
	if cs := cli.getCall("CID"); cs != nil {
		t.Fatalf("getCall(unregistered) = %+v, want nil", cs)
	}
	want := &callState{to: callStatePeerJID(), creator: callStateSelfJID()}
	cli.putCall("CID", want)
	if got := cli.getCall("CID"); got != want {
		t.Fatalf("getCall(registered) = %+v, want %+v", got, want)
	}
	cli.dropCall("CID")
	if cs := cli.getCall("CID"); cs != nil {
		t.Fatalf("getCall(dropped) = %+v, want nil", cs)
	}
}

func TestComposeOfferBuildsCallOfferWithID(t *testing.T) {
	cli := &Client{}
	self, peer := callStateSelfJID(), callStatePeerJID()
	dk := voip.DeviceKey{DeviceJID: peer, Ciphertext: []byte{1, 2, 3}, EncType: "pkmsg"}
	offer := cli.composeOffer("CID", self, peer, []voip.DeviceKey{dk}, []byte{0xaa}, []byte{0xbb}, false)

	if offer.Tag != "call" {
		t.Errorf("outer tag = %q, want call", offer.Tag)
	}
	if id := offer.AttrGetter().String("id"); id == "" {
		t.Error("call stanza id was not stamped")
	}
	if to, _ := offer.Attrs["to"].(types.JID); to != peer {
		t.Errorf("to = %v, want %v", to, peer)
	}

	actions := offer.GetChildren()
	if len(actions) != 1 {
		t.Fatalf("call action count = %d, want 1", len(actions))
	}
	action := actions[0]
	if action.Tag != "offer" {
		t.Fatalf("action tag = %q, want offer", action.Tag)
	}
	if cid := action.AttrGetter().String("call-id"); cid != "CID" {
		t.Errorf("call-id = %q, want CID", cid)
	}
	if creator, _ := action.Attrs["call-creator"].(types.JID); creator != self {
		t.Errorf("call-creator = %v, want %v", creator, self)
	}
	want := []string{"privacy", "audio", "audio", "net", "capability", "enc", "encopt", "device-identity"}
	children := action.GetChildren()
	got := make([]string, len(children))
	for i := range children {
		got[i] = children[i].Tag
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("child tags = %v, want %v", got, want)
	}
}

func TestNewOutgoingCallStateMapsFields(t *testing.T) {
	cli := &Client{calls: map[string]*callState{}}
	self, peer := callStateSelfJID(), callStatePeerJID()
	callKey := []byte{1, 2, 3, 4}

	cs := cli.newOutgoingCallState("CID", self, peer, callKey, false)

	if cs.meta.CallID != "CID" {
		t.Errorf("meta.CallID = %q, want CID", cs.meta.CallID)
	}
	if cs.meta.From != self || cs.meta.CallCreator != self {
		t.Errorf("meta.From/CallCreator = %v/%v, want both %v", cs.meta.From, cs.meta.CallCreator, self)
	}
	if cs.selfLID != self {
		t.Errorf("selfLID = %v, want %v", cs.selfLID, self)
	}
	if cs.peerLID != peer {
		t.Errorf("peerLID = %v, want %v", cs.peerLID, peer)
	}
	if cs.to != peer {
		t.Errorf("to = %v, want %v (the offer is sent to the peer)", cs.to, peer)
	}
	if cs.creator != self {
		t.Errorf("creator = %v, want %v (we placed this call)", cs.creator, self)
	}
	if !cs.outgoing {
		t.Error("outgoing = false, want true for a call we placed")
	}
	if string(cs.callKey) != string(callKey) {
		t.Errorf("callKey = %v, want %v", cs.callKey, callKey)
	}

	cli.putCall("CID", cs)
	if cli.getCall("CID") == nil {
		t.Fatal("getCall(CID) = nil after putCall, want the registered state")
	}
}

func TestResolvePeerCallLIDAlreadyLID(t *testing.T) {
	cli := &Client{}
	peer := callStatePeerJID()
	got, err := cli.resolvePeerCallLID(context.Background(), peer)
	if err != nil {
		t.Fatalf("resolvePeerCallLID: %v", err)
	}
	if got != peer {
		t.Errorf("resolvePeerCallLID = %v, want %v unchanged", got, peer)
	}
}

func TestAcceptCallArmsDeferredAccept(t *testing.T) {
	cli := &Client{calls: map[string]*callState{}, Log: waLog.Noop}
	cli.putCall("CID", &callState{})
	if err := cli.AcceptCall(context.Background(), "CID"); err != nil {
		t.Fatalf("AcceptCall: %v", err)
	}
	cs := cli.getCall("CID")
	if cs == nil || !cs.acceptPending {
		t.Fatalf("acceptPending = %+v, want true", cs)
	}
}

func TestAcceptCallUnknownCallReturnsError(t *testing.T) {
	cli := &Client{calls: map[string]*callState{}, Log: waLog.Noop}
	if err := cli.AcceptCall(context.Background(), "NOPE"); err == nil {
		t.Fatal("AcceptCall(unknown) = nil error, want error")
	}
}

func TestHangupCallUnknownCallReturnsError(t *testing.T) {
	cli := &Client{calls: map[string]*callState{}, Log: waLog.Noop}
	if err := cli.HangupCall(context.Background(), "NOPE"); err == nil {
		t.Fatal("HangupCall(unknown) = nil error, want error")
	}
}

func TestHangupCallSendFailureLeavesStateRegistered(t *testing.T) {
	cli := &Client{calls: map[string]*callState{}, Log: waLog.Noop}
	cli.putCall("CID", &callState{to: callStatePeerJID(), creator: callStateSelfJID()})
	err := cli.HangupCall(context.Background(), "CID")
	if err == nil || !strings.Contains(err.Error(), ErrNotConnected.Error()) {
		t.Fatalf("HangupCall send-failure error = %v, want it to wrap %v", err, ErrNotConnected)
	}
	if cs := cli.getCall("CID"); cs == nil {
		t.Error("call state was dropped despite the terminate send failing")
	}
}

func TestHangupCallEmitsCallMediaStop(t *testing.T) {
	cli, _, ce := routerTestClient()
	peer := callStatePeerJID()
	cli.putCall("CID", &callState{
		meta:    types.BasicCallMeta{CallID: "CID", From: peer, CallCreator: peer},
		to:      peer,
		creator: peer,
	})

	_ = cli.HangupCall(context.Background(), "CID")

	stops := ce.filter(isCallMediaStop)
	if len(stops) != 1 {
		t.Fatalf("CallMediaStop dispatch count = %d, want 1", len(stops))
	}
	stop, ok := stops[0].(*events.CallMediaStop)
	if !ok {
		t.Fatal("captured event is not *events.CallMediaStop")
	}
	if stop.Reason != "hangup" {
		t.Errorf("Reason = %q, want hangup", stop.Reason)
	}
	if stop.CallID != "CID" {
		t.Errorf("CallID = %q, want CID", stop.CallID)
	}
}

func TestHangupCallUnknownCallEmitsNoMediaStop(t *testing.T) {
	cli, _, ce := routerTestClient()
	_ = cli.HangupCall(context.Background(), "NOPE")
	if n := len(ce.filter(isCallMediaStop)); n != 0 {
		t.Errorf("CallMediaStop dispatch count = %d, want 0 for an unknown call", n)
	}
}

func TestSetCallMuteUnknownCallReturnsError(t *testing.T) {
	cli := &Client{calls: map[string]*callState{}, Log: waLog.Noop}
	if err := cli.SetCallMute(context.Background(), "NOPE", true); err == nil {
		t.Fatal("SetCallMute(unknown) = nil error, want error")
	}
}

func TestSetCallMuteReachesSend(t *testing.T) {
	cli := &Client{calls: map[string]*callState{}, Log: waLog.Noop}
	cli.putCall("CID", &callState{to: callStatePeerJID(), creator: callStateSelfJID()})
	err := cli.SetCallMute(context.Background(), "CID", true)
	if err == nil || !strings.Contains(err.Error(), ErrNotConnected.Error()) {
		t.Fatalf("SetCallMute send error = %v, want it to wrap %v", err, ErrNotConnected)
	}
}
