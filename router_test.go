// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type capturedLog struct {
	mu    sync.Mutex
	warns []string
}

func (l *capturedLog) Warnf(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warns = append(l.warns, fmt.Sprintf(msg, args...))
}
func (l *capturedLog) Errorf(string, ...any)   {}
func (l *capturedLog) Infof(string, ...any)    {}
func (l *capturedLog) Debugf(string, ...any)   {}
func (l *capturedLog) Sub(string) waLog.Logger { return l }

func (l *capturedLog) hasWarn(substr string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, w := range l.warns {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

type capturedEvents struct {
	mu   sync.Mutex
	evts []any
}

func (c *capturedEvents) add(evt any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evts = append(c.evts, evt)
}

func (c *capturedEvents) filter(pred func(any) bool) []any {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []any
	for _, e := range c.evts {
		if pred(e) {
			out = append(out, e)
		}
	}
	return out
}

func routerTestClient() (*Client, *capturedLog, *capturedEvents) {
	log := &capturedLog{}
	cli := &Client{calls: map[string]*callState{}, Log: log}
	ce := &capturedEvents{}
	cli.AddEventHandler(ce.add)
	return cli, log, ce
}

func routerCallMeta() types.BasicCallMeta {
	return types.BasicCallMeta{
		From:        callStatePeerJID(),
		CallCreator: callStatePeerJID(),
		CallID:      "ROUTERCID",
	}
}

func syntheticRelayNode() waBinary.Node {
	return waBinary.Node{
		Tag: "relay",
		Content: []waBinary.Node{
			{Tag: "key", Content: []byte("relay-integrity-key")},
			{Tag: "token", Attrs: waBinary.Attrs{"id": "0"}, Content: []byte("token-zero")},
			{Tag: "token", Attrs: waBinary.Attrs{"id": "1"}, Content: []byte("token-one")},
			{
				Tag: "te2",
				Attrs: waBinary.Attrs{
					"relay_id":      "5",
					"relay_name":    "gru1c02",
					"token_id":      "0",
					"auth_token_id": "1",
					"is_fna":        "0",
				},
				Content: []byte{10, 0, 0, 1, 0x1f, 0x90},
			},
		},
	}
}

func routerRelayAckNode(callID string) waBinary.Node {
	relay := syntheticRelayNode()
	relay.Attrs = waBinary.Attrs{"call-id": callID}
	return waBinary.Node{
		Tag:     "ack",
		Attrs:   waBinary.Attrs{"class": "call"},
		Content: []waBinary.Node{relay},
	}
}

func isCallOffer(e any) bool      { _, ok := e.(*events.CallOffer); return ok }
func isCallMediaReady(e any) bool { _, ok := e.(*events.CallMediaReady); return ok }
func isCallMediaStop(e any) bool  { _, ok := e.(*events.CallMediaStop); return ok }
func isCallTerminate(e any) bool  { _, ok := e.(*events.CallTerminate); return ok }

func TestOnCallOfferIgnoresAlreadyEndedOffer(t *testing.T) {
	cli, _, ce := routerTestClient()
	node := waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"from": callStatePeerJID()},
		Content: []waBinary.Node{{
			Tag:   "offer",
			Attrs: waBinary.Attrs{"call-id": "ROUTERCID", "call-creator": callStatePeerJID(), "is_call_ended": "1"},
		}},
	}
	cli.handleCallEvent(context.Background(), &node)

	if cli.getCall("ROUTERCID") != nil {
		t.Error("already-ended offer registered call state, want none")
	}
	if n := len(ce.filter(isCallOffer)); n != 0 {
		t.Errorf("CallOffer dispatch count = %d, want 0", n)
	}
}

func TestOnCallOfferUndecryptableKeyIsIgnored(t *testing.T) {
	cli, _, ce := routerTestClient()
	node := waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"from": callStatePeerJID()},
		Content: []waBinary.Node{{
			Tag:   "offer",
			Attrs: waBinary.Attrs{"call-id": "ROUTERCID", "call-creator": callStatePeerJID()},
			Content: []waBinary.Node{
				{Tag: "enc", Attrs: waBinary.Attrs{"type": "msg"}, Content: []byte("not-a-real-signal-message")},
			},
		}},
	}
	cli.handleCallEvent(context.Background(), &node)

	if cli.getCall("ROUTERCID") != nil {
		t.Error("undecryptable offer registered call state, want none")
	}
	if n := len(ce.filter(isCallOffer)); n != 0 {
		t.Errorf("CallOffer dispatch count = %d, want 0", n)
	}
}

func TestAcceptInboundOfferRegistersPreacceptsAndDispatches(t *testing.T) {
	cli, log, ce := routerTestClient()
	meta := routerCallMeta()
	remote := types.CallRemoteMeta{RemotePlatform: "android", RemoteVersion: "2.24"}
	offerChild := waBinary.Node{Tag: "offer"}
	callKey := bytes.Repeat([]byte{0x01}, 32)

	cs := cli.acceptInboundOffer(context.Background(), &offerChild, meta, remote, callKey)

	if cs == nil {
		t.Fatal("acceptInboundOffer returned nil callState")
	}
	if got := cli.getCall(meta.CallID); got != cs {
		t.Fatalf("getCall(%s) = %+v, want the registered state", meta.CallID, got)
	}
	if !bytes.Equal(cs.callKey, callKey) {
		t.Errorf("callKey = %x, want %x", cs.callKey, callKey)
	}
	if cs.to != meta.From || cs.creator != meta.CallCreator {
		t.Errorf("to/creator = %v/%v, want %v/%v", cs.to, cs.creator, meta.From, meta.CallCreator)
	}
	if cs.outgoing {
		t.Error("outgoing = true, want false for an inbound offer")
	}
	if !log.hasWarn("preaccept") {
		t.Error("expected a preaccept send attempt (observed via the ErrNotConnected warning)")
	}
	if n := len(ce.filter(isCallOffer)); n != 1 {
		t.Errorf("CallOffer dispatch count = %d, want 1", n)
	}
	if n := len(ce.filter(isCallMediaReady)); n != 0 {
		t.Errorf("CallMediaReady dispatch count = %d, want 0 (no relay yet)", n)
	}
}

func TestAcceptInboundOfferWithRelayFiresMediaReady(t *testing.T) {
	cli, _, ce := routerTestClient()
	meta := routerCallMeta()
	meta.CallID = "ROUTERCID2"
	offerChild := waBinary.Node{Tag: "offer", Content: []waBinary.Node{syntheticRelayNode()}}
	callKey := bytes.Repeat([]byte{0x02}, 32)

	cli.acceptInboundOffer(context.Background(), &offerChild, meta, types.CallRemoteMeta{}, callKey)

	if n := len(ce.filter(isCallMediaReady)); n != 1 {
		t.Fatalf("CallMediaReady dispatch count = %d, want 1", n)
	}
}

func TestAcceptInboundOfferRetransmitPreservesAcceptPending(t *testing.T) {
	cli, log, ce := routerTestClient()
	meta := routerCallMeta()
	remote := types.CallRemoteMeta{}
	callKey := bytes.Repeat([]byte{0x05}, 32)

	first := waBinary.Node{Tag: "offer"}
	cs := cli.acceptInboundOffer(context.Background(), &first, meta, remote, callKey)
	if err := cli.AcceptCall(context.Background(), meta.CallID); err != nil {
		t.Fatalf("AcceptCall: %v", err)
	}
	if !cs.acceptPending {
		t.Fatal("acceptPending not armed after AcceptCall")
	}

	second := waBinary.Node{Tag: "offer", Content: []waBinary.Node{syntheticRelayNode()}}
	cs2 := cli.acceptInboundOffer(context.Background(), &second, meta, remote, callKey)
	if cs2 != cs {
		t.Fatal("offer retransmit replaced the callState entry, want the same pointer reused")
	}
	if !cs.acceptPending {
		t.Error("acceptPending was reset by the offer retransmit, want it preserved")
	}
	if got := cli.getCall(meta.CallID); got != cs {
		t.Fatalf("getCall(%s) = %+v, want the original registered state", meta.CallID, got)
	}
	if n := len(ce.filter(isCallOffer)); n != 1 {
		t.Errorf("CallOffer dispatch count = %d, want 1 (no duplicate incoming-call notification)", n)
	}
	if n := len(ce.filter(isCallMediaReady)); n != 1 {
		t.Fatalf("CallMediaReady dispatch count = %d, want 1 (relay arrived on the retransmit)", n)
	}

	third := waBinary.Node{Tag: "offer", Content: []waBinary.Node{syntheticRelayNode()}}
	cli.acceptInboundOffer(context.Background(), &third, meta, remote, callKey)
	if n := len(ce.filter(isCallMediaReady)); n != 1 {
		t.Errorf("CallMediaReady dispatch count after a second relay-carrying offer = %d, want 1 (no duplicate)", n)
	}

	muteNode := waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"from": meta.From},
		Content: []waBinary.Node{{
			Tag:   "mute_v2",
			Attrs: waBinary.Attrs{"call-id": meta.CallID, "call-creator": meta.CallCreator, "mute-state": "1"},
		}},
	}
	cli.handleCallEvent(context.Background(), &muteNode)

	if cs.acceptPending {
		t.Error("acceptPending still true after mute_v2, want cleared by the deferred accept")
	}
	if !log.hasWarn("send call accept") {
		t.Error("expected the deferred accept send attempt to have survived the retransmit (observed via the ErrNotConnected warning)")
	}
}

func TestHandleCallAckRelayFiresMediaReadyOnceAndNotTwice(t *testing.T) {
	cli, _, ce := routerTestClient()
	cs := &callState{meta: routerCallMeta(), callKey: bytes.Repeat([]byte{3}, 32)}
	cli.putCall("ROUTERCID", cs)

	ack := routerRelayAckNode("ROUTERCID")
	cli.handleCallAck(context.Background(), &ack)
	cli.handleCallAck(context.Background(), &ack)

	ready := ce.filter(isCallMediaReady)
	if len(ready) != 1 {
		t.Fatalf("CallMediaReady dispatch count = %d, want 1 (no duplicate on second ack)", len(ready))
	}
	evt, ok := ready[0].(*events.CallMediaReady)
	if !ok {
		t.Fatal("captured event is not *events.CallMediaReady")
	}
	if evt.Relay.IPv4 != "10.0.0.1" || evt.Relay.Port != 8080 {
		t.Errorf("Relay = %+v, want 10.0.0.1:8080", evt.Relay)
	}
	if !bytes.Equal(evt.Relay.Key, []byte("relay-integrity-key")) {
		t.Errorf("Relay.Key = %q, want relay-integrity-key", evt.Relay.Key)
	}
}

func routerTransportRelayNode(callID string, peer types.JID) waBinary.Node {
	return waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"from": peer},
		Content: []waBinary.Node{{
			Tag:     "transport",
			Attrs:   waBinary.Attrs{"call-id": callID, "call-creator": peer},
			Content: []waBinary.Node{syntheticRelayNode()},
		}},
	}
}

func TestTransportRelayFiresMediaReadyOnceAndNotTwice(t *testing.T) {
	cli, _, ce := routerTestClient()
	peer := callStatePeerJID()
	cli.putCall("ROUTERCID", &callState{
		meta:    types.BasicCallMeta{CallID: "ROUTERCID", From: peer, CallCreator: peer},
		to:      peer,
		creator: peer,
		callKey: bytes.Repeat([]byte{4}, 32),
	})

	node := routerTransportRelayNode("ROUTERCID", peer)
	cli.handleCallEvent(context.Background(), &node)
	cli.handleCallEvent(context.Background(), &node)

	ready := ce.filter(isCallMediaReady)
	if len(ready) != 1 {
		t.Fatalf("CallMediaReady dispatch count = %d, want 1 (no duplicate on second transport)", len(ready))
	}
	evt, ok := ready[0].(*events.CallMediaReady)
	if !ok {
		t.Fatal("captured event is not *events.CallMediaReady")
	}
	if evt.Relay.IPv4 != "10.0.0.1" || evt.Relay.Port != 8080 {
		t.Errorf("Relay = %+v, want 10.0.0.1:8080", evt.Relay)
	}
	if !bytes.Equal(evt.Relay.Key, []byte("relay-integrity-key")) {
		t.Errorf("Relay.Key = %q, want relay-integrity-key", evt.Relay.Key)
	}

	cs := cli.getCall("ROUTERCID")
	if cs == nil {
		t.Fatal("call state disappeared after transport")
	}
	if cs.relay == nil {
		t.Fatal("cs.relay is nil, want the parsed relay")
	}
}

func TestHandleCallAckErrorDropsStateAndEmitsMediaStop(t *testing.T) {
	cli, _, ce := routerTestClient()
	cli.putCall("ROUTERCID", &callState{meta: routerCallMeta()})

	ack := waBinary.Node{
		Tag:   "ack",
		Attrs: waBinary.Attrs{"class": "call", "error": "500"},
		Content: []waBinary.Node{{
			Tag:   "error",
			Attrs: waBinary.Attrs{"call-id": "ROUTERCID"},
		}},
	}
	cli.handleCallAck(context.Background(), &ack)

	if cli.getCall("ROUTERCID") != nil {
		t.Error("call state was not dropped after a server error ack")
	}
	stops := ce.filter(isCallMediaStop)
	if len(stops) != 1 {
		t.Fatalf("CallMediaStop dispatch count = %d, want 1", len(stops))
	}
	stop, ok := stops[0].(*events.CallMediaStop)
	if !ok {
		t.Fatal("captured event is not *events.CallMediaStop")
	}
	if stop.Reason != "server:500" {
		t.Errorf("Reason = %q, want server:500", stop.Reason)
	}
}

func TestHandleCallAckIgnoresNonCallClass(t *testing.T) {
	cli, _, ce := routerTestClient()
	ack := waBinary.Node{Tag: "ack", Attrs: waBinary.Attrs{"class": "message"}}
	cli.handleCallAck(context.Background(), &ack)

	if n := len(ce.filter(func(any) bool { return true })); n != 0 {
		t.Errorf("non-call ack dispatched %d events, want 0", n)
	}
}

func TestMuteV2FirstFiresDeferredAcceptOnce(t *testing.T) {
	cli, log, ce := routerTestClient()
	peer := callStatePeerJID()
	cli.putCall("ROUTERCID", &callState{
		meta:          types.BasicCallMeta{CallID: "ROUTERCID", From: peer, CallCreator: peer},
		to:            peer,
		creator:       peer,
		acceptPending: true,
	})

	muteNode := func(state string) waBinary.Node {
		return waBinary.Node{
			Tag:   "call",
			Attrs: waBinary.Attrs{"from": peer},
			Content: []waBinary.Node{{
				Tag:   "mute_v2",
				Attrs: waBinary.Attrs{"call-id": "ROUTERCID", "call-creator": peer, "mute-state": state},
			}},
		}
	}

	first := muteNode("1")
	cli.handleCallEvent(context.Background(), &first)

	cs := cli.getCall("ROUTERCID")
	if cs == nil {
		t.Fatal("call state disappeared after mute_v2")
	}
	if cs.acceptPending {
		t.Error("acceptPending still true after first mute_v2, want cleared")
	}
	if !log.hasWarn("accept") {
		t.Error("expected an accept send attempt (observed via the ErrNotConnected warning)")
	}
	if n := len(ce.filter(func(e any) bool { m, ok := e.(*events.CallMute); return ok && m.Muted })); n != 1 {
		t.Errorf("CallMute(muted=true) dispatch count = %d, want 1", n)
	}

	second := muteNode("0")
	cli.handleCallEvent(context.Background(), &second)
	if n := len(ce.filter(func(e any) bool { m, ok := e.(*events.CallMute); return ok && !m.Muted })); n != 1 {
		t.Errorf("CallMute(muted=false) dispatch count = %d, want 1", n)
	}
}

func TestAckShouldEnqueueOnlyCallClass(t *testing.T) {
	callAck := waBinary.Node{Tag: "ack", Attrs: waBinary.Attrs{"class": "call"}}
	if !ackShouldEnqueue(&callAck) {
		t.Error("class=call ack should enqueue")
	}
	msgAck := waBinary.Node{Tag: "ack", Attrs: waBinary.Attrs{"class": "message"}}
	if ackShouldEnqueue(&msgAck) {
		t.Error("class=message ack should not enqueue")
	}
	noClassAck := waBinary.Node{Tag: "ack"}
	if ackShouldEnqueue(&noClassAck) {
		t.Error("ack with no class attr should not enqueue")
	}
}

func TestCallTerminateDropsStateAndEmitsMediaStop(t *testing.T) {
	cli, _, ce := routerTestClient()
	peer := callStatePeerJID()
	cli.putCall("ROUTERCID", &callState{meta: types.BasicCallMeta{CallID: "ROUTERCID", From: peer, CallCreator: peer}})

	node := waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"from": peer},
		Content: []waBinary.Node{{
			Tag:   "terminate",
			Attrs: waBinary.Attrs{"call-id": "ROUTERCID", "call-creator": peer, "reason": "hangup"},
		}},
	}
	cli.handleCallEvent(context.Background(), &node)

	if cli.getCall("ROUTERCID") != nil {
		t.Error("call state was not dropped on terminate")
	}
	stops := ce.filter(isCallMediaStop)
	if len(stops) != 1 {
		t.Fatalf("CallMediaStop dispatch count = %d, want 1", len(stops))
	}
	if s := stops[0].(*events.CallMediaStop); s.Reason != "hangup" {
		t.Errorf("CallMediaStop.Reason = %q, want hangup", s.Reason)
	}
	if n := len(ce.filter(isCallTerminate)); n != 1 {
		t.Errorf("CallTerminate dispatch count = %d, want 1", n)
	}
}
