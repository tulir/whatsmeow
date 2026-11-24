// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

func TestGetButtonTypeFromMessage(t *testing.T) {
	cases := []struct {
		name string
		msg  *waE2E.Message
		want string
	}{
		{"buttons", &waE2E.Message{ButtonsMessage: &waE2E.ButtonsMessage{}}, "buttons"},
		{"list", &waE2E.Message{ListMessage: &waE2E.ListMessage{}}, "list"},
		{"ephemeral list", &waE2E.Message{EphemeralMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{ListMessage: &waE2E.ListMessage{}}}}, "list"},
		{"buttons response", &waE2E.Message{ButtonsResponseMessage: &waE2E.ButtonsResponseMessage{}}, ""},
		{"list response", &waE2E.Message{ListResponseMessage: &waE2E.ListResponseMessage{}}, ""},
		{"interactive response", &waE2E.Message{InteractiveResponseMessage: &waE2E.InteractiveResponseMessage{}}, ""},
		{"text", &waE2E.Message{Conversation: new("hi")}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := getButtonTypeFromMessage(c.msg); got != c.want {
				t.Errorf("getButtonTypeFromMessage() = %q, want %q", got, c.want)
			}
		})
	}
}
