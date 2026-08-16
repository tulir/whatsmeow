// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

func TestAcceptCallSendsActiveGroupAcceptImmediately(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/676ebee3eca513b5348fab36cae5c560cc791238/datasheets/voip-group-invite-accept.md#L26-L45
	creator := mustGroupInviteAcceptRouterJID(t, "100001:43@lid")
	cli := &Client{calls: map[string]*callState{}}
	cs := &callState{
		meta: types.BasicCallMeta{
			From:        creator,
			CallCreator: creator,
			CallID:      "ACTIVE-AD-HOC-CALL",
		},
		to:      creator,
		creator: creator,
		group: &groupCallState{snapshot: types.GroupCallUpdate{
			CallID:        "ACTIVE-AD-HOC-CALL",
			CallCreator:   creator,
			TransactionID: 7,
		}},
	}
	cli.putCall(cs.meta.CallID, cs)

	var sent []waBinary.Node
	err := cli.acceptCallWithDependencies(
		context.Background(),
		cs.meta.CallID,
		func() string { return "request-id" },
		func(_ context.Context, node waBinary.Node) error {
			sent = append(sent, node)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("acceptCallWithDependencies: %v", err)
	}
	if len(sent) != 1 {
		t.Fatalf("sent node count = %d, want 1", len(sent))
	}
	if !cs.connected {
		t.Fatal("successful active-group accept did not mark the exact call state connected")
	}
	if cs.acceptPending {
		t.Fatal("active-group accept armed the direct-call deferred accept")
	}

	node := sent[0]
	if node.Tag != "call" {
		t.Fatalf("wrapper tag = %q, want call", node.Tag)
	}
	if got := node.AttrGetter().JID("to"); got != types.NewJID(cs.meta.CallID, "call") {
		t.Fatalf("accept target = %s, want %s@call", got, cs.meta.CallID)
	}
	if got := node.AttrGetter().String("id"); got != "request-id" {
		t.Fatalf("wrapper id = %q, want request-id", got)
	}
	actions := node.GetChildren()
	if len(actions) != 1 || actions[0].Tag != "accept" {
		t.Fatalf("wrapper children = %+v, want one accept", actions)
	}
	accept := actions[0]
	if got := accept.AttrGetter().String("call-id"); got != cs.meta.CallID {
		t.Fatalf("accept call-id = %q, want %q", got, cs.meta.CallID)
	}
	if got := accept.AttrGetter().JID("call-creator"); got != creator {
		t.Fatalf("accept call-creator = %s, want %s", got, creator)
	}
	children := accept.GetChildren()
	if len(children) != 3 {
		t.Fatalf("accept child count = %d, want 3: audio, net, encopt", len(children))
	}
	wantTags := []string{"audio", "net", "encopt"}
	for i, want := range wantTags {
		if children[i].Tag != want {
			t.Fatalf("accept child %d tag = %q, want %q", i, children[i].Tag, want)
		}
	}
	if got := children[0].AttrGetter().String("enc"); got != "opus" {
		t.Fatalf("audio enc = %q, want opus", got)
	}
	if got := children[0].AttrGetter().String("rate"); got != "16000" {
		t.Fatalf("audio rate = %q, want 16000", got)
	}
	if got := children[1].AttrGetter().String("medium"); got != "2" {
		t.Fatalf("net medium = %q, want 2", got)
	}
	if got := children[2].AttrGetter().String("keygen"); got != "2" {
		t.Fatalf("encopt keygen = %q, want 2", got)
	}
	for _, child := range children {
		if child.Tag == "metadata" {
			t.Fatal("active-group accept contains forbidden 1:1 metadata")
		}
	}
}

func TestAcceptCallActiveGroupSendFailureRemainsRetryable(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/676ebee3eca513b5348fab36cae5c560cc791238/datasheets/voip-group-invite-accept.md#L26-L45
	creator := mustGroupInviteAcceptRouterJID(t, "100001:43@lid")
	cli := &Client{calls: map[string]*callState{}}
	cs := &callState{
		meta:    types.BasicCallMeta{CallID: "ACTIVE-AD-HOC-CALL", CallCreator: creator},
		to:      creator,
		creator: creator,
		group:   &groupCallState{},
	}
	cli.putCall(cs.meta.CallID, cs)
	sendFailure := errors.New("synthetic send failure")
	sendCalls := 0
	send := func(context.Context, waBinary.Node) error {
		sendCalls++
		if sendCalls == 1 {
			return sendFailure
		}
		return nil
	}

	err := cli.acceptCallWithDependencies(
		context.Background(),
		cs.meta.CallID,
		func() string { return "request-id" },
		send,
	)
	if !errors.Is(err, sendFailure) {
		t.Fatalf("first accept error = %v, want it to wrap %v", err, sendFailure)
	}
	if cs.connected {
		t.Fatal("failed active-group accept marked call connected")
	}
	if cs.acceptPending {
		t.Fatal("failed active-group accept armed the direct-call deferred accept")
	}

	err = cli.acceptCallWithDependencies(
		context.Background(),
		cs.meta.CallID,
		func() string { return "request-id-retry" },
		send,
	)
	if err != nil {
		t.Fatalf("retry active-group accept: %v", err)
	}
	if sendCalls != 2 {
		t.Fatalf("send calls = %d, want 2", sendCalls)
	}
	if !cs.connected {
		t.Fatal("successful retry did not mark call connected")
	}
}

func TestAcceptCallActiveGroupSequentialSuccessSendsOnce(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/412d4b75759ed65f6182703398be47ca109f4e19/datasheets/voip-group-invite-accept.md#L96-L106
	creator := mustGroupInviteAcceptRouterJID(t, "100001:43@lid")
	cli := &Client{calls: map[string]*callState{}}
	cs := &callState{
		meta:    types.BasicCallMeta{CallID: "ACTIVE-AD-HOC-CALL", CallCreator: creator},
		to:      creator,
		creator: creator,
		group:   &groupCallState{},
	}
	cli.putCall(cs.meta.CallID, cs)
	requestIDs := 0
	sendCalls := 0
	requestID := func() string {
		requestIDs++
		return "request-id"
	}
	send := func(context.Context, waBinary.Node) error {
		sendCalls++
		return nil
	}

	for i := 0; i < 2; i++ {
		if err := cli.acceptCallWithDependencies(
			context.Background(),
			cs.meta.CallID,
			requestID,
			send,
		); err != nil {
			t.Fatalf("accept call %d: %v", i+1, err)
		}
	}
	if sendCalls != 1 {
		t.Fatalf("send calls = %d, want 1", sendCalls)
	}
	if requestIDs != 1 {
		t.Fatalf("request ID calls = %d, want 1", requestIDs)
	}
	if !cs.connected {
		t.Fatal("sequential successful accept did not leave the call connected")
	}
}

func TestAcceptCallActiveGroupConcurrentBlockedSendSendsOnce(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/412d4b75759ed65f6182703398be47ca109f4e19/datasheets/voip-group-invite-accept.md#L96-L106
	creator := mustGroupInviteAcceptRouterJID(t, "100001:43@lid")
	cli := &Client{calls: map[string]*callState{}}
	cs := &callState{
		meta:    types.BasicCallMeta{CallID: "ACTIVE-AD-HOC-CALL", CallCreator: creator},
		to:      creator,
		creator: creator,
		group:   &groupCallState{},
	}
	cli.putCall(cs.meta.CallID, cs)
	sendStarted := make(chan struct{}, 2)
	releaseSend := make(chan struct{})
	send := func(context.Context, waBinary.Node) error {
		sendStarted <- struct{}{}
		<-releaseSend
		return nil
	}
	accept := func() error {
		return cli.acceptCallWithDependencies(
			context.Background(),
			cs.meta.CallID,
			func() string { return "request-id" },
			send,
		)
	}

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- accept()
	}()
	<-sendStarted

	secondDone := make(chan error, 1)
	go func() {
		secondDone <- accept()
	}()

	select {
	case err := <-secondDone:
		close(releaseSend)
		<-firstDone
		t.Fatalf("coalesced accept returned before the shared send completed: %v", err)
	case <-sendStarted:
		close(releaseSend)
		<-firstDone
		<-secondDone
		t.Fatal("concurrent accept entered a second send")
	case <-time.After(25 * time.Millisecond):
	}

	close(releaseSend)
	if err := <-firstDone; err != nil {
		t.Fatalf("first accept: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("coalesced accept: %v", err)
	}
	if !cs.connected {
		t.Fatal("successful blocked send did not mark the call connected")
	}
}

func TestAcceptCallActiveGroupConcurrentFailureSharesResultAndRetries(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/412d4b75759ed65f6182703398be47ca109f4e19/datasheets/voip-group-invite-accept.md#L96-L106
	creator := mustGroupInviteAcceptRouterJID(t, "100001:43@lid")
	cli := &Client{calls: map[string]*callState{}}
	cs := &callState{
		meta:    types.BasicCallMeta{CallID: "ACTIVE-AD-HOC-CALL", CallCreator: creator},
		to:      creator,
		creator: creator,
		group:   &groupCallState{},
	}
	cli.putCall(cs.meta.CallID, cs)
	sendFailure := errors.New("synthetic send failure")
	sendStarted := make(chan int32, 2)
	releaseFirstSend := make(chan struct{})
	var sendCalls atomic.Int32
	send := func(context.Context, waBinary.Node) error {
		attempt := sendCalls.Add(1)
		sendStarted <- attempt
		if attempt == 1 {
			<-releaseFirstSend
			return sendFailure
		}
		return nil
	}
	accept := func() error {
		return cli.acceptCallWithDependencies(
			context.Background(),
			cs.meta.CallID,
			func() string { return "request-id" },
			send,
		)
	}

	firstDone := make(chan error, 1)
	go func() {
		firstDone <- accept()
	}()
	if attempt := <-sendStarted; attempt != 1 {
		t.Fatalf("first send attempt = %d, want 1", attempt)
	}

	secondDone := make(chan error, 1)
	go func() {
		secondDone <- accept()
	}()

	select {
	case err := <-secondDone:
		close(releaseFirstSend)
		<-firstDone
		t.Fatalf("coalesced accept returned before the shared send failed: %v", err)
	case attempt := <-sendStarted:
		close(releaseFirstSend)
		<-firstDone
		<-secondDone
		t.Fatalf("concurrent accept entered send attempt %d, want one shared attempt", attempt)
	case <-time.After(25 * time.Millisecond):
	}

	close(releaseFirstSend)
	firstErr := <-firstDone
	secondErr := <-secondDone
	if !errors.Is(firstErr, sendFailure) {
		t.Fatalf("first accept error = %v, want it to wrap %v", firstErr, sendFailure)
	}
	if !errors.Is(secondErr, sendFailure) {
		t.Fatalf("coalesced accept error = %v, want it to wrap %v", secondErr, sendFailure)
	}
	if firstErr != secondErr {
		t.Fatalf("coalesced callers received different attempt errors: %T and %T", firstErr, secondErr)
	}
	if got := sendCalls.Load(); got != 1 {
		t.Fatalf("failed shared send calls = %d, want 1", got)
	}
	if cs.connected {
		t.Fatal("failed shared send marked call connected")
	}

	if err := accept(); err != nil {
		t.Fatalf("retry after shared failure: %v", err)
	}
	if got := sendCalls.Load(); got != 2 {
		t.Fatalf("send calls after retry = %d, want 2", got)
	}
	if !cs.connected {
		t.Fatal("successful retry did not mark call connected")
	}
}

func TestAcceptCallActiveGroupDoesNotConnectReplacementState(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/676ebee3eca513b5348fab36cae5c560cc791238/datasheets/voip-group-invite-accept.md#L74-L104
	creator := mustGroupInviteAcceptRouterJID(t, "100001:43@lid")
	cli := &Client{calls: map[string]*callState{}}
	original := &callState{
		meta:    types.BasicCallMeta{CallID: "ACTIVE-AD-HOC-CALL", CallCreator: creator},
		to:      creator,
		creator: creator,
		group:   &groupCallState{},
	}
	replacement := &callState{
		meta:    original.meta,
		to:      creator,
		creator: creator,
		group:   &groupCallState{},
	}
	cli.putCall(original.meta.CallID, original)

	err := cli.acceptCallWithDependencies(
		context.Background(),
		original.meta.CallID,
		func() string { return "request-id" },
		func(context.Context, waBinary.Node) error {
			cli.putCall(original.meta.CallID, replacement)
			return nil
		},
	)
	if err == nil || !strings.Contains(err.Error(), "call state changed") {
		t.Fatalf("replacement-state error = %v, want call state changed", err)
	}
	if original.connected || replacement.connected {
		t.Fatalf("connected state = original %t replacement %t, want both false",
			original.connected, replacement.connected)
	}
}

func TestAcceptCallActiveGroupFailureDoesNotClearReplacementInFlight(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/412d4b75759ed65f6182703398be47ca109f4e19/datasheets/voip-group-invite-accept.md#L96-L106
	creator := mustGroupInviteAcceptRouterJID(t, "100001:43@lid")
	cli := &Client{calls: map[string]*callState{}}
	original := &callState{
		meta:    types.BasicCallMeta{CallID: "ACTIVE-AD-HOC-CALL", CallCreator: creator},
		to:      creator,
		creator: creator,
		group:   &groupCallState{},
	}
	replacement := &callState{
		meta:    original.meta,
		to:      creator,
		creator: creator,
		group:   &groupCallState{},
		acceptAttempt: &callAcceptAttempt{
			done: make(chan struct{}),
		},
	}
	replacementAttempt := replacement.acceptAttempt
	cli.putCall(original.meta.CallID, original)
	sendFailure := errors.New("synthetic send failure")

	err := cli.acceptCallWithDependencies(
		context.Background(),
		original.meta.CallID,
		func() string { return "request-id" },
		func(context.Context, waBinary.Node) error {
			cli.putCall(original.meta.CallID, replacement)
			return sendFailure
		},
	)
	if !errors.Is(err, sendFailure) {
		t.Fatalf("replacement-state error = %v, want it to wrap %v", err, sendFailure)
	}
	if replacement.acceptAttempt != replacementAttempt {
		t.Fatal("failed send for stale state cleared replacement accept-in-flight")
	}
	if original.connected || replacement.connected {
		t.Fatalf("connected state = original %t replacement %t, want both false",
			original.connected, replacement.connected)
	}
}

func TestOnCallOfferRegistersActiveGroupInviteWithoutOfferKey(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L21-L54
	self := mustGroupInviteAcceptRouterJID(t, "300003:14@lid")
	creator := mustGroupInviteAcceptRouterJID(t, "100001:1@lid")
	peer := mustGroupInviteAcceptRouterJID(t, "200002@lid")
	cli, log, captured := routerTestClient()
	cli.Store = &store.Device{LID: self}
	offer := waBinary.Node{
		Tag: "offer",
		Attrs: waBinary.Attrs{
			"call-id":      "ACTIVE-AD-HOC-CALL",
			"call-creator": creator,
			"joinable":     "1",
		},
		Content: []waBinary.Node{{
			Tag: "group_info",
			Attrs: waBinary.Attrs{
				"call-id":         "ACTIVE-AD-HOC-CALL",
				"call-creator":    creator,
				"transaction-id":  "7",
				"media":           "audio",
				"connected-limit": "32",
			},
			Content: []waBinary.Node{
				groupInviteAcceptRouterParticipant(creator.ToNonAD(), creator, "connected", "web", true),
				groupInviteAcceptRouterParticipant(peer.ToNonAD(), peer, "connected", "iphone", true),
				groupInviteAcceptRouterParticipant(self.ToNonAD(), self, "outgoing", "", false),
			},
		}},
	}
	meta := types.BasicCallMeta{
		From:        creator,
		CallCreator: creator,
		CallID:      "ACTIVE-AD-HOC-CALL",
	}

	cli.onCallOffer(context.Background(), &offer, meta, types.CallRemoteMeta{RemotePlatform: "web"})

	cs := cli.getCall(meta.CallID)
	if cs == nil {
		t.Fatal("active group invite did not register call state")
	}
	if cs.group == nil {
		t.Fatal("active group invite did not seed group state")
	}
	if cs.group.snapshot.TransactionID != 7 || !cs.group.snapshot.Joinable {
		t.Fatalf("seeded group state = transaction %d, joinable %t",
			cs.group.snapshot.TransactionID, cs.group.snapshot.Joinable)
	}
	if cs.callKey != nil {
		t.Fatalf("pending active group invite has %d call-key bytes, want nil", len(cs.callKey))
	}
	if got := cs.signalingTarget(); got != types.NewJID(meta.CallID, "call") {
		t.Fatalf("signaling target = %s, want %s@call", got, meta.CallID)
	}
	if n := len(captured.filter(isCallOffer)); n != 1 {
		t.Fatalf("CallOffer dispatch count = %d, want 1", n)
	}
	if n := len(captured.filter(isCallMediaReady)); n != 0 {
		t.Fatalf("CallMediaReady dispatch count = %d, want 0 before rekey", n)
	}
	if log.hasWarn("Failed to decrypt call key") {
		t.Fatal("active group invite incorrectly entered the 1:1 key-decrypt path")
	}
}

func TestOnCallOfferStillRejectsOneToOneOfferWithoutKey(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L30-L40
	creator := mustGroupInviteAcceptRouterJID(t, "100001:1@lid")
	cli, log, captured := routerTestClient()
	cli.Store = &store.Device{LID: mustGroupInviteAcceptRouterJID(t, "300003:14@lid")}
	offer := waBinary.Node{
		Tag: "offer",
		Attrs: waBinary.Attrs{
			"call-id":      "ONE-TO-ONE-CALL",
			"call-creator": creator,
		},
	}
	meta := types.BasicCallMeta{
		From:        creator,
		CallCreator: creator,
		CallID:      "ONE-TO-ONE-CALL",
	}

	cli.onCallOffer(context.Background(), &offer, meta, types.CallRemoteMeta{})

	if cs := cli.getCall(meta.CallID); cs != nil {
		t.Fatalf("keyless 1:1 offer registered call state: %+v", cs)
	}
	if n := len(captured.filter(isCallOffer)); n != 0 {
		t.Fatalf("CallOffer dispatch count = %d, want 0", n)
	}
	if !log.hasWarn("Failed to decrypt call key") {
		t.Fatal("keyless 1:1 offer did not report the missing encrypted key")
	}
}

func TestAcceptInboundGroupOfferRetransmitPreservesInstalledCallKey(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L47-L54
	self := mustGroupInviteAcceptRouterJID(t, "300003:14@lid")
	creator := mustGroupInviteAcceptRouterJID(t, "100001:1@lid")
	cli, _, _ := routerTestClient()
	cli.Store = &store.Device{LID: self}
	installedKey := []byte("already-installed-group-call-key")
	cs := &callState{
		meta: types.BasicCallMeta{
			From:        creator,
			CallCreator: creator,
			CallID:      "ACTIVE-AD-HOC-CALL",
		},
		selfLID: self,
		to:      creator,
		creator: creator,
		callKey: append([]byte(nil), installedKey...),
		group: &groupCallState{snapshot: types.GroupCallUpdate{
			CallID:        "ACTIVE-AD-HOC-CALL",
			CallCreator:   creator,
			TransactionID: 7,
			Joinable:      true,
		}},
	}
	cli.putCall(cs.meta.CallID, cs)
	offer := waBinary.Node{
		Tag: "offer",
		Content: []waBinary.Node{groupInviteAcceptRouterParticipant(
			creator.ToNonAD(), creator, "connected", "web", true,
		)},
	}

	cli.acceptInboundOffer(
		context.Background(),
		&offer,
		cs.meta,
		types.CallRemoteMeta{},
		nil,
		&cs.group.snapshot,
	)

	if string(cs.callKey) != string(installedKey) {
		t.Fatalf("retransmitted group offer replaced installed key with %d bytes", len(cs.callKey))
	}
}

func TestBuildDeferredCallAcceptUsesCallScopedGroupTarget(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L35-L39
	creator := mustGroupInviteAcceptRouterJID(t, "100001:1@lid")
	meta := types.BasicCallMeta{
		From:        creator,
		CallCreator: creator,
		CallID:      "ACTIVE-AD-HOC-CALL",
	}
	cs := &callState{
		to:   creator,
		meta: meta,
		group: &groupCallState{snapshot: types.GroupCallUpdate{
			CallID:        meta.CallID,
			CallCreator:   creator,
			TransactionID: 7,
		}},
	}

	accept := buildDeferredCallAccept(cs, meta, false)

	if got := accept.AttrGetter().JID("to"); got != types.NewJID(meta.CallID, "call") {
		t.Fatalf("group accept target = %s, want %s@call", got, meta.CallID)
	}
}

func TestBuildDeferredCallAcceptPreservesDirectTarget(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L30-L39
	peer := mustGroupInviteAcceptRouterJID(t, "200002:1@lid")
	creator := mustGroupInviteAcceptRouterJID(t, "100001:1@lid")
	meta := types.BasicCallMeta{
		From:        peer,
		CallCreator: creator,
		CallID:      "ONE-TO-ONE-CALL",
	}
	cs := &callState{to: peer, meta: meta}

	accept := buildDeferredCallAccept(cs, meta, false)

	if got := accept.AttrGetter().JID("to"); got != peer {
		t.Fatalf("direct accept target = %s, want %s", got, peer)
	}
}

func TestBuildInboundPreacceptUsesCallScopedGroupTarget(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L35-L39
	creator := mustGroupInviteAcceptRouterJID(t, "100001:1@lid")
	meta := types.BasicCallMeta{
		From:        creator,
		CallCreator: creator,
		CallID:      "ACTIVE-AD-HOC-CALL",
	}
	cs := &callState{
		to:   creator,
		meta: meta,
		group: &groupCallState{snapshot: types.GroupCallUpdate{
			CallID:        meta.CallID,
			CallCreator:   creator,
			TransactionID: 7,
		}},
	}

	preaccept := buildInboundPreaccept(cs, meta, "request-id", false)

	if got := preaccept.AttrGetter().JID("to"); got != types.NewJID(meta.CallID, "call") {
		t.Fatalf("group preaccept target = %s, want %s@call", got, meta.CallID)
	}
}

func TestBuildInboundPreacceptPreservesDirectTarget(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L30-L39
	peer := mustGroupInviteAcceptRouterJID(t, "200002:1@lid")
	creator := mustGroupInviteAcceptRouterJID(t, "100001:1@lid")
	meta := types.BasicCallMeta{
		From:        peer,
		CallCreator: creator,
		CallID:      "ONE-TO-ONE-CALL",
	}
	cs := &callState{to: peer, meta: meta}

	preaccept := buildInboundPreaccept(cs, meta, "request-id", false)

	if got := preaccept.AttrGetter().JID("to"); got != peer {
		t.Fatalf("direct preaccept target = %s, want %s", got, peer)
	}
}

func groupInviteAcceptRouterParticipant(
	user, device types.JID,
	state, platform string,
	withCapability bool,
) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L21-L40
	var deviceContent []waBinary.Node
	if withCapability {
		deviceContent = []waBinary.Node{{
			Tag:     "capability",
			Attrs:   waBinary.Attrs{"ver": "1"},
			Content: []byte{1, 5, 247, 9, 224, 187, 83},
		}}
	}
	return waBinary.Node{
		Tag: "user",
		Attrs: waBinary.Attrs{
			"jid":   user,
			"state": state,
		},
		Content: []waBinary.Node{{
			Tag: "device",
			Attrs: waBinary.Attrs{
				"jid":      device,
				"platform": platform,
			},
			Content: deviceContent,
		}},
	}
}

func mustGroupInviteAcceptRouterJID(t *testing.T, raw string) types.JID {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L21-L40
	t.Helper()
	jid, err := types.ParseJID(raw)
	if err != nil {
		t.Fatalf("parse JID %q: %v", raw, err)
	}
	return jid
}
