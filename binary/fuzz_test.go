package binary_test

import (
	"testing"

	"go.mau.fi/whatsmeow/binary"
)

// FuzzUnmarshal asserts the decoder never panics on arbitrary input: a malformed
// or malicious frame from the server must always produce an error, never a crash.
func FuzzUnmarshal(f *testing.F) {
	seeds := [][]byte{
		{248, 1, 0},
		{248, 1, 255, 128},
		{248, 2, 252, 1, 97, 250, 248, 0, 252, 1, 115},
		{248, 2, 252, 1, 97, 247, 1, 1, 248, 0},
		{248, 2, 252, 1, 97, 246, 248, 0, 0, 0, 252, 4, 109, 115, 103, 114},
		{0},
		{},
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = binary.Unmarshal(data)
	})
}

// FuzzUnpack asserts Unpack never panics on arbitrary input.
func FuzzUnpack(f *testing.F) {
	for _, s := range [][]byte{{}, {0}, {1}, {2, 1, 2, 3}, {3, 4, 5}} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = binary.Unpack(data)
	})
}
