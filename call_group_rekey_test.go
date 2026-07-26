// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/voip"
)

func TestDecodeCallEncRekeyPlaintextUnwrapsCallKeyMessage(t *testing.T) {
	rawKey := bytes.Repeat([]byte{0x52}, 32)
	plaintext, err := proto.Marshal(&waE2E.Message{
		Call: &waE2E.Call{CallKey: rawKey},
	})
	if err != nil {
		t.Fatalf("marshal call key message: %v", err)
	}
	plaintext = append(plaintext, 0xfa, 0x07, 0x28)
	plaintext = append(plaintext, bytes.Repeat([]byte{0x78}, 40)...)
	if len(plaintext) != 79 {
		t.Fatalf("plaintext length = %d, want live-observed 79", len(plaintext))
	}

	got, err := decodeCallEncRekeyPlaintext(plaintext)
	if err != nil {
		t.Fatalf("decodeCallEncRekeyPlaintext: %v", err)
	}
	if !bytes.Equal(got, rawKey) {
		t.Fatal("decoder did not extract the 32-byte Call.callKey")
	}
}

func TestNewCallEncRekeyEventClonesSensitiveBytes(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L75-L123
	author := mustParseCallEncRekeyJID(t, "111111111111111@lid")
	creator := mustParseCallEncRekeyJID(t, "222222222222222:14@lid")
	ciphertext := bytes.Repeat([]byte{0x41}, 146)
	rawKey := bytes.Repeat([]byte{0x52}, 32)
	rekey := &types.GroupCallEncRekey{
		CallID: "SYNTHETIC-CALL-17", CallCreator: creator, TransactionID: 17,
		KeyGeneration: 2, EncryptionType: "msg", EncryptionVersion: 2,
		Ciphertext: ciphertext,
	}
	child := &waBinary.Node{Tag: "enc_rekey"}
	event, err := newCallEncRekeyEvent(
		types.BasicCallMeta{From: author, CallCreator: creator, CallID: rekey.CallID},
		rekey,
		rawKey,
		child,
	)
	if err != nil {
		t.Fatalf("newCallEncRekeyEvent: %v", err)
	}
	if event.From != author || event.CallCreator != creator || event.Rekey.TransactionID != 17 || event.Data != child {
		t.Fatalf("event metadata = %+v", event)
	}
	ciphertext[0] ^= 0xff
	rawKey[0] ^= 0xff
	if event.Rekey.Ciphertext[0] == ciphertext[0] {
		t.Fatal("event ciphertext aliases parsed envelope")
	}
	if event.RawKey[0] == rawKey[0] {
		t.Fatal("event raw key aliases decrypt buffer")
	}
}

func TestNewCallEncRekeyEventRequiresExactRawKeyLength(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L53-L65
	rekey := &types.GroupCallEncRekey{}
	for _, length := range []int{0, 31, 33} {
		t.Run(strconv.Itoa(length), func(t *testing.T) {
			// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L53-L65
			if _, err := newCallEncRekeyEvent(types.BasicCallMeta{}, rekey, make([]byte, length), &waBinary.Node{}); err == nil {
				t.Fatalf("newCallEncRekeyEvent accepted %d-byte raw key", length)
			}
		})
	}
}

func TestInstallGroupKeyEpochStartsKeylessAddedParticipantMedia(t *testing.T) {
	cli, _, captured := routerTestClient()
	self := mustParseCallEncRekeyJID(t, "300003:14@lid")
	peer := mustParseCallEncRekeyJID(t, "100001:1@lid")
	meta := types.BasicCallMeta{CallID: "ACTIVE-AD-HOC-CALL", From: peer, CallCreator: peer}
	cli.calls[meta.CallID] = &callState{
		meta:    meta,
		selfLID: self,
		peerLID: peer,
		creator: peer,
		group: &groupCallState{snapshot: types.GroupCallUpdate{
			CallID: meta.CallID, TransactionID: 14,
			Participants: []types.GroupCallParticipant{
				{
					JID: self.ToNonAD(), State: "connected",
					Devices: []types.GroupCallDevice{{JID: self, PID: 0, HasPID: true}},
				},
				{
					JID: peer.ToNonAD(), State: "connected",
					Devices: []types.GroupCallDevice{{JID: peer, PID: 1, HasPID: true}},
				},
			},
			Relay: &types.GroupCallRelay{
				Key:        bytes.Repeat([]byte{0x41}, 24),
				Tokens:     [][]byte{bytes.Repeat([]byte{0x42}, 16)},
				AuthTokens: [][]byte{bytes.Repeat([]byte{0x43}, 16)},
				Endpoints: []types.GroupCallRelayEndpoint{{
					RelayName: "euc1",
					Address:   []byte{157, 240, 17, 133, 0x0d, 0x96},
				}},
			},
		}},
	}
	rawKey := bytes.Repeat([]byte{0x71}, 32)
	rekey := types.GroupCallEncRekey{
		CallID: meta.CallID, CallCreator: peer, TransactionID: 14,
		KeyGeneration: 2, EncryptionType: "msg", EncryptionVersion: 2,
	}
	if err := cli.installGroupKeyEpoch(meta, rekey, rawKey, &waBinary.Node{Tag: "enc_rekey"}, false); err != nil {
		t.Fatalf("installGroupKeyEpoch: %v", err)
	}
	rawKey[0] ^= 0xff

	cs := cli.getCall(meta.CallID)
	if cs == nil {
		t.Fatal("call state missing after install")
	}
	if len(cs.callKey) != 32 || cs.callKey[0] != 0x71 {
		t.Fatalf("installed call key = %x", cs.callKey)
	}
	if !cs.mediaReadySent || !cs.hasGroupKeyEpoch || cs.groupKeyTransactionID != 14 {
		t.Fatalf("installed state = ready %t has_epoch %t tx %d", cs.mediaReadySent, cs.hasGroupKeyEpoch, cs.groupKeyTransactionID)
	}
	rekeys := captured.filter(isCallEncRekey)
	if len(rekeys) != 1 {
		t.Fatalf("CallEncRekey count = %d, want 1", len(rekeys))
	}
	event := rekeys[0].(*events.CallEncRekey)
	if event.Local || event.From != peer || event.RawKey[0] != 0x71 {
		t.Fatalf("inbound epoch event = %+v", event)
	}
	ready := captured.filter(isCallMediaReady)
	if len(ready) != 1 {
		t.Fatalf("CallMediaReady count = %d, want 1", len(ready))
	}
	if got := ready[0].(*events.CallMediaReady); got.CallKey[0] != 0x71 || got.SelfLID != self {
		t.Fatalf("media ready = %+v", got)
	}
}

func TestInstallGroupKeyEpochDeduplicatesAndDoesNotRestartActiveMedia(t *testing.T) {
	cli, _, captured := routerTestClient()
	self := mustParseCallEncRekeyJID(t, "100001:14@lid")
	distributor := mustParseCallEncRekeyJID(t, "300003:43@lid")
	meta := types.BasicCallMeta{CallID: "ACTIVE-CALL", From: distributor, CallCreator: self}
	cli.calls[meta.CallID] = &callState{
		meta:           meta,
		selfLID:        self,
		peerLID:        distributor,
		creator:        self,
		callKey:        bytes.Repeat([]byte{0x11}, 32),
		relay:          &types.RelayEndpoint{RelayName: "euc1"},
		mediaReadySent: true,
		group: &groupCallState{snapshot: types.GroupCallUpdate{
			CallID: meta.CallID, TransactionID: 18,
		}},
	}
	rawKey := bytes.Repeat([]byte{0x72}, 32)
	rekey := types.GroupCallEncRekey{
		CallID: meta.CallID, CallCreator: self, TransactionID: 17,
		KeyGeneration: 2, EncryptionType: "msg", EncryptionVersion: 2,
	}
	if err := cli.installGroupKeyEpoch(meta, rekey, rawKey, nil, false); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if err := cli.installGroupKeyEpoch(meta, rekey, rawKey, nil, false); err != nil {
		t.Fatalf("identical duplicate: %v", err)
	}
	conflicting := bytes.Repeat([]byte{0x27}, 32)
	if err := cli.installGroupKeyEpoch(meta, rekey, conflicting, nil, false); err == nil {
		t.Fatal("conflicting duplicate was accepted")
	}
	stale := rekey
	stale.TransactionID = 16
	if err := cli.installGroupKeyEpoch(meta, stale, bytes.Repeat([]byte{0x33}, 32), nil, false); err != nil {
		t.Fatalf("stale epoch: %v", err)
	}
	if n := len(captured.filter(isCallEncRekey)); n != 1 {
		t.Fatalf("CallEncRekey count = %d, want 1", n)
	}
	if n := len(captured.filter(isCallMediaReady)); n != 0 {
		t.Fatalf("active media restarted %d times", n)
	}
	cs := cli.getCall(meta.CallID)
	if cs.callKey[0] != 0x72 || cs.groupKeyTransactionID != 17 {
		t.Fatalf("final epoch = key %x tx %d", cs.callKey[0], cs.groupKeyTransactionID)
	}
}

func TestGroupRekeyRecipientsSelectsConnectedPIDDevicesInRosterOrder(t *testing.T) {
	self := mustParseCallEncRekeyJID(t, "100001:14@lid")
	peer := mustParseCallEncRekeyJID(t, "200002@lid")
	added := mustParseCallEncRekeyJID(t, "300003:43@lid")
	receipt := mustParseCallEncRekeyJID(t, "400004:63@lid")
	noPID := mustParseCallEncRekeyJID(t, "500005:9@lid")
	update := types.GroupCallUpdate{
		CallID: "CID", TransactionID: 14,
		Participants: []types.GroupCallParticipant{
			{
				JID: self.ToNonAD(), State: "connected",
				Devices: []types.GroupCallDevice{{JID: self, PID: 1, HasPID: true}},
			},
			{
				JID: peer.ToNonAD(), State: "connected",
				Devices: []types.GroupCallDevice{{JID: peer, PID: 0, HasPID: true}},
			},
			{
				JID: added.ToNonAD(), State: "connected",
				Devices: []types.GroupCallDevice{
					{JID: added, PID: 2, HasPID: true},
					{JID: added, PID: 2, HasPID: true},
					{JID: noPID},
				},
			},
			{
				JID: receipt.ToNonAD(), State: "receipt",
				Devices: []types.GroupCallDevice{{JID: receipt, PID: 3, HasPID: true}},
			},
		},
	}
	got := groupRekeyRecipients(update, self)
	want := []types.JID{peer, added}
	if len(got) != len(want) {
		t.Fatalf("recipients = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("recipient %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestRunRequestedGroupEpochFanoutUsesOneRootAndInstallsAfterAllSends(t *testing.T) {
	self := mustParseCallEncRekeyJID(t, "100001:14@lid")
	creator := self
	peer := mustParseCallEncRekeyJID(t, "200002@lid")
	added := mustParseCallEncRekeyJID(t, "300003:43@lid")
	meta := types.BasicCallMeta{CallID: "CID", CallCreator: creator}
	update := types.GroupCallUpdate{
		CallID: "CID", CallCreator: creator, TransactionID: 14, RekeyRequested: true,
		Participants: []types.GroupCallParticipant{
			{JID: self.ToNonAD(), State: "connected", Devices: []types.GroupCallDevice{{JID: self, PID: 1, HasPID: true}}},
			{JID: peer.ToNonAD(), State: "connected", Devices: []types.GroupCallDevice{{JID: peer, PID: 0, HasPID: true}}},
			{JID: added.ToNonAD(), State: "connected", Devices: []types.GroupCallDevice{{JID: added, PID: 2, HasPID: true}}},
		},
	}
	root := bytes.Repeat([]byte{0x84}, 32)
	var encryptedRoot []byte
	var sent []waBinary.Node
	var installedRoot []byte
	var installedAfter int
	requestIDs := []string{"REQ-A", "REQ-B"}
	nextID := 0
	deps := groupEpochFanoutDependencies{
		random: bytes.NewReader(root),
		encrypt: func(_ context.Context, recipients []types.JID, rawKey []byte) ([]voip.DeviceKey, error) {
			if len(recipients) != 2 || recipients[0] != peer || recipients[1] != added {
				t.Fatalf("encrypt recipients = %v", recipients)
			}
			encryptedRoot = bytes.Clone(rawKey)
			return []voip.DeviceKey{
				{DeviceJID: peer, Ciphertext: []byte{0x21}, EncType: "msg"},
				{DeviceJID: added, Ciphertext: []byte{0x22}, EncType: "msg"},
			}, nil
		},
		requestID: func() string {
			id := requestIDs[nextID]
			nextID++
			return id
		},
		send: func(_ context.Context, node waBinary.Node) error {
			if installedRoot != nil {
				t.Fatal("local epoch installed before all sends")
			}
			sent = append(sent, node)
			return nil
		},
		install: func(gotMeta types.BasicCallMeta, rekey types.GroupCallEncRekey, rawKey []byte, _ *waBinary.Node, local bool) error {
			if gotMeta.From != self || rekey.TransactionID != 14 || !local {
				t.Fatalf("local install metadata = %+v/%+v local=%t", gotMeta, rekey, local)
			}
			installedRoot = bytes.Clone(rawKey)
			installedAfter = len(sent)
			return nil
		},
	}
	if err := runRequestedGroupEpochFanout(context.Background(), meta, update, self, deps); err != nil {
		t.Fatalf("runRequestedGroupEpochFanout: %v", err)
	}
	if !bytes.Equal(encryptedRoot, root) || !bytes.Equal(installedRoot, root) {
		t.Fatal("encryption and local install did not receive the same root")
	}
	if len(sent) != 2 || installedAfter != 2 {
		t.Fatalf("sent/install order = %d/%d", len(sent), installedAfter)
	}
	for i, node := range sent {
		if node.AttrGetter().JID("to") != []types.JID{peer, added}[i] ||
			node.AttrGetter().String("id") != requestIDs[i] {
			t.Fatalf("stanza %d outer attrs = %+v", i, node.Attrs)
		}
		action := node.GetChildren()[0]
		if action.Tag != "enc_rekey" || action.AttrGetter().String("transaction-id") != "14" {
			t.Fatalf("stanza %d action = %+v", i, action)
		}
	}
}

func TestRunRequestedGroupEpochFanoutInstallsAfterPartialSendFailure(t *testing.T) {
	self := mustParseCallEncRekeyJID(t, "100001:14@lid")
	peer := mustParseCallEncRekeyJID(t, "200002@lid")
	added := mustParseCallEncRekeyJID(t, "300003:43@lid")
	update := types.GroupCallUpdate{
		CallID: "CID", CallCreator: self, TransactionID: 14, RekeyRequested: true,
		Participants: []types.GroupCallParticipant{
			{JID: self.ToNonAD(), State: "connected", Devices: []types.GroupCallDevice{{JID: self, PID: 1, HasPID: true}}},
			{JID: peer.ToNonAD(), State: "connected", Devices: []types.GroupCallDevice{{JID: peer, PID: 0, HasPID: true}}},
			{JID: added.ToNonAD(), State: "connected", Devices: []types.GroupCallDevice{{JID: added, PID: 2, HasPID: true}}},
		},
	}
	sentinel := errors.New("send failed")
	sends := 0
	installed := false
	deps := groupEpochFanoutDependencies{
		random: bytes.NewReader(bytes.Repeat([]byte{0x85}, 32)),
		encrypt: func(_ context.Context, recipients []types.JID, _ []byte) ([]voip.DeviceKey, error) {
			return []voip.DeviceKey{
				{DeviceJID: recipients[0], Ciphertext: []byte{1}, EncType: "msg"},
				{DeviceJID: recipients[1], Ciphertext: []byte{2}, EncType: "msg"},
			}, nil
		},
		requestID: func() string { return "REQ" },
		send: func(context.Context, waBinary.Node) error {
			sends++
			if sends == 1 {
				return sentinel
			}
			return nil
		},
		install: func(types.BasicCallMeta, types.GroupCallEncRekey, []byte, *waBinary.Node, bool) error {
			installed = true
			return nil
		},
	}
	err := runRequestedGroupEpochFanout(
		context.Background(),
		types.BasicCallMeta{CallID: "CID", CallCreator: self},
		update,
		self,
		deps,
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("fanout error = %v, want %v", err, sentinel)
	}
	if sends != 2 {
		t.Fatalf("send attempts = %d, want 2", sends)
	}
	if !installed {
		t.Fatal("partial fanout did not install the local epoch")
	}
}

func TestRunRequestedGroupEpochFanoutPreservesSendAndInstallFailures(t *testing.T) {
	self := mustParseCallEncRekeyJID(t, "100001:14@lid")
	peer := mustParseCallEncRekeyJID(t, "200002@lid")
	update := types.GroupCallUpdate{
		CallID: "CID", CallCreator: self, TransactionID: 14, RekeyRequested: true,
		Participants: []types.GroupCallParticipant{
			{JID: self.ToNonAD(), State: "connected", Devices: []types.GroupCallDevice{{JID: self, PID: 1, HasPID: true}}},
			{JID: peer.ToNonAD(), State: "connected", Devices: []types.GroupCallDevice{{JID: peer, PID: 0, HasPID: true}}},
		},
	}
	sendErr := errors.New("send failed")
	installErr := errors.New("install failed")
	deps := groupEpochFanoutDependencies{
		random: bytes.NewReader(bytes.Repeat([]byte{0x86}, 32)),
		encrypt: func(_ context.Context, recipients []types.JID, _ []byte) ([]voip.DeviceKey, error) {
			return []voip.DeviceKey{{
				DeviceJID: recipients[0], Ciphertext: []byte{1}, EncType: "msg",
			}}, nil
		},
		requestID: func() string { return "REQ" },
		send: func(context.Context, waBinary.Node) error {
			return sendErr
		},
		install: func(types.BasicCallMeta, types.GroupCallEncRekey, []byte, *waBinary.Node, bool) error {
			return installErr
		},
	}
	err := runRequestedGroupEpochFanout(
		context.Background(),
		types.BasicCallMeta{CallID: "CID", CallCreator: self},
		update,
		self,
		deps,
	)
	if !errors.Is(err, sendErr) {
		t.Fatalf("fanout error = %v, missing %v", err, sendErr)
	}
	if !errors.Is(err, installErr) {
		t.Fatalf("fanout error = %v, missing %v", err, installErr)
	}
}

func TestCallEncRekeyMalformedDispatchesUnknownAndStillAcks(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L125-L140
	cli, log, captured := routerTestClient()
	cli.SynchronousAck = true
	node := testCallEncRekeyNode(t, "msg")
	child, _ := node.GetOptionalChildByTag("enc_rekey")
	children := child.Content.([]waBinary.Node)
	children[0].Attrs["keygen"] = "1"
	child.Content = children
	node.Content = []waBinary.Node{child}

	cli.handleCallEvent(t.Context(), &node)

	if n := len(captured.filter(isCallEncRekey)); n != 0 {
		t.Fatalf("CallEncRekey dispatch count = %d, want 0", n)
	}
	unknown := captured.filter(isUnknownCallEvent)
	if len(unknown) != 1 {
		t.Fatalf("UnknownCallEvent dispatch count = %d, want 1", len(unknown))
	}
	if event := unknown[0].(*events.UnknownCallEvent); event.Node == nil || event.Node.Tag != "enc_rekey" {
		t.Fatalf("UnknownCallEvent node = %+v, want enc_rekey child", event.Node)
	}
	if !log.hasWarn("Failed to parse group call rekey") {
		t.Fatal("missing malformed-rekey warning")
	}
	if n := log.warnCount("Failed to send acknowledgement for call"); n != 1 {
		t.Fatalf("deferred ACK attempts = %d, want 1", n)
	}
}

func TestCallEncRekeyDecryptFailureDispatchesUnknownAndStillAcks(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L67-L73
	cli, log, captured := routerTestClient()
	cli.SynchronousAck = true
	node := testCallEncRekeyNode(t, "msg")

	cli.handleCallEvent(t.Context(), &node)

	if n := len(captured.filter(isCallEncRekey)); n != 0 {
		t.Fatalf("CallEncRekey dispatch count = %d, want 0", n)
	}
	if n := len(captured.filter(isUnknownCallEvent)); n != 1 {
		t.Fatalf("UnknownCallEvent dispatch count = %d, want 1", n)
	}
	if !log.hasWarn("Failed to decrypt group call rekey") {
		t.Fatal("missing decrypt-failure warning")
	}
	if n := log.warnCount("Failed to send acknowledgement for call"); n != 1 {
		t.Fatalf("deferred ACK attempts = %d, want 1", n)
	}
}

func testCallEncRekeyNode(t *testing.T, encType string) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L39-L65
	t.Helper()
	author := mustParseCallEncRekeyJID(t, "111111111111111@lid")
	creator := mustParseCallEncRekeyJID(t, "222222222222222:14@lid")
	return waBinary.Node{
		Tag: "call",
		Attrs: waBinary.Attrs{
			"from": author,
			"id":   "synthetic-rekey-stanza",
			"t":    "1721730000",
		},
		Content: []waBinary.Node{{
			Tag: "enc_rekey",
			Attrs: waBinary.Attrs{
				"call-id":        "SYNTHETIC-CALL-17",
				"call-creator":   creator,
				"transaction-id": "17",
			},
			Content: []waBinary.Node{
				{Tag: "encopt", Attrs: waBinary.Attrs{"keygen": "2"}},
				{
					Tag:     "enc",
					Attrs:   waBinary.Attrs{"type": encType, "v": "2"},
					Content: bytes.Repeat([]byte{0x63}, 146),
				},
			},
		}},
	}
}

func mustParseCallEncRekeyJID(t *testing.T, raw string) types.JID {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L42-L50
	t.Helper()
	jid, err := types.ParseJID(raw)
	if err != nil {
		t.Fatalf("parse JID %q: %v", raw, err)
	}
	return jid
}

func isCallEncRekey(event any) bool {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L97-L105
	_, ok := event.(*events.CallEncRekey)
	return ok
}

func TestCallEncRekeyLogsDoNotContainSensitiveBytes(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L125-L135
	cli, log, _ := routerTestClient()
	cli.SynchronousAck = true
	node := testCallEncRekeyNode(t, "msg")
	cli.handleCallEvent(t.Context(), &node)
	for _, warning := range log.warns {
		if strings.Contains(warning, "63636363") {
			t.Fatalf("warning contains ciphertext bytes: %q", warning)
		}
	}
}
