// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package voip

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

type groupEncRekeyCorpus struct {
	Schema string                  `json:"schema"`
	Cases  []groupEncRekeyTestCase `json:"cases"`
}

type groupEncRekeyTestCase struct {
	Name              string `json:"name"`
	Author            string `json:"author"`
	CallID            string `json:"call_id"`
	CallCreator       string `json:"call_creator"`
	TransactionID     uint32 `json:"transaction_id"`
	KeyGeneration     uint32 `json:"key_generation"`
	EncryptionType    string `json:"encryption_type"`
	EncryptionVersion uint32 `json:"encryption_version"`
	CiphertextLength  int    `json:"ciphertext_length"`
}

func TestParseGroupCallEncRekeyCorpus(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L39-L65
	corpus := loadGroupEncRekeyCorpus(t)
	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L39-L65
			node, ciphertext := groupEncRekeyNode(t, tc)
			rekey, err := ParseGroupCallEncRekey(&node)
			if err != nil {
				t.Fatalf("ParseGroupCallEncRekey: %v", err)
			}
			wantCreator := mustParseGroupEncRekeyJID(t, tc.CallCreator)
			if rekey.CallID != tc.CallID || rekey.CallCreator != wantCreator {
				t.Errorf("call identity = %q/%s, want %q/%s", rekey.CallID, rekey.CallCreator, tc.CallID, wantCreator)
			}
			if rekey.TransactionID != tc.TransactionID ||
				rekey.KeyGeneration != tc.KeyGeneration ||
				rekey.EncryptionType != tc.EncryptionType ||
				rekey.EncryptionVersion != tc.EncryptionVersion {
				t.Errorf("rekey metadata = %+v", rekey)
			}
			if len(rekey.Ciphertext) != tc.CiphertextLength {
				t.Fatalf("ciphertext length = %d, want %d", len(rekey.Ciphertext), tc.CiphertextLength)
			}
			ciphertext[0] ^= 0xff
			if rekey.Ciphertext[0] == ciphertext[0] {
				t.Fatal("parsed ciphertext aliases the wire node")
			}
		})
	}
}

func TestParseGroupCallEncRekeyRejectsMalformedEnvelope(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L39-L65
	for _, kind := range []string{
		"wrong_tag",
		"missing_transaction",
		"overflow_transaction",
		"missing_encopt",
		"duplicate_encopt",
		"unsupported_keygen",
		"missing_enc",
		"duplicate_enc",
		"unsupported_type",
		"unsupported_version",
		"non_byte_ciphertext",
	} {
		t.Run(kind, func(t *testing.T) {
			// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L39-L65
			node := malformedGroupEncRekeyNode(t, kind)
			if _, err := ParseGroupCallEncRekey(&node); err == nil {
				t.Fatalf("ParseGroupCallEncRekey accepted %s", kind)
			}
		})
	}
}

func loadGroupEncRekeyCorpus(t *testing.T) groupEncRekeyCorpus {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L6-L8
	t.Helper()
	data, err := os.ReadFile("../testdata/group_enc_rekey_corpus.json")
	if err != nil {
		t.Fatalf("read group enc rekey corpus: %v", err)
	}
	var corpus groupEncRekeyCorpus
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("decode group enc rekey corpus: %v", err)
	}
	if corpus.Schema != "whatsmeow.group-enc-rekey-corpus.v1" {
		t.Fatalf("corpus schema = %q", corpus.Schema)
	}
	if len(corpus.Cases) != 3 {
		t.Fatalf("corpus cases = %d, want 3", len(corpus.Cases))
	}
	return corpus
}

func groupEncRekeyNode(t *testing.T, tc groupEncRekeyTestCase) (waBinary.Node, []byte) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L39-L65
	t.Helper()
	ciphertext := make([]byte, tc.CiphertextLength)
	for i := range ciphertext {
		ciphertext[i] = byte(i)
	}
	return waBinary.Node{
		Tag: "enc_rekey",
		Attrs: waBinary.Attrs{
			"call-id":        tc.CallID,
			"call-creator":   mustParseGroupEncRekeyJID(t, tc.CallCreator),
			"transaction-id": strconv.FormatUint(uint64(tc.TransactionID), 10),
		},
		Content: []waBinary.Node{
			{Tag: "encopt", Attrs: waBinary.Attrs{"keygen": strconv.FormatUint(uint64(tc.KeyGeneration), 10)}},
			{
				Tag: "enc",
				Attrs: waBinary.Attrs{
					"type": tc.EncryptionType,
					"v":    strconv.FormatUint(uint64(tc.EncryptionVersion), 10),
				},
				Content: ciphertext,
			},
		},
	}, ciphertext
}

func malformedGroupEncRekeyNode(t *testing.T, kind string) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L39-L65
	t.Helper()
	node, _ := groupEncRekeyNode(t, loadGroupEncRekeyCorpus(t).Cases[0])
	children := node.Content.([]waBinary.Node)
	switch kind {
	case "wrong_tag":
		node.Tag = "group_update"
	case "missing_transaction":
		delete(node.Attrs, "transaction-id")
	case "overflow_transaction":
		node.Attrs["transaction-id"] = "4294967296"
	case "missing_encopt":
		node.Content = children[1:]
	case "duplicate_encopt":
		node.Content = append(children, children[0])
	case "unsupported_keygen":
		children[0].Attrs["keygen"] = "1"
		node.Content = children
	case "missing_enc":
		node.Content = children[:1]
	case "duplicate_enc":
		node.Content = append(children, children[1])
	case "unsupported_type":
		children[1].Attrs["type"] = "skmsg"
		node.Content = children
	case "unsupported_version":
		children[1].Attrs["v"] = "3"
		node.Content = children
	case "non_byte_ciphertext":
		children[1].Content = "ciphertext"
		node.Content = children
	default:
		t.Fatalf("unknown malformed rekey case %q", kind)
	}
	return node
}

func mustParseGroupEncRekeyJID(t *testing.T, raw string) types.JID {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L42-L50
	t.Helper()
	jid, err := types.ParseJID(raw)
	if err != nil {
		t.Fatalf("parse JID %q: %v", raw, err)
	}
	return jid
}
