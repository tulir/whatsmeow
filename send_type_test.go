// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestGetTypeFromMessage_FutureProofWrappers(t *testing.T) {
	sticker := &waE2E.Message{StickerMessage: &waE2E.StickerMessage{
		Mimetype: proto.String("image/webp"),
		Audio:    &waE2E.StickerMessage_AudioMessage{AudioMessage: &waE2E.AudioMessage{Mimetype: proto.String("audio/ogg; codecs=opus")}},
	}}
	text := &waE2E.Message{Conversation: proto.String("hi")}
	tests := []struct {
		name      string
		msg       *waE2E.Message
		wantType  string
		wantMedia string
	}{
		{"audio sticker", &waE2E.Message{AudioStickerMessage: &waE2E.FutureProofMessage{Message: sticker}}, "media", "sticker"},
		{"lottie sticker", &waE2E.Message{LottieStickerMessage: &waE2E.FutureProofMessage{Message: sticker}}, "media", "sticker"},
		{"group mentioned text", &waE2E.Message{GroupMentionedMessage: &waE2E.FutureProofMessage{Message: text}}, "text", ""},
		{"group mentioned sticker", &waE2E.Message{GroupMentionedMessage: &waE2E.FutureProofMessage{Message: sticker}}, "media", "sticker"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getTypeFromMessage(tt.msg); got != tt.wantType {
				t.Errorf("getTypeFromMessage = %q, want %q", got, tt.wantType)
			}
			if got := getMediaTypeFromMessage(tt.msg); got != tt.wantMedia {
				t.Errorf("getMediaTypeFromMessage = %q, want %q", got, tt.wantMedia)
			}
		})
	}
}
