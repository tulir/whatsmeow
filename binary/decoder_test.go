package binary_test

import (
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
