// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"bytes"
	"testing"

	waLog "go.mau.fi/whatsmeow/util/log"
)

func makeFrame(payload []byte) []byte {
	length := len(payload)
	return append([]byte{byte(length >> 16), byte(length >> 8), byte(length)}, payload...)
}

func makePayload(length int, seed byte) []byte {
	payload := make([]byte, length)
	for i := range payload {
		payload[i] = seed + byte(i)
	}
	return payload
}

func TestFrameSocket_ProcessData(t *testing.T) {
	frameA := makePayload(100, 1)
	frameB := makePayload(50, 101)
	stream := append(makeFrame(frameA), makeFrame(frameB)...)
	lenA := len(frameA) + FrameLengthSize

	tests := []struct {
		name   string
		splits []int
	}{
		{"one message", nil},
		{"frame split in two", []int{40}},
		{"frame split in three", []int{20, 70}},
		{"split at frame boundary", []int{lenA}},
		{"split across both frames", []int{60, lenA + 10}},
		{"split inside header", []int{1, lenA + 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &FrameSocket{log: waLog.Noop, Frames: make(chan []byte, 4)}
			prev := 0
			for _, split := range append(tt.splits, len(stream)) {
				fs.processData(bytes.Clone(stream[prev:split]))
				prev = split
			}
			close(fs.Frames)
			var got [][]byte
			for frame := range fs.Frames {
				got = append(got, frame)
			}
			want := [][]byte{frameA, frameB}
			for i := 0; i < len(got) && i < len(want); i++ {
				if !bytes.Equal(got[i], want[i]) {
					t.Errorf("frame %d mismatch:\n got  %x\n want %x", i, got[i], want[i])
				}
			}
			if len(got) != len(want) {
				t.Errorf("expected %d frames, got %d", len(want), len(got))
			}
		})
	}
}
