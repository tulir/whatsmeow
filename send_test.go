// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"testing"

	"go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

func secretEncryptedMessage(encType waE2E.SecretEncryptedMessage_SecretEncType) *waE2E.Message {
	return &waE2E.Message{
		SecretEncryptedMessage: &waE2E.SecretEncryptedMessage{SecretEncType: encType.Enum()},
	}
}

func TestEventEditStanza(t *testing.T) {
	msg := secretEncryptedMessage(waE2E.SecretEncryptedMessage_EVENT_EDIT)
	if got := getTypeFromMessage(msg); got != "event" {
		t.Errorf("type = %q, want event", got)
	}
	if got := getEditAttribute(msg); got != types.EditAttributeMessageEdit {
		t.Errorf("edit = %q, want %q", got, types.EditAttributeMessageEdit)
	}
	content := (&Client{}).getMessageContent(binary.Node{Tag: "enc"}, msg, binary.Attrs{"type": "event"}, false, nodeExtraParams{})
	if len(content) != 2 || content[1].Tag != "meta" || content[1].Attrs["event_type"] != "edit" {
		t.Errorf("content = %v, want enc followed by <meta event_type=edit>", content)
	}
}

func TestOtherSecretEncryptedMessageStanza(t *testing.T) {
	msg := secretEncryptedMessage(waE2E.SecretEncryptedMessage_MESSAGE_EDIT)
	if got := getTypeFromMessage(msg); got != "text" {
		t.Errorf("type = %q, want text", got)
	}
	if got := getEditAttribute(msg); got != types.EditAttributeEmpty {
		t.Errorf("edit = %q, want empty", got)
	}
	content := (&Client{}).getMessageContent(binary.Node{Tag: "enc"}, msg, binary.Attrs{"type": "text"}, false, nodeExtraParams{})
	if len(content) != 1 {
		t.Errorf("content = %v, want only enc", content)
	}
}
