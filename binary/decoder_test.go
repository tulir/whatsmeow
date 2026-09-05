// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package binary_test

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

// TestMarshalUnmarshalRoundTrip verifies that the hardened decoder still decodes
// valid nodes correctly: a node with attributes (including a JID, which exercises
// the JIDPair reader) and nested content round-trips back to an equal value.
func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	original := binary.Node{
		Tag: "iq",
		Attrs: binary.Attrs{
			"id":   "abc",
			"type": "result",
			"from": types.NewJID("12345", types.DefaultUserServer),
		},
		Content: []binary.Node{
			{Tag: "body", Content: []byte("hi")},
		},
	}
	marshaled, err := binary.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	unpacked, err := binary.Unpack(marshaled)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}
	decoded, err := binary.Unmarshal(unpacked)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(*decoded, original) {
		t.Errorf("round trip mismatch:\n original = %+v\n decoded  = %+v", original, *decoded)
	}
}

// TestUnmarshalMalformedInputDoesNotPanic feeds the decoder binary nodes that are
// well-formed enough to start decoding but contain a token of the wrong type where
// a string is required (e.g. a list token in the node tag or JID user position).
//
// A malformed or malicious frame from the server must never crash the client, so
// Unmarshal must return an error for all of these rather than panicking.
func TestUnmarshalMalformedInputDoesNotPanic(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		// List8 node, size 1, tag token = ListEmpty -> read returns nil -> tag.(string)
		{"node tag is nil", []byte{248, 1, 0}},
		// List8 node, size 1, tag token = Nibble8 packed string with high bit set but length 0
		{"empty packed-string tag", []byte{248, 1, 255, 128}},
		// node "a" whose content is a JIDPair whose user sub-token is an empty list
		{"jidpair user is a list", []byte{248, 2, 252, 1, 97, 250, 248, 0, 252, 1, 115}},
		// node "a" whose content is a JIDPair whose user is empty and server is a list
		{"jidpair server is a list", []byte{248, 2, 252, 1, 97, 250, 0, 248, 0}},
		// node "a" whose content is an ADJID whose user sub-token is an empty list
		{"adjid user is a list", []byte{248, 2, 252, 1, 97, 247, 1, 1, 248, 0}},
		// node "a" whose content is an FBJID (server "msgr") whose user is an empty list
		{"fbjid user is a list", []byte{248, 2, 252, 1, 97, 246, 248, 0, 0, 0, 252, 4, 109, 115, 103, 114}},
		// node "a" whose content is an InteropJID (server "interop") whose user is an empty list
		{"interopjid user is a list", []byte{248, 2, 252, 1, 97, 245, 248, 0, 0, 0, 0, 0, 252, 7, 105, 110, 116, 101, 114, 111, 112}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Unmarshal panicked on malformed input: %v", r)
				}
			}()
			if _, err := binary.Unmarshal(tc.input); err == nil {
				t.Error("expected an error for malformed input, got nil")
			}
		})
	}
}

// TestUnpackEmptyInputDoesNotPanic ensures Unpack returns an error instead of
// panicking when given an empty payload (it reads the first byte as a flags byte).
func TestUnpackEmptyInputDoesNotPanic(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Unpack panicked on empty input: %v", r)
				}
			}()
			if _, err := binary.Unpack(tc.input); err == nil {
				t.Error("expected an error for empty input, got nil")
			}
		})
	}
}

// TestUnmarshalTruncatedInputDoesNotPanic feeds the decoder frames that are cut
// off partway through a token, which is what a short or corrupted read from the
// socket looks like. Every read must hit the end-of-stream check and return an
// error instead of indexing past the buffer.
func TestUnmarshalTruncatedInputDoesNotPanic(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"list8 without size", []byte{248}},
		{"node without content", []byte{248, 2, 252, 1, 97}},
		{"node tag shorter than declared length", []byte{248, 2, 252, 3, 97}},
		{"packed string tag shorter than declared length", []byte{248, 2, 255, 3, 0}},
		{"jidpair without server", []byte{248, 2, 252, 1, 97, 250, 252, 1, 98}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Unmarshal panicked on truncated input: %v", r)
				}
			}()
			if _, err := binary.Unmarshal(tc.input); err == nil {
				t.Error("expected an error for truncated input, got nil")
			}
		})
	}
}

// TestUnmarshalDeeplyNestedInputDoesNotOverflowStack feeds the decoder frames
// that nest a value inside itself for the whole length of the frame. Both stay
// well under socket.FrameMaxSize, but before the depth limit they drove the
// decoder millions of levels deep and killed the process with a stack overflow,
// which is a fatal error that the recover in the read loop cannot catch.
func TestUnmarshalDeeplyNestedInputDoesNotOverflowStack(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		// List8 node of size 2, tag token 1, content = List8 of size 1 -> next node
		{"nested nodes", bytes.Repeat([]byte{248, 2, 1, 248, 1}, 2_500_000)},
		// List8 node of size 2, tag token 1, content = JIDPair whose user is
		// another JIDPair, and so on
		{"nested jidpairs", append([]byte{248, 2, 1}, bytes.Repeat([]byte{250}, 2_500_000)...)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Unmarshal panicked on deeply nested input: %v", r)
				}
			}()
			_, err := binary.Unmarshal(tc.input)
			if !errors.Is(err, binary.ErrNodeTooDeep) {
				t.Errorf("expected ErrNodeTooDeep, got %v", err)
			}
		})
	}
}

// TestUnmarshalNestingWithinLimit ensures the depth limit doesn't reject nodes
// that are nested far deeper than any real stanza but still within the cap.
func TestUnmarshalNestingWithinLimit(t *testing.T) {
	node := binary.Node{Tag: "leaf", Content: []byte("hi")}
	for i := 0; i < 200; i++ {
		node = binary.Node{Tag: "iq", Content: []binary.Node{node}}
	}
	marshaled, err := binary.Marshal(node)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	unpacked, err := binary.Unpack(marshaled)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}
	decoded, err := binary.Unmarshal(unpacked)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(*decoded, node) {
		t.Error("deeply nested node did not round trip")
	}
}
