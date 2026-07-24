// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
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
