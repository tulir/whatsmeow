// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

func TestCallKeyPlaintext(t *testing.T) {
	callKey := make([]byte, 32)
	for i := range callKey {
		callKey[i] = byte(i + 1)
	}
	pt, err := callKeyPlaintext(callKey)
	if err != nil {
		t.Fatalf("callKeyPlaintext: %v", err)
	}
	var msg waE2E.Message
	if err := proto.Unmarshal(pt, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := msg.GetCall().GetCallKey()
	if !bytes.Equal(got, callKey) {
		t.Errorf("GetCall().GetCallKey() = %x, want %x", got, callKey)
	}
}

const decryptCallKeyErrPrefix = "whatsmeow: decrypt call key:"

func TestDecryptIncomingCallKeyNilOffer(t *testing.T) {
	cli := &Client{}
	_, err := cli.decryptIncomingCallKey(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "no data node") {
		t.Fatalf("decryptIncomingCallKey(nil) error = %v, want \"no data node\"", err)
	}
}

func TestDecryptIncomingCallKeyNoDataNode(t *testing.T) {
	cli := &Client{}
	offer := &events.CallOffer{}
	_, err := cli.decryptIncomingCallKey(context.Background(), offer)
	if err == nil || !strings.Contains(err.Error(), "no data node") {
		t.Fatalf("decryptIncomingCallKey(no Data) error = %v, want \"no data node\"", err)
	}
}

func TestDecryptIncomingCallKeyNoEncChild(t *testing.T) {
	cli := &Client{}
	offer := &events.CallOffer{
		Data: &waBinary.Node{Tag: "offer", Content: []waBinary.Node{{Tag: "audio"}, {Tag: "video"}}},
	}
	_, err := cli.decryptIncomingCallKey(context.Background(), offer)
	if err == nil || !strings.Contains(err.Error(), "no enc node") {
		t.Fatalf("decryptIncomingCallKey(no enc child) error = %v, want \"no enc node\"", err)
	}
}

func TestDecryptIncomingCallKeyFindsEncAmongDecoys(t *testing.T) {
	cli := &Client{}
	offer := &events.CallOffer{
		Data: &waBinary.Node{
			Tag: "offer",
			Content: []waBinary.Node{
				{Tag: "audio"},
				{Tag: "enc", Attrs: waBinary.Attrs{"type": "msg"}, Content: []byte("not-a-real-signal-message")},
				{Tag: "video"},
			},
		},
	}
	_, err := cli.decryptIncomingCallKey(context.Background(), offer)
	if err == nil {
		t.Fatal("decryptIncomingCallKey(enc among decoys) error = nil, want a decrypt-attempt error")
	}
	if strings.Contains(err.Error(), "no data node") || strings.Contains(err.Error(), "no enc node") {
		t.Fatalf("decryptIncomingCallKey(enc among decoys) error = %v, want extraction to succeed (no sentinel error)", err)
	}
	if !strings.HasPrefix(err.Error(), decryptCallKeyErrPrefix) {
		t.Errorf("decryptIncomingCallKey(enc among decoys) error = %v, want prefix %q (reached decrypt attempt)", err, decryptCallKeyErrPrefix)
	}
}

func TestDecryptIncomingCallKeyDetectsPreKeyType(t *testing.T) {
	cli := &Client{}
	offer := &events.CallOffer{
		Data: &waBinary.Node{
			Tag: "offer",
			Content: []waBinary.Node{
				{Tag: "enc", Attrs: waBinary.Attrs{"type": "pkmsg"}, Content: []byte("not-a-real-signal-message")},
			},
		},
	}
	_, err := cli.decryptIncomingCallKey(context.Background(), offer)
	if err == nil || !strings.HasPrefix(err.Error(), decryptCallKeyErrPrefix) {
		t.Fatalf("decryptIncomingCallKey(pkmsg) error = %v, want prefix %q", err, decryptCallKeyErrPrefix)
	}
	if !strings.Contains(err.Error(), "prekey message") {
		t.Errorf("decryptIncomingCallKey(pkmsg) error = %v, want it to have taken the pre-key parse path", err)
	}
}
