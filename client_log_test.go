// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"strings"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
)

func TestSanitizeNodeForLogRedactsNestedByteContent(t *testing.T) {
	ciphertext := []byte("short printable ciphertext")
	identity := bytes.Repeat([]byte{0xa5}, 32)
	node := waBinary.Node{
		Tag: "call",
		Content: []waBinary.Node{{
			Tag: "enc_rekey",
			Content: []waBinary.Node{
				{Tag: "enc", Content: ciphertext},
				{Tag: "device-identity", Content: identity},
			},
		}},
	}

	sanitized := sanitizeNodeForLog(node)
	rendered := sanitized.String()
	if strings.Contains(rendered, string(ciphertext)) ||
		strings.Contains(rendered, "a5a5a5a5") {
		t.Fatalf("sanitized node leaked byte content: %s", rendered)
	}
	if strings.Count(rendered, "bytes redacted") != 2 {
		t.Fatalf("sanitized node = %s, want two byte-length markers", rendered)
	}
	if !bytes.Equal(node.GetChildren()[0].GetChildren()[0].Content.([]byte), ciphertext) {
		t.Fatal("sanitizing a node mutated the original ciphertext")
	}
}
