// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package appstate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"

	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waServerSync"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/util/cbcutil"
)

// PatchList represents a decoded response to getting app state patches from the WhatsApp servers.
type PatchList struct {
	Name           WAPatchName
	HasMorePatches bool
	Patches        []*waServerSync.SyncdPatch
	Snapshot       *waServerSync.SyncdSnapshot
}

// DownloadExternalFunc is a function that can download a blob of external app state patches.
type DownloadExternalFunc func(context.Context, *waServerSync.ExternalBlobReference) ([]byte, error)

func parseSnapshotInternal(ctx context.Context, collection *waBinary.Node, downloadExternal DownloadExternalFunc) (*waServerSync.SyncdSnapshot, error) {
	snapshotNode := collection.GetChildByTag("snapshot")
	rawSnapshot, ok := snapshotNode.Content.([]byte)
	if snapshotNode.Tag != "snapshot" || !ok {
		return nil, nil
	}
	var snapshot waServerSync.ExternalBlobReference
	err := proto.Unmarshal(rawSnapshot, &snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}
	var rawData []byte
	rawData, err = downloadExternal(ctx, &snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to download external mutations: %w", err)
	}
	var downloaded waServerSync.SyncdSnapshot
	err = proto.Unmarshal(rawData, &downloaded)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal mutation list: %w", err)
	}
	return &downloaded, nil
}

func parsePatchListInternal(ctx context.Context, collection *waBinary.Node, downloadExternal DownloadExternalFunc) ([]*waServerSync.SyncdPatch, error) {
	patchesNode := collection.GetChildByTag("patches")
	patchNodes := patchesNode.GetChildren()
	patches := make([]*waServerSync.SyncdPatch, 0, len(patchNodes))
	for i, patchNode := range patchNodes {
		rawPatch, ok := patchNode.Content.([]byte)
		if patchNode.Tag != "patch" || !ok {
			continue
		}
		var patch waServerSync.SyncdPatch
		err := proto.Unmarshal(rawPatch, &patch)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal patch #%d: %w", i+1, err)
		}
		if patch.GetExternalMutations() != nil && downloadExternal != nil {
			var rawData []byte
			rawData, err = downloadExternal(ctx, patch.GetExternalMutations())
			if err != nil {
				return nil, fmt.Errorf("failed to download external mutations: %w", err)
			}
			var downloaded waServerSync.SyncdMutations
			err = proto.Unmarshal(rawData, &downloaded)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal mutation list: %w", err)
			} else if len(downloaded.GetMutations()) == 0 {
				return nil, fmt.Errorf("didn't get any mutations from download")
			}
			patch.Mutations = downloaded.Mutations
		}
		patches = append(patches, &patch)
	}
	return patches, nil
}

// ParsePatchList will decode an XML node containing app state patches, including downloading any external blobs.
func ParsePatchList(ctx context.Context, collection *waBinary.Node, downloadExternal DownloadExternalFunc) (*PatchList, error) {
	ag := collection.AttrGetter()
	snapshot, err := parseSnapshotInternal(ctx, collection, downloadExternal)
	if err != nil {
		return nil, err
	}
	patches, err := parsePatchListInternal(ctx, collection, downloadExternal)
	if err != nil {
		return nil, err
	}
	list := &PatchList{
		Name:           WAPatchName(ag.String("name")),
		HasMorePatches: ag.OptionalBool("has_more_patches"),
		Patches:        patches,
		Snapshot:       snapshot,
	}
	return list, ag.Error()
}

type patchOutput struct {
	RemovedMACs [][]byte
	AddedMACs   []store.AppStateMutationMAC
	Mutations   []Mutation
}

func (out *patchOutput) RemoveMAC(indexMAC []byte) {
	out.RemovedMACs = append(out.RemovedMACs, indexMAC)
	// If the mutation was previously added in this patch, remove it from AddedMACs
	out.AddedMACs = slices.DeleteFunc(out.AddedMACs, func(mac store.AppStateMutationMAC) bool {
		return bytes.Equal(mac.IndexMAC, indexMAC)
	})
}

func (out *patchOutput) AddMAC(indexMAC, valueMAC []byte) {
	out.AddedMACs = append(out.AddedMACs, store.AppStateMutationMAC{
		IndexMAC: indexMAC,
		ValueMAC: valueMAC,
	})
}

func (proc *Processor) decodeMutation(
	ctx context.Context,
	mutation *waServerSync.SyncdMutation,
	i int,
	validateMACs bool,
) (indexMAC, valueMAC []byte, index []string, syncAction *waSyncAction.SyncActionData, keys ExpandedAppStateKeys, err error) {
	keyID := mutation.GetRecord().GetKeyID().GetID()
	keys, err = proc.getAppStateKey(ctx, keyID)
	if err != nil {
		err = fmt.Errorf("failed to get key %X to decode mutation: %w", keyID, err)
		return
	}
	content := bytes.Clone(mutation.GetRecord().GetValue().GetBlob())
	content, valueMAC = content[:len(content)-32], content[len(content)-32:]
	if validateMACs {
		expectedValueMAC := generateContentMAC(mutation.GetOperation(), content, keyID, keys.ValueMAC)
		if !bytes.Equal(expectedValueMAC, valueMAC) {
			err = fmt.Errorf("failed to verify mutation #%d: %w", i+1, ErrMismatchingContentMAC)
			return
		}
	}
	iv, content := content[:16], content[16:]
	plaintext, err := cbcutil.Decrypt(keys.ValueEncryption, iv, content)
	if err != nil {
		err = fmt.Errorf("failed to decrypt mutation #%d: %w", i+1, err)
		return
	}
	syncAction = &waSyncAction.SyncActionData{}
	err = proto.Unmarshal(plaintext, syncAction)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal mutation #%d: %w", i+1, err)
		return
	}
	indexMAC = mutation.GetRecord().GetIndex().GetBlob()
	if validateMACs {
		expectedIndexMAC := concatAndHMAC(sha256.New, keys.Index, syncAction.Index)
		if !bytes.Equal(expectedIndexMAC, indexMAC) {
			err = fmt.Errorf("failed to verify mutation #%d: %w", i+1, ErrMismatchingIndexMAC)
			return
		}
	}
	err = json.Unmarshal(syncAction.GetIndex(), &index)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal index of mutation #%d: %w", i+1, err)
	}
	return
}

func indexMACToArray(indexMAC []byte) [32]byte {
	if len(indexMAC) != 32 {
		return [32]byte{}
	}
	return *(*[32]byte)(indexMAC)
}

func (proc *Processor) decodeMutations(
	ctx context.Context,
	mutations []*waServerSync.SyncdMutation,
	out *patchOutput,
	validateMACs bool,
	patchVersion uint64,
	fakeIndexesToRemove map[[32]byte][]byte,
) error {
	for i, mutation := range mutations {
		indexMAC, valueMAC, index, syncAction, _, err := proc.decodeMutation(ctx, mutation, i, validateMACs)
		if err != nil {
			return err
		}
		if mutation.GetOperation() == waServerSync.SyncdMutation_REMOVE {
			out.RemoveMAC(indexMAC)
			altIndexMAC, ok := fakeIndexesToRemove[indexMACToArray(indexMAC)]
			if ok && len(indexMAC) == 32 {
				out.RemoveMAC(altIndexMAC)
			}
		} else if mutation.GetOperation() == waServerSync.SyncdMutation_SET {
			out.AddMAC(indexMAC, valueMAC)
		}
		out.Mutations = append(out.Mutations, Mutation{
			KeyID:     mutation.GetRecord().GetKeyID().GetID(),
			Operation: mutation.GetOperation(),
			Action:    syncAction.GetValue(),
			Version:   syncAction.GetVersion(),
			Index:     index,
			IndexMAC:  indexMAC,
			ValueMAC:  valueMAC,

			PatchVersion: patchVersion,
		})
	}
	return nil
}

func (proc *Processor) storeMACs(ctx context.Context, name WAPatchName, currentState HashState, out *patchOutput) error {
	err := proc.Store.AppState.PutAppStateVersion(ctx, string(name), currentState.Version, currentState.Hash)
	if err != nil {
		return fmt.Errorf("failed to update app state version in the database: %w", err)
	}
	err = proc.Store.AppState.DeleteAppStateMutationMACs(ctx, string(name), out.RemovedMACs)
	if err != nil {
		return fmt.Errorf("failed to remove deleted mutation MACs from the database: %w", err)
	}
	err = proc.Store.AppState.PutAppStateMutationMACs(ctx, string(name), currentState.Version, out.AddedMACs)
	if err != nil {
		return fmt.Errorf("failed to insert added mutation MACs to the database: %w", err)
	}
	return nil
}

func (proc *Processor) validateSnapshotMAC(ctx context.Context, name WAPatchName, currentState HashState, keyID, expectedSnapshotMAC []byte) (keys ExpandedAppStateKeys, err error) {
	keys, err = proc.getAppStateKey(ctx, keyID)
	if err != nil {
		err = fmt.Errorf("failed to get key %X to verify patch v%d MACs: %w", keyID, currentState.Version, err)
		return
	}
	snapshotMAC := currentState.generateSnapshotMAC(name, keys.SnapshotMAC)
	if !bytes.Equal(snapshotMAC, expectedSnapshotMAC) {
		err = fmt.Errorf("failed to verify patch v%d: %w", currentState.Version, ErrMismatchingLTHash)
	}
	return
}

func (proc *Processor) decodeSnapshot(
	ctx context.Context,
	name WAPatchName,
	ss *waServerSync.SyncdSnapshot,
	initialState HashState,
	validateMACs bool,
	newMutationsInput []Mutation,
) (newMutations []Mutation, currentState HashState, err error) {
	currentState = initialState
	currentState.Version = ss.GetVersion().GetVersion()

	encryptedMutations := make([]*waServerSync.SyncdMutation, len(ss.GetRecords()))
	for i, record := range ss.GetRecords() {
		encryptedMutations[i] = &waServerSync.SyncdMutation{
			Operation: waServerSync.SyncdMutation_SET.Enum(),
			Record:    record,
		}
	}

	var fakeIndexesToRemove map[[32]byte][]byte
	var warn []error
	warn, err = currentState.updateHash(encryptedMutations, func(indexMAC []byte, maxIndex int) ([]byte, error) {
		return nil, nil
	})
	if err != nil {
		err = fmt.Errorf("failed to update state hash: %w", err)
		return
	}

	if validateMACs {
		_, err = proc.validateSnapshotMAC(ctx, name, currentState, ss.GetKeyID().GetID(), ss.GetMac())
		if err != nil {
			if len(warn) > 0 {
				proc.Log.Warnf("Warnings while updating hash for %s: %+v", name, warn)
			}
			err = fmt.Errorf("failed to verify snapshot: %w", err)
			return
		}
	}

	var out patchOutput
	out.Mutations = newMutationsInput
	err = proc.decodeMutations(ctx, encryptedMutations, &out, validateMACs, currentState.Version, fakeIndexesToRemove)
	if err != nil {
		err = fmt.Errorf("failed to decode snapshot of v%d: %w", currentState.Version, err)
		return
	}
	err = proc.storeMACs(ctx, name, currentState, &out)
	if err != nil {
		return
	}
	newMutations = out.Mutations
	return
}

func (proc *Processor) validatePatch(
	ctx context.Context,
	patchName WAPatchName,
	patch *waServerSync.SyncdPatch,
	currentState HashState,
	validateMACs bool,
) (newState HashState, warn []error, err error) {
	version := patch.GetVersion().GetVersion()
	newState = currentState
	newState.Version = version
	warn, err = newState.updateHash(patch.GetMutations(), func(indexMAC []byte, maxIndex int) ([]byte, error) {
		for i := maxIndex - 1; i >= 0; i-- {
			if bytes.Equal(patch.Mutations[i].GetRecord().GetIndex().GetBlob(), indexMAC) {
				if patch.Mutations[i].GetOperation() == waServerSync.SyncdMutation_SET {
					value := patch.Mutations[i].GetRecord().GetValue().GetBlob()
					return value[len(value)-32:], nil
				}
				// Found a REMOVE operation, no previous value
				return nil, nil
			}
		}
		// Previous value not found in current patch, look in the database
		return proc.Store.AppState.GetAppStateMutationMAC(ctx, string(patchName), indexMAC)
	})
	if err != nil {
		err = fmt.Errorf("failed to update state hash: %w", err)
		return
	}

	if validateMACs {
		var keys ExpandedAppStateKeys
		keys, err = proc.validateSnapshotMAC(ctx, patchName, newState, patch.GetKeyID().GetID(), patch.GetSnapshotMAC())
		if err != nil {
			return
		}
		patchMAC := generatePatchMAC(patch, patchName, keys.PatchMAC, patch.GetVersion().GetVersion())
		if !bytes.Equal(patchMAC, patch.GetPatchMAC()) {
			err = fmt.Errorf("failed to verify patch v%d: %w", version, ErrMismatchingPatchMAC)
			return
		}
	}
	return
}

// DecodePatches will decode all the patches in a PatchList into a list of app state mutations.
func (proc *Processor) DecodePatches(
	ctx context.Context,
	list *PatchList,
	initialState HashState,
	validateMACs bool,
) (newMutations []Mutation, currentState HashState, err error) {
	currentState = initialState
	var expectedLength int
	if list.Snapshot != nil {
		expectedLength = len(list.Snapshot.GetRecords())
	}
	for _, patch := range list.Patches {
		expectedLength += len(patch.GetMutations())
	}
	newMutations = make([]Mutation, 0, expectedLength)

	if list.Snapshot != nil {
		newMutations, currentState, err = proc.decodeSnapshot(ctx, list.Name, list.Snapshot, currentState, validateMACs, newMutations)
		if err != nil {
			return
		}
	}

	for _, patch := range list.Patches {
		var out patchOutput
		var warn []error
		var newState HashState
		var fakeIndexesToRemove map[[32]byte][]byte
		newState, warn, err = proc.validatePatch(ctx, list.Name, patch, currentState, validateMACs)
		if err != nil {
			if len(warn) > 0 {
				proc.Log.Warnf("Warnings while updating hash for %s: %+v", list.Name, warn)
			}
			return
		}

		out.Mutations = newMutations
		err = proc.decodeMutations(ctx, patch.GetMutations(), &out, validateMACs, newState.Version, fakeIndexesToRemove)
		if err != nil {
			return
		}
		err = proc.storeMACs(ctx, list.Name, newState, &out)
		if err != nil {
			return
		}
		newMutations = out.Mutations
		currentState = newState
	}
	return
}
