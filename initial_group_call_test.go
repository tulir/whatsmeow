// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/voip"
)

type initialGroupStartCorpus struct {
	Schema string                    `json:"schema"`
	Offer  initialGroupStartOffer    `json:"offer"`
	Ack    initialGroupStartSnapshot `json:"ack"`
	Ready  initialGroupStartSnapshot `json:"ready"`
}

type initialGroupStartOffer struct {
	CallID        string                         `json:"call_id"`
	CallCreator   string                         `json:"call_creator"`
	GroupBoundJID string                         `json:"group_bound_jid"`
	Participants  []initialGroupStartParticipant `json:"participants"`
}

type initialGroupStartSnapshot struct {
	TransactionID  uint32                         `json:"transaction_id"`
	Media          string                         `json:"media"`
	ConnectedLimit uint32                         `json:"connected_limit"`
	Joinable       bool                           `json:"joinable"`
	Participants   []initialGroupStartParticipant `json:"participants"`
	Relay          initialGroupStartRelay         `json:"relay"`
}

type initialGroupStartParticipant struct {
	JID     string                    `json:"jid"`
	State   string                    `json:"state"`
	Devices []initialGroupStartDevice `json:"devices"`
}

type initialGroupStartDevice struct {
	JID string  `json:"jid"`
	PID *uint32 `json:"pid"`
}

type initialGroupStartRelay struct {
	TransactionID    uint32                      `json:"transaction_id"`
	SelfPID          uint32                      `json:"self_pid"`
	KeyLength        int                         `json:"key_length"`
	KeyFill          byte                        `json:"key_fill"`
	TokenLengths     []int                       `json:"token_lengths"`
	TokenFills       []byte                      `json:"token_fills"`
	AuthTokenLengths []int                       `json:"auth_token_lengths"`
	AuthTokenFills   []byte                      `json:"auth_token_fills"`
	Endpoints        []initialGroupStartEndpoint `json:"endpoints"`
}

type initialGroupStartEndpoint struct {
	RelayID     uint32 `json:"relay_id"`
	TokenID     uint32 `json:"token_id"`
	AuthTokenID uint32 `json:"auth_token_id"`
	RelayName   string `json:"relay_name"`
	RTT         uint32 `json:"rtt"`
	IsFNA       bool   `json:"is_fna"`
	AddressHex  string `json:"address_hex"`
}

func loadInitialGroupStartCorpus(t *testing.T) initialGroupStartCorpus {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L47-L111
	t.Helper()
	raw, err := os.ReadFile("testdata/initial_group_call_corpus.json")
	if err != nil {
		t.Fatalf("read initial group call corpus: %v", err)
	}
	var corpus initialGroupStartCorpus
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatalf("decode initial group call corpus: %v", err)
	}
	if corpus.Schema != "whatsmeow.initial-group-call-corpus.v1" {
		t.Fatalf("corpus schema = %q", corpus.Schema)
	}
	return corpus
}

func TestRunInitialGroupCallStartInstallsExactStateBeforeSendingOrderedOffer(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L164-L181
	self := mustInitialGroupStartJID(t, "100001:14@lid")
	rawTargets := []types.JID{
		mustInitialGroupStartJID(t, "111111@s.whatsapp.net"),
		mustInitialGroupStartJID(t, "222222@lid"),
		mustInitialGroupStartJID(t, "333333@s.whatsapp.net"),
	}
	lids := []types.JID{
		mustInitialGroupStartJID(t, "200001@lid"),
		mustInitialGroupStartJID(t, "200002@lid"),
		mustInitialGroupStartJID(t, "200003@lid"),
	}
	devices := []types.JID{
		mustInitialGroupStartJID(t, "200003:9@lid"),
		mustInitialGroupStartJID(t, "200001:4@lid"),
		mustInitialGroupStartJID(t, "200002@lid"),
		mustInitialGroupStartJID(t, "200001@lid"),
	}
	groupJID := mustInitialGroupStartJID(t, "120363411251996986@g.us")
	var resolvedInputs []types.JID
	var discoveredInputs []types.JID
	var sent waBinary.Node
	var installedCallID string
	var installed *callState
	var stateAtSend *callState

	gotCallID, err := runInitialGroupCallStart(
		context.Background(),
		self,
		rawTargets,
		GroupCallOfferOptions{GroupJID: groupJID},
		initialGroupCallStartDependencies{
			resolve: func(_ context.Context, target types.JID) (types.JID, error) {
				resolvedInputs = append(resolvedInputs, target)
				for i := range rawTargets {
					if target == rawTargets[i] {
						return lids[i], nil
					}
				}
				return types.EmptyJID, errors.New("unexpected target")
			},
			discover: func(_ context.Context, targets []types.JID) ([]types.JID, error) {
				discoveredInputs = append(discoveredInputs, targets...)
				return append([]types.JID(nil), devices...), nil
			},
			callID:    func() string { return "GROUP-CALL-ID" },
			requestID: func() string { return "request-id" },
			send: func(_ context.Context, node waBinary.Node) error {
				stateAtSend = installed
				sent = node
				return nil
			},
			install: func(callID string, cs *callState) {
				installedCallID = callID
				installed = cs
			},
			remove: func(string, *callState) {
				t.Fatal("successful group start removed its provisional call state")
			},
		},
	)
	if err != nil {
		t.Fatalf("runInitialGroupCallStart: %v", err)
	}
	if gotCallID != "GROUP-CALL-ID" {
		t.Fatalf("call ID = %q, want GROUP-CALL-ID", gotCallID)
	}
	if !equalInitialGroupJIDs(resolvedInputs, rawTargets) {
		t.Fatalf("resolved targets = %v, want %v", resolvedInputs, rawTargets)
	}
	if !equalInitialGroupJIDs(discoveredInputs, lids) {
		t.Fatalf("device discovery targets = %v, want %v", discoveredInputs, lids)
	}
	if stateAtSend == nil {
		t.Fatal("offer send could not observe the provisional call state")
	}
	if installed == nil || installedCallID != "GROUP-CALL-ID" {
		t.Fatalf("installed state = %p for %q", installed, installedCallID)
	}
	if installed != stateAtSend {
		t.Fatalf("successful send retained state %p, want exact provisional state %p", installed, stateAtSend)
	}
	if installed.group == nil || installed.group.snapshot.CallID != "GROUP-CALL-ID" ||
		installed.group.snapshot.GroupJID != groupJID {
		t.Fatalf("installed group snapshot = %+v", installed.group)
	}
	if got := installed.signalingTarget(); got != types.NewJID("GROUP-CALL-ID", "call") {
		t.Fatalf("signaling target = %s, want GROUP-CALL-ID@call", got)
	}
	if installed.peerLID != lids[0] || installed.to != lids[0] {
		t.Fatalf("legacy peer/to = %s/%s, want first target %s", installed.peerLID, installed.to, lids[0])
	}
	if !installed.outgoing || installed.callKey != nil || installed.hasGroupKeyEpoch {
		t.Fatalf("initial group key/direction state = outgoing %t key %x has_epoch %t",
			installed.outgoing, installed.callKey, installed.hasGroupKeyEpoch)
	}
	if installed.meta.GroupJID != groupJID {
		t.Fatalf("meta group JID = %s, want %s", installed.meta.GroupJID, groupJID)
	}
	if !bytes.Equal(installed.inviteSelfDevice.Capability, voip.CapabilityOffer) {
		t.Fatalf("local capability = %x, want Go client capability %x",
			installed.inviteSelfDevice.Capability, voip.CapabilityOffer)
	}

	if got := sent.AttrGetter().JID("to"); got != types.NewJID("GROUP-CALL-ID", "call") {
		t.Fatalf("offer target = %s, want GROUP-CALL-ID@call", got)
	}
	if got := sent.AttrGetter().String("id"); got != "request-id" {
		t.Fatalf("offer stanza ID = %q, want request-id", got)
	}
	offer := sent.GetChildren()[0]
	if got := offer.AttrGetter().JID("group-jid"); got != groupJID {
		t.Fatalf("offer group JID = %s, want %s", got, groupJID)
	}
	groupInfo := offer.GetChildByTag("group_info")
	users := groupInfo.GetChildren()
	wantUsers := []types.JID{self.ToNonAD(), lids[0], lids[1], lids[2]}
	if len(users) != len(wantUsers) {
		t.Fatalf("offer users = %d, want %d", len(users), len(wantUsers))
	}
	for i := range wantUsers {
		if got := users[i].AttrGetter().JID("jid"); got != wantUsers[i] {
			t.Errorf("offer user %d = %s, want %s", i, got, wantUsers[i])
		}
	}
	wantDevices := [][]types.JID{
		{self},
		{devices[1], devices[3]},
		{devices[2]},
		{devices[0]},
	}
	for i, want := range wantDevices {
		gotDevices := users[i].GetChildren()
		if len(gotDevices) != len(want) {
			t.Fatalf("offer user %d devices = %d, want %d", i, len(gotDevices), len(want))
		}
		for j := range want {
			if got := gotDevices[j].AttrGetter().JID("jid"); got != want[j] {
				t.Errorf("offer user %d device %d = %s, want %s", i, j, got, want[j])
			}
		}
	}
	selfCapability := users[0].GetChildren()[0].GetChildByTag("capability")
	if got := selfCapability.AttrGetter().String("ver"); got != "1" ||
		!bytes.Equal(selfCapability.Content.([]byte), voip.CapabilityOffer) {
		t.Fatalf("self capability = attrs %#v content %x", selfCapability.Attrs, selfCapability.Content)
	}
	for i := 1; i < len(users); i++ {
		for _, device := range users[i].GetChildren() {
			if len(device.GetChildren()) != 0 {
				t.Fatalf("remote user %d device %s advertised capability", i, device.AttrGetter().JID("jid"))
			}
		}
	}
}

func TestRunInitialGroupCallStartRejectsInvalidOrFailedPreparationWithoutState(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L164-L179
	self := mustInitialGroupStartJID(t, "100001:14@lid")
	first := mustInitialGroupStartJID(t, "200001@lid")
	second := mustInitialGroupStartJID(t, "200002@lid")
	third := mustInitialGroupStartJID(t, "200003@lid")
	discoveryFailure := errors.New("discovery failed")
	sendFailure := errors.New("send failed")
	cases := []struct {
		name    string
		self    types.JID
		targets []types.JID
		edit    func(*initialGroupCallStartDependencies)
		want    string
	}{
		{name: "missing self", targets: []types.JID{first, second}, want: "not logged in"},
		{name: "one target", self: self, targets: []types.JID{first}, want: "at least two"},
		{name: "empty target", self: self, targets: []types.JID{first, types.EmptyJID}, want: "target 1 is empty"},
		{name: "self target", self: self, targets: []types.JID{self.ToNonAD(), second}, want: "target 0 is self"},
		{name: "duplicate resolved target", self: self, targets: []types.JID{first, second}, edit: func(deps *initialGroupCallStartDependencies) {
			deps.resolve = func(context.Context, types.JID) (types.JID, error) { return first, nil }
		}, want: "duplicate target"},
		{name: "resolution failure", self: self, targets: []types.JID{first, second}, edit: func(deps *initialGroupCallStartDependencies) {
			deps.resolve = func(context.Context, types.JID) (types.JID, error) {
				return types.EmptyJID, errors.New("resolve failed")
			}
		}, want: "resolve failed"},
		{name: "discovery failure", self: self, targets: []types.JID{first, second}, edit: func(deps *initialGroupCallStartDependencies) {
			deps.discover = func(context.Context, []types.JID) ([]types.JID, error) {
				return nil, discoveryFailure
			}
		}, want: discoveryFailure.Error()},
		{name: "missing target devices", self: self, targets: []types.JID{first, second}, edit: func(deps *initialGroupCallStartDependencies) {
			deps.discover = func(context.Context, []types.JID) ([]types.JID, error) {
				return []types.JID{first}, nil
			}
		}, want: "has no devices"},
		{name: "send failure", self: self, targets: []types.JID{first, second, third}, edit: func(deps *initialGroupCallStartDependencies) {
			deps.send = func(context.Context, waBinary.Node) error { return sendFailure }
		}, want: sendFailure.Error()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var installed *callState
			deps := initialGroupCallStartDependencies{
				resolve: func(_ context.Context, target types.JID) (types.JID, error) {
					return target.ToNonAD(), nil
				},
				discover: func(_ context.Context, targets []types.JID) ([]types.JID, error) {
					return append([]types.JID(nil), targets...), nil
				},
				callID:    func() string { return "CID" },
				requestID: func() string { return "request-id" },
				send:      func(context.Context, waBinary.Node) error { return nil },
				install:   func(_ string, state *callState) { installed = state },
				remove: func(_ string, state *callState) {
					if installed == state {
						installed = nil
					}
				},
			}
			if tc.edit != nil {
				tc.edit(&deps)
			}
			_, err := runInitialGroupCallStart(
				context.Background(),
				tc.self,
				tc.targets,
				GroupCallOfferOptions{},
				deps,
			)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
			if installed != nil {
				t.Fatalf("failed group start retained call state %p", installed)
			}
		})
	}
}

func TestRunInitialGroupCallStartSendFailurePreservesReplacementState(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L178-L180
	self := mustInitialGroupStartJID(t, "100001:14@lid")
	targets := []types.JID{
		mustInitialGroupStartJID(t, "200001@lid"),
		mustInitialGroupStartJID(t, "200002@lid"),
	}
	replacement := &callState{meta: types.BasicCallMeta{CallID: "replacement"}}
	var installed *callState
	var provisional *callState
	_, err := runInitialGroupCallStart(
		context.Background(),
		self,
		targets,
		GroupCallOfferOptions{},
		initialGroupCallStartDependencies{
			resolve: func(_ context.Context, target types.JID) (types.JID, error) {
				return target.ToNonAD(), nil
			},
			discover: func(_ context.Context, targets []types.JID) ([]types.JID, error) {
				return append([]types.JID(nil), targets...), nil
			},
			callID:    func() string { return "CID" },
			requestID: func() string { return "request-id" },
			install: func(_ string, state *callState) {
				installed = state
			},
			send: func(context.Context, waBinary.Node) error {
				provisional = installed
				installed = replacement
				return errors.New("send failed")
			},
			remove: func(_ string, state *callState) {
				if installed == state {
					installed = nil
				}
			},
		},
	)
	if err == nil {
		t.Fatal("runInitialGroupCallStart accepted failed send")
	}
	if provisional == nil {
		t.Fatal("send did not observe provisional call state")
	}
	if installed != replacement {
		t.Fatalf("failed send cleanup retained %p, want replacement %p", installed, replacement)
	}
}

func TestRemoveCallIfSameDeletesOnlyExpectedState(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L178-L180
	cli, _, _ := routerTestClient()
	provisional := &callState{meta: types.BasicCallMeta{CallID: "CID"}}
	replacement := &callState{meta: types.BasicCallMeta{CallID: "replacement"}}
	cli.calls["CID"] = replacement

	cli.removeCallIfSame("CID", provisional)
	if got := cli.getCall("CID"); got != replacement {
		t.Fatalf("mismatched cleanup retained %p, want replacement %p", got, replacement)
	}

	cli.removeCallIfSame("CID", replacement)
	if got := cli.getCall("CID"); got != nil {
		t.Fatalf("matched cleanup retained state %p", got)
	}
}

func TestGroupMediaReadyFieldsUsesFirstConnectedRemoteAndUsableRelay(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L103-L111
	corpus := loadInitialGroupStartCorpus(t)
	self := mustInitialGroupStartJID(t, corpus.Offer.CallCreator)
	update := initialGroupStartUpdateFromVector(t, corpus, corpus.Ready)
	peer, relay, ok := groupMediaReadyFields(self, update)
	if !ok {
		t.Fatal("transaction-21 capture did not produce media readiness fields")
	}
	wantPeer := mustInitialGroupStartJID(t, "242653052539031@lid")
	if peer != wantPeer {
		t.Fatalf("media peer = %s, want connected device %s", peer, wantPeer)
	}
	if relay.RelayID != 0 || relay.TokenID != 0 || relay.AuthTokenID != 0 ||
		relay.RelayName != "zrh1c01" || relay.IPv4 != "157.240.17.133" ||
		relay.Port != 3478 || relay.IsFNA {
		t.Fatalf("selected relay = %+v", relay)
	}
	if len(relay.Key) != 24 || len(relay.Token) != 193 || len(relay.AuthToken) != 70 ||
		relay.Key[0] != 66 || relay.Token[0] != 82 || relay.AuthToken[0] != 98 {
		t.Fatalf("selected relay credentials = key %d/%x token %d/%x auth %d/%x",
			len(relay.Key), relay.Key[0], len(relay.Token), relay.Token[0],
			len(relay.AuthToken), relay.AuthToken[0])
	}
	update.Relay.Key[0] ^= 0xff
	update.Relay.Tokens[0][0] ^= 0xff
	update.Relay.AuthTokens[0][0] ^= 0xff
	if relay.Key[0] != 66 || relay.Token[0] != 82 || relay.AuthToken[0] != 98 {
		t.Fatal("selected relay aliases mutable group snapshot credentials")
	}
}

func TestGroupMediaReadyFieldsRejectsSelfOnlyAndUnusableRelay(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L103-L111
	corpus := loadInitialGroupStartCorpus(t)
	self := mustInitialGroupStartJID(t, corpus.Offer.CallCreator)
	ack := initialGroupStartUpdateFromVector(t, corpus, corpus.Ack)
	if peer, relay, ok := groupMediaReadyFields(self, ack); ok {
		t.Fatalf("transaction-11 self/outgoing roster produced peer %s relay %+v", peer, relay)
	}

	base := initialGroupStartUpdateFromVector(t, corpus, corpus.Ready)
	cases := []struct {
		name string
		edit func(*types.GroupCallUpdate)
	}{
		{name: "missing relay", edit: func(update *types.GroupCallUpdate) { update.Relay = nil }},
		{name: "missing relay key", edit: func(update *types.GroupCallUpdate) { update.Relay.Key = nil }},
		{name: "FNA only", edit: func(update *types.GroupCallUpdate) {
			for i := range update.Relay.Endpoints {
				update.Relay.Endpoints[i].IsFNA = true
			}
		}},
		{name: "IPv6 only", edit: func(update *types.GroupCallUpdate) {
			update.Relay.Endpoints = update.Relay.Endpoints[1:2]
		}},
		{name: "token out of range", edit: func(update *types.GroupCallUpdate) {
			update.Relay.Endpoints[0].TokenID = uint32(len(update.Relay.Tokens))
			update.Relay.Endpoints = update.Relay.Endpoints[:1]
		}},
		{name: "auth token out of range", edit: func(update *types.GroupCallUpdate) {
			update.Relay.Endpoints[0].AuthTokenID = uint32(len(update.Relay.AuthTokens))
			update.Relay.Endpoints = update.Relay.Endpoints[:1]
		}},
		{name: "connected device has no PID", edit: func(update *types.GroupCallUpdate) {
			for i := range update.Participants {
				for j := range update.Participants[i].Devices {
					update.Participants[i].Devices[j].HasPID = false
				}
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			update := cloneInitialGroupStartUpdate(base)
			tc.edit(&update)
			if peer, relay, ok := groupMediaReadyFields(self, update); ok {
				t.Fatalf("invalid snapshot produced peer %s relay %+v", peer, relay)
			}
		})
	}
}

func TestInitialGroupAckThenConnectedUpdateEmitsMediaReadyOnce(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L103-L111
	corpus := loadInitialGroupStartCorpus(t)
	self := mustInitialGroupStartJID(t, corpus.Offer.CallCreator)
	firstSelected := mustInitialGroupStartJID(t, "9908623781998@lid")
	cli, _, captured := routerTestClient()
	cli.Store = &store.Device{LID: self}
	cli.calls[corpus.Offer.CallID] = &callState{
		meta: types.BasicCallMeta{
			From:        self,
			CallCreator: self,
			CallID:      corpus.Offer.CallID,
		},
		selfLID:               self,
		peerLID:               firstSelected,
		to:                    firstSelected,
		creator:               self,
		outgoing:              true,
		callKey:               bytes.Repeat([]byte{0x71}, 32),
		hasGroupKeyEpoch:      true,
		groupKeyTransactionID: 20,
		group: &groupCallState{snapshot: types.GroupCallUpdate{
			CallID:        corpus.Offer.CallID,
			CallCreator:   self,
			TransactionID: 0,
		}},
	}

	ack := initialGroupStartAckNode(t, corpus)
	cli.handleCallAck(context.Background(), &ack)
	cs := cli.getCall(corpus.Offer.CallID)
	if cs.group == nil || cs.group.snapshot.TransactionID != 11 {
		t.Fatalf("ACK snapshot = %+v, want transaction 11", cs.group)
	}
	if n := len(captured.filter(isCallMediaReady)); n != 0 {
		t.Fatalf("transaction-11 readiness count = %d, want 0", n)
	}

	updateNode := initialGroupStartUpdateNode(t, corpus, corpus.Ready)
	meta := types.BasicCallMeta{
		From:        types.NewJID(corpus.Offer.CallID, "call"),
		CallCreator: self,
		CallID:      corpus.Offer.CallID,
	}
	cli.onCallGroupUpdate(context.Background(), &updateNode, meta)
	cli.onCallGroupUpdate(context.Background(), &updateNode, meta)

	ready := captured.filter(isCallMediaReady)
	if len(ready) != 1 {
		t.Fatalf("transaction-21 readiness count = %d, want 1", len(ready))
	}
	event := ready[0].(*events.CallMediaReady)
	if event.PeerLID != mustInitialGroupStartJID(t, "242653052539031@lid") ||
		event.Relay.IPv4 != "157.240.17.133" || event.Relay.Port != 3478 ||
		event.Direction != types.CallDirectionOutgoing {
		t.Fatalf("media readiness event = %+v", event)
	}
	if cs.peerLID != firstSelected || cs.to != firstSelected {
		t.Fatalf("legacy selected peer changed to %s/%s", cs.peerLID, cs.to)
	}
	if cs.group.snapshot.TransactionID != 21 {
		t.Fatalf("final group transaction = %d, want 21", cs.group.snapshot.TransactionID)
	}
}

func TestGroupAcceptPreservesLegacyPeerWhileDirectAcceptUpdatesIt(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L180-L181
	legacyPeer := mustInitialGroupStartJID(t, "200001@lid")
	laterPeer := mustInitialGroupStartJID(t, "200002:9@lid")
	groupClient, _, _ := routerTestClient()
	groupClient.calls["GROUP"] = &callState{
		meta:    types.BasicCallMeta{CallID: "GROUP"},
		peerLID: legacyPeer,
		to:      legacyPeer,
		group: &groupCallState{snapshot: types.GroupCallUpdate{
			CallID: "GROUP",
		}},
	}
	groupClient.onCallAccept(
		types.BasicCallMeta{CallID: "GROUP", From: laterPeer},
		types.CallRemoteMeta{},
		&waBinary.Node{Tag: "accept"},
	)
	groupState := groupClient.getCall("GROUP")
	if groupState.peerLID != legacyPeer || groupState.to != legacyPeer || !groupState.connected {
		t.Fatalf("group accept state = peer %s to %s connected %t",
			groupState.peerLID, groupState.to, groupState.connected)
	}

	directClient, _, _ := routerTestClient()
	directClient.calls["DIRECT"] = &callState{
		meta:    types.BasicCallMeta{CallID: "DIRECT"},
		peerLID: legacyPeer,
		to:      legacyPeer,
	}
	directClient.onCallAccept(
		types.BasicCallMeta{CallID: "DIRECT", From: laterPeer},
		types.CallRemoteMeta{},
		&waBinary.Node{Tag: "accept"},
	)
	directState := directClient.getCall("DIRECT")
	if directState.peerLID != laterPeer || directState.to != laterPeer || !directState.connected {
		t.Fatalf("direct accept state = peer %s to %s connected %t",
			directState.peerLID, directState.to, directState.connected)
	}
}

func TestCallOfferCarriesClonedOptionalGroupSnapshot(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L190-L191
	self := mustInitialGroupStartJID(t, "300003:14@lid")
	creator := mustInitialGroupStartJID(t, "100001:1@lid")
	peer := mustInitialGroupStartJID(t, "200002@lid")
	group := types.GroupCallUpdate{
		CallID:        "ACTIVE-GROUP",
		CallCreator:   creator,
		TransactionID: 7,
		Participants: []types.GroupCallParticipant{{
			JID:   peer,
			State: "connected",
			Devices: []types.GroupCallDevice{{
				JID:               peer,
				PID:               1,
				HasPID:            true,
				CapabilityVersion: 1,
				Capability:        []byte{1, 5, 247, 9, 224, 187, 83},
			}},
		}},
	}
	groupClient, _, groupEvents := routerTestClient()
	groupClient.Store = &store.Device{LID: self}
	meta := types.BasicCallMeta{CallID: group.CallID, From: creator, CallCreator: creator}
	groupClient.acceptInboundOffer(
		context.Background(),
		&waBinary.Node{Tag: "offer"},
		meta,
		types.CallRemoteMeta{},
		nil,
		&group,
	)
	offers := groupEvents.filter(isCallOffer)
	if len(offers) != 1 {
		t.Fatalf("group CallOffer count = %d, want 1", len(offers))
	}
	groupEvent := offers[0].(*events.CallOffer)
	if groupEvent.Group == nil || groupEvent.Group.TransactionID != 7 ||
		groupEvent.Group.Participants[0].Devices[0].PID != 1 {
		t.Fatalf("group CallOffer snapshot = %+v", groupEvent.Group)
	}
	group.Participants[0].Devices[0].Capability[0] ^= 0xff
	group.Participants[0].Devices[0].PID = 9
	if groupEvent.Group.Participants[0].Devices[0].Capability[0] != 1 ||
		groupEvent.Group.Participants[0].Devices[0].PID != 1 {
		t.Fatal("CallOffer group snapshot aliases parser-owned data")
	}

	directClient, _, directEvents := routerTestClient()
	directClient.Store = &store.Device{LID: self}
	directClient.acceptInboundOffer(
		context.Background(),
		&waBinary.Node{Tag: "offer"},
		types.BasicCallMeta{CallID: "DIRECT", From: peer, CallCreator: peer},
		types.CallRemoteMeta{},
		bytes.Repeat([]byte{0x11}, 32),
	)
	directOffers := directEvents.filter(isCallOffer)
	if len(directOffers) != 1 {
		t.Fatalf("direct CallOffer count = %d, want 1", len(directOffers))
	}
	if directOffers[0].(*events.CallOffer).Group != nil {
		t.Fatalf("direct CallOffer group snapshot = %+v, want nil", directOffers[0].(*events.CallOffer).Group)
	}
}

func initialGroupStartUpdateFromVector(
	t *testing.T,
	corpus initialGroupStartCorpus,
	vector initialGroupStartSnapshot,
) types.GroupCallUpdate {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L103-L111
	t.Helper()
	update := types.GroupCallUpdate{
		CallID:         corpus.Offer.CallID,
		CallCreator:    mustInitialGroupStartJID(t, corpus.Offer.CallCreator),
		TransactionID:  vector.TransactionID,
		Media:          vector.Media,
		ConnectedLimit: vector.ConnectedLimit,
		Joinable:       vector.Joinable,
		Relay: &types.GroupCallRelay{
			TransactionID: vector.Relay.TransactionID,
			SelfPID:       vector.Relay.SelfPID,
			HasSelfPID:    true,
			Key:           bytes.Repeat([]byte{vector.Relay.KeyFill}, vector.Relay.KeyLength),
		},
	}
	for _, participantVector := range vector.Participants {
		participant := types.GroupCallParticipant{
			JID:   mustInitialGroupStartJID(t, participantVector.JID),
			State: participantVector.State,
		}
		for _, deviceVector := range participantVector.Devices {
			device := types.GroupCallDevice{JID: mustInitialGroupStartJID(t, deviceVector.JID)}
			if deviceVector.PID != nil {
				device.PID = *deviceVector.PID
				device.HasPID = true
			}
			participant.Devices = append(participant.Devices, device)
		}
		update.Participants = append(update.Participants, participant)
	}
	for i, length := range vector.Relay.TokenLengths {
		update.Relay.Tokens = append(
			update.Relay.Tokens,
			bytes.Repeat([]byte{vector.Relay.TokenFills[i]}, length),
		)
	}
	for i, length := range vector.Relay.AuthTokenLengths {
		update.Relay.AuthTokens = append(
			update.Relay.AuthTokens,
			bytes.Repeat([]byte{vector.Relay.AuthTokenFills[i]}, length),
		)
	}
	for _, endpointVector := range vector.Relay.Endpoints {
		address, err := hex.DecodeString(endpointVector.AddressHex)
		if err != nil {
			t.Fatalf("decode endpoint: %v", err)
		}
		update.Relay.Endpoints = append(update.Relay.Endpoints, types.GroupCallRelayEndpoint{
			RelayID:     endpointVector.RelayID,
			TokenID:     endpointVector.TokenID,
			AuthTokenID: endpointVector.AuthTokenID,
			RelayName:   endpointVector.RelayName,
			RTT:         endpointVector.RTT,
			IsFNA:       endpointVector.IsFNA,
			Address:     address,
		})
	}
	return update
}

func initialGroupStartAckNode(t *testing.T, corpus initialGroupStartCorpus) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L27-L33
	t.Helper()
	update := initialGroupStartUpdateFromVector(t, corpus, corpus.Ack)
	children := []waBinary.Node{
		initialGroupStartGroupInfoNode(update),
		initialGroupStartRelayNode(update.Relay),
	}
	return waBinary.Node{
		Tag:     "ack",
		Attrs:   waBinary.Attrs{"class": "call", "type": "offer"},
		Content: children,
	}
}

func initialGroupStartUpdateNode(
	t *testing.T,
	corpus initialGroupStartCorpus,
	vector initialGroupStartSnapshot,
) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L30-L33
	t.Helper()
	update := initialGroupStartUpdateFromVector(t, corpus, vector)
	return waBinary.Node{
		Tag: "group_update",
		Attrs: waBinary.Attrs{
			"call-id":      update.CallID,
			"call-creator": update.CallCreator,
		},
		Content: []waBinary.Node{
			initialGroupStartGroupInfoNode(update),
			initialGroupStartRelayNode(update.Relay),
		},
	}
}

func initialGroupStartGroupInfoNode(update types.GroupCallUpdate) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L103-L111
	attrs := waBinary.Attrs{
		"call-id":         update.CallID,
		"call-creator":    update.CallCreator,
		"transaction-id":  strconv.FormatUint(uint64(update.TransactionID), 10),
		"media":           update.Media,
		"connected-limit": strconv.FormatUint(uint64(update.ConnectedLimit), 10),
	}
	if update.Joinable {
		attrs["joinable"] = "1"
	}
	users := make([]waBinary.Node, len(update.Participants))
	for i, participant := range update.Participants {
		devices := make([]waBinary.Node, len(participant.Devices))
		for j, device := range participant.Devices {
			deviceAttrs := waBinary.Attrs{"jid": device.JID}
			if device.HasPID {
				deviceAttrs["pid"] = strconv.FormatUint(uint64(device.PID), 10)
			}
			devices[j] = waBinary.Node{Tag: "device", Attrs: deviceAttrs}
		}
		users[i] = waBinary.Node{
			Tag:     "user",
			Attrs:   waBinary.Attrs{"jid": participant.JID, "state": participant.State},
			Content: devices,
		}
	}
	return waBinary.Node{Tag: "group_info", Attrs: attrs, Content: users}
}

func initialGroupStartRelayNode(relay *types.GroupCallRelay) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L103-L111
	children := []waBinary.Node{{Tag: "key", Content: bytes.Clone(relay.Key)}}
	for i, token := range relay.Tokens {
		children = append(children, waBinary.Node{
			Tag:     "token",
			Attrs:   waBinary.Attrs{"id": strconv.Itoa(i)},
			Content: bytes.Clone(token),
		})
	}
	for i, token := range relay.AuthTokens {
		children = append(children, waBinary.Node{
			Tag:     "auth_token",
			Attrs:   waBinary.Attrs{"id": strconv.Itoa(i)},
			Content: bytes.Clone(token),
		})
	}
	for _, endpoint := range relay.Endpoints {
		attrs := waBinary.Attrs{
			"relay_id":      strconv.FormatUint(uint64(endpoint.RelayID), 10),
			"token_id":      strconv.FormatUint(uint64(endpoint.TokenID), 10),
			"auth_token_id": strconv.FormatUint(uint64(endpoint.AuthTokenID), 10),
			"relay_name":    endpoint.RelayName,
		}
		if endpoint.RTT != 0 {
			attrs["c2r_rtt"] = strconv.FormatUint(uint64(endpoint.RTT), 10)
		}
		if endpoint.IsFNA {
			attrs["is_fna"] = "1"
		}
		children = append(children, waBinary.Node{
			Tag:     "te2",
			Attrs:   attrs,
			Content: bytes.Clone(endpoint.Address),
		})
	}
	return waBinary.Node{
		Tag: "relay",
		Attrs: waBinary.Attrs{
			"transaction-id":   strconv.FormatUint(uint64(relay.TransactionID), 10),
			"self_pid":         strconv.FormatUint(uint64(relay.SelfPID), 10),
			"uuid":             "capture-relay",
			"participant_uuid": "capture-self",
		},
		Content: children,
	}
}

func cloneInitialGroupStartUpdate(update types.GroupCallUpdate) types.GroupCallUpdate {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L177-L188
	cloned := update
	cloned.Participants = make([]types.GroupCallParticipant, len(update.Participants))
	for i, participant := range update.Participants {
		cloned.Participants[i] = participant
		cloned.Participants[i].Devices = append([]types.GroupCallDevice(nil), participant.Devices...)
	}
	if update.Relay != nil {
		relay := *update.Relay
		relay.Key = bytes.Clone(update.Relay.Key)
		relay.Tokens = make([][]byte, len(update.Relay.Tokens))
		for i := range update.Relay.Tokens {
			relay.Tokens[i] = bytes.Clone(update.Relay.Tokens[i])
		}
		relay.AuthTokens = make([][]byte, len(update.Relay.AuthTokens))
		for i := range update.Relay.AuthTokens {
			relay.AuthTokens[i] = bytes.Clone(update.Relay.AuthTokens[i])
		}
		relay.Endpoints = append([]types.GroupCallRelayEndpoint(nil), update.Relay.Endpoints...)
		for i := range relay.Endpoints {
			relay.Endpoints[i].Address = bytes.Clone(update.Relay.Endpoints[i].Address)
		}
		cloned.Relay = &relay
	}
	return cloned
}

func equalInitialGroupJIDs(a, b []types.JID) bool {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L164-L176
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mustInitialGroupStartJID(t *testing.T, raw string) types.JID {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L47-L111
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
