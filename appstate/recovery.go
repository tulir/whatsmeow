// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package appstate

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waServerSync"
	"go.mau.fi/whatsmeow/proto/waSyncdSnapshotRecovery"
	"go.mau.fi/whatsmeow/store"
)

func ParseRecovery(
	resp *waE2E.PeerDataOperationRequestResponseMessage_PeerDataOperationResult_SyncDSnapshotFatalRecoveryResponse,
) (*waSyncdSnapshotRecovery.SyncdSnapshotRecovery, error) {
	data := resp.GetCollectionSnapshot()
	if resp.GetIsCompressed() {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to start decompressing: %w", err)
		}
		data, err = io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to decompress: %w", err)
		} else if closeErr != nil {
			return nil, fmt.Errorf("failed to close decompress reader: %w", closeErr)
		}
	}
	var out waSyncdSnapshotRecovery.SyncdSnapshotRecovery
	err := proto.Unmarshal(data, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (proc *Processor) ProcessRecovery(ctx context.Context, recovery *waSyncdSnapshotRecovery.SyncdSnapshotRecovery) ([]Mutation, error) {
	if len(recovery.GetCollectionLthash()) != 128 {
		return nil, fmt.Errorf("invalid lthash length: %d", len(recovery.GetCollectionLthash()))
	}
	name := recovery.GetCollectionName()
	version := recovery.GetVersion().GetVersion()
	macs := make([]store.AppStateMutationMAC, len(recovery.MutationRecords))
	mutations := make([]Mutation, len(recovery.MutationRecords))
	for i, mutation := range recovery.MutationRecords {
		keys, err := proc.getAppStateKey(ctx, mutation.GetKeyID())
		if err != nil {
			return nil, fmt.Errorf("failed to get key %x for mutation #%d: %w", mutation.GetKeyID(), i+1, err)
		}
		var parsedIndex []string
		err = json.Unmarshal(mutation.GetValue().GetIndex(), &parsedIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal index for mutation #%d: %w", i+1, err)
		}
		macs[i] = store.AppStateMutationMAC{
			IndexMAC: concatAndHMAC(sha256.New, keys.Index, mutation.GetValue().GetIndex()),
			ValueMAC: mutation.GetMac(),
		}
		mutations[i] = Mutation{
			KeyID:        mutation.GetKeyID(),
			Operation:    waServerSync.SyncdMutation_SET,
			Action:       mutation.GetValue().GetValue(),
			Version:      mutation.GetValue().GetVersion(),
			Index:        parsedIndex,
			IndexMAC:     macs[i].IndexMAC,
			ValueMAC:     macs[i].ValueMAC,
			PatchVersion: version,
		}
	}
	err := proc.Store.AppState.DeleteAppStateVersion(ctx, name)
	if err != nil {
		return mutations, fmt.Errorf("failed to reset app state version in database: %w", err)
	}
	err = proc.Store.AppState.PutAppStateVersion(ctx, name, version, *(*[128]byte)(recovery.GetCollectionLthash()))
	if err != nil {
		return mutations, fmt.Errorf("failed to update app state version in the database: %w", err)
	}
	err = proc.Store.AppState.PutAppStateMutationMACs(ctx, name, version, macs)
	if err != nil {
		return mutations, fmt.Errorf("failed to insert added mutation MACs to the database: %w", err)
	}
	return mutations, nil
}
