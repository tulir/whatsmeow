// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"encoding/json"
	"os"
	"testing"
)

type groupUpdateCorpus struct {
	Schema string            `json:"schema"`
	Cases  []json.RawMessage `json:"cases"`
}

func TestGroupUpdateIngestionCorpus(t *testing.T) {
	vector, err := os.ReadFile("testdata/group_update_corpus.json")
	if err != nil {
		t.Fatalf("read group-update corpus: %v", err)
	}
	var corpus groupUpdateCorpus
	if err = json.Unmarshal(vector, &corpus); err != nil {
		t.Fatalf("decode group-update corpus: %v", err)
	}
	if corpus.Schema != "whatsmeow.group-update-corpus.v1" {
		t.Fatalf("corpus schema = %q", corpus.Schema)
	}
	if len(corpus.Cases) != 4 {
		t.Fatalf("corpus cases = %d, want 4", len(corpus.Cases))
	}
	t.Skip("blocked: group_update ingestion is a stub; enable when implemented")
}
