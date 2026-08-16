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

type groupRekeyDirectiveCorpus struct {
	Schema string                        `json:"schema"`
	Cases  []groupRekeyDirectiveTestCase `json:"cases"`
}

type groupRekeyDirectiveTestCase struct {
	Name               string `json:"name"`
	TransactionID      uint32 `json:"transaction_id"`
	Rekey              string `json:"rekey"`
	WantRekeyRequested bool   `json:"want_rekey_requested"`
}

func TestParseGroupRekeyDirectiveCorpus(t *testing.T) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/68f039c1d44407788d543f2a510afd550c25591c/datasheets/voip-group-rekey-directive.md#L25-L33
	data, err := os.ReadFile("../testdata/group_rekey_directive_corpus.json")
	if err != nil {
		t.Fatalf("read group rekey directive corpus: %v", err)
	}
	var corpus groupRekeyDirectiveCorpus
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("decode group rekey directive corpus: %v", err)
	}
	if corpus.Schema != "whatsmeow.group-rekey-directive-corpus.v1" {
		t.Fatalf("corpus schema = %q", corpus.Schema)
	}
	if len(corpus.Cases) != 2 {
		t.Fatalf("corpus cases = %d, want 2", len(corpus.Cases))
	}

	creator := types.JID{User: "100001", Device: 1, Server: types.HiddenUserServer}
	for _, tc := range corpus.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			// Source of truth: https://github.com/purpshell/meowcaller/blob/68f039c1d44407788d543f2a510afd550c25591c/datasheets/voip-group-rekey-directive.md#L25-L33
			groupAttrs := waBinary.Attrs{
				"call-creator":    creator,
				"call-id":         "CAPTURE-AUTHORITATIVE-CALL-ID",
				"transaction-id":  strconv.FormatUint(uint64(tc.TransactionID), 10),
				"media":           "audio",
				"connected-limit": "32",
			}
			if tc.Rekey != "" {
				groupAttrs["rekey"] = tc.Rekey
			}
			node := waBinary.Node{
				Tag: "group_update",
				Attrs: waBinary.Attrs{
					"call-creator": creator,
					"call-id":      "CAPTURE-AUTHORITATIVE-CALL-ID",
				},
				Content: []waBinary.Node{{
					Tag:   "group_info",
					Attrs: groupAttrs,
				}},
			}

			update, err := ParseGroupUpdate(&node)
			if err != nil {
				t.Fatalf("ParseGroupUpdate: %v", err)
			}
			if update.RekeyRequested != tc.WantRekeyRequested {
				t.Errorf("RekeyRequested = %t, want %t", update.RekeyRequested, tc.WantRekeyRequested)
			}
			if update.TransactionID != tc.TransactionID {
				t.Errorf("TransactionID = %d, want %d", update.TransactionID, tc.TransactionID)
			}
		})
	}
}
