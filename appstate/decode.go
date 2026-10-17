// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package appstate

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
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
		return hmac.Equal(mac.IndexMAC, indexMAC)
	})
}

func (out *patchOutput) AddMAC(indexMAC, valueMAC, indexHash []byte) {
	out.AddedMACs = append(out.AddedMACs, store.AppStateMutationMAC{
		IndexMAC:  indexMAC,
		ValueMAC:  valueMAC,
		IndexHash: indexHash,
	})
}

// indexHash identifies a logical index independently of the app state key that
// encrypted it. The index MAC cannot do that: it is keyed by the app state key,
// so a rewrite of the same index under a rotated key gets a different one.
func indexHash(indexJSON []byte) []byte {
	sum := sha256.Sum256(indexJSON)
	return sum[:]
}

// mutationPlan is what one mutation supersedes: the records of its logical
// index that the state currently holds, whether written under the same key
// (same index MAC) or under an earlier one.
type mutationPlan struct {
	indexHash []byte
	previous  []store.AppStateMutationMAC
}

type mutationPlans []mutationPlan

func (plans mutationPlans) previousValueMACs(_ []byte, i int) ([][]byte, error) {
	if i >= len(plans) {
		return nil, nil
	}
	removals := make([][]byte, 0, len(plans[i].previous))
	for _, prev := range plans[i].previous {
		removals = append(removals, prev.ValueMAC)
	}
	return removals, nil
}

// sameIndexMACOnly restricts every plan to the records that share the
// mutation's index MAC, which is what versions of this library before index
// hashes existed subtracted. A patch authored by such a companion verifies only
// under that rule, and the records it left in place stay in the store: the
// next SET of the index by the primary subtracts them all.
func (plans mutationPlans) sameIndexMACOnly(mutations []*waServerSync.SyncdMutation) mutationPlans {
	restricted := make(mutationPlans, len(plans))
	for i, plan := range plans {
		restricted[i].indexHash = plan.indexHash
		indexMAC := mutations[i].GetRecord().GetIndex().GetBlob()
		for _, prev := range plan.previous {
			if hmac.Equal(prev.IndexMAC, indexMAC) {
				restricted[i].previous = append(restricted[i].previous, prev)
			}
		}
	}
	return restricted
}

func (plans mutationPlans) collapsesAnything() bool {
	for _, plan := range plans {
		if len(plan.previous) > 0 {
			return true
		}
	}
	return false
}

// planMutations finds, for every mutation, the records it supersedes.
//
// The primary device keeps one value per logical index: when it SETs an index
// it subtracts every record it holds for that index, including records
// written under earlier keys, whose index MACs differ. A companion that only
// subtracts the record with the same index MAC computes a different LTHash
// and fails the snapshot MAC. Earlier mutations in the same list count as held
// records too, which is the rule updateHash already applied to repeated index
// MACs inside one patch. With lookupStore the stored records are consulted as
// well, by index hash and, for rows stored before hashes were kept, by index
// MAC. A mutation whose key is unavailable keeps the index-MAC rule only;
// decodeMutations reports the missing key afterwards.
func (proc *Processor) planMutations(ctx context.Context, name WAPatchName, mutations []*waServerSync.SyncdMutation, lookupStore bool) (mutationPlans, error) {
	plans := make(mutationPlans, len(mutations))
	byHash := make(map[string]store.AppStateMutationMAC)
	byIndexMAC := make(map[string]store.AppStateMutationMAC)
	consumed := make(map[string]struct{})
	for i, mutation := range mutations {
		record := mutation.GetRecord()
		indexMAC := record.GetIndex().GetBlob()
		plan := &plans[i]
		_, _, _, syncAction, _, err := proc.decodeMutation(ctx, mutation, i, false)
		if err == nil {
			plan.indexHash = indexHash(syncAction.GetIndex())
		} else if !errors.Is(err, ErrKeyNotFound) {
			return nil, err
		}
		seen := make(map[string]struct{})
		addPrevious := func(mac store.AppStateMutationMAC) {
			if _, dup := seen[string(mac.IndexMAC)]; dup {
				return
			}
			seen[string(mac.IndexMAC)] = struct{}{}
			plan.previous = append(plan.previous, mac)
		}
		if plan.indexHash != nil {
			if prev, ok := byHash[string(plan.indexHash)]; ok {
				addPrevious(prev)
			}
		}
		if prev, ok := byIndexMAC[string(indexMAC)]; ok {
			addPrevious(prev)
		}
		if lookupStore {
			if plan.indexHash != nil {
				stored, err := proc.Store.AppState.GetAppStateMutationMACsByIndexHash(ctx, string(name), plan.indexHash)
				if err != nil {
					return nil, fmt.Errorf("failed to get previous mutations of index: %w", err)
				}
				for _, mac := range stored {
					if _, done := consumed[string(mac.IndexMAC)]; !done {
						addPrevious(mac)
					}
				}
			}
			if _, done := consumed[string(indexMAC)]; !done {
				valueMAC, err := proc.Store.AppState.GetAppStateMutationMAC(ctx, string(name), indexMAC)
				if err != nil {
					return nil, fmt.Errorf("failed to get value MAC of previous SET operation: %w", err)
				}
				if valueMAC != nil {
					addPrevious(store.AppStateMutationMAC{IndexMAC: indexMAC, ValueMAC: valueMAC})
				}
			}
		}
		for _, prev := range plan.previous {
			consumed[string(prev.IndexMAC)] = struct{}{}
			delete(byIndexMAC, string(prev.IndexMAC))
			if prev.IndexHash != nil {
				delete(byHash, string(prev.IndexHash))
			}
		}
		if plan.indexHash != nil {
			delete(byHash, string(plan.indexHash))
		}
		if mutation.GetOperation() == waServerSync.SyncdMutation_SET {
			value := record.GetValue().GetBlob()
			if len(value) < 32 {
				return nil, fmt.Errorf("failed to read value MAC of mutation #%d: value too short", i+1)
			}
			current := store.AppStateMutationMAC{IndexMAC: indexMAC, ValueMAC: value[len(value)-32:], IndexHash: plan.indexHash}
			byIndexMAC[string(indexMAC)] = current
			if plan.indexHash != nil {
				byHash[string(plan.indexHash)] = current
			}
		}
	}
	return plans, nil
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
		if !hmac.Equal(expectedValueMAC, valueMAC) {
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
		if !hmac.Equal(expectedIndexMAC, indexMAC) {
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
	plans mutationPlans,
) error {
	for i, mutation := range mutations {
		indexMAC, valueMAC, index, syncAction, _, err := proc.decodeMutation(ctx, mutation, i, validateMACs)
		if err != nil {
			return err
		}
		var plan mutationPlan
		if i < len(plans) {
			plan = plans[i]
		}
		for _, prev := range plan.previous {
			out.RemoveMAC(prev.IndexMAC)
		}
		if mutation.GetOperation() == waServerSync.SyncdMutation_REMOVE {
			out.RemoveMAC(indexMAC)
			altIndexMAC, ok := fakeIndexesToRemove[indexMACToArray(indexMAC)]
			if ok && len(indexMAC) == 32 {
				out.RemoveMAC(altIndexMAC)
			}
		} else if mutation.GetOperation() == waServerSync.SyncdMutation_SET {
			out.AddMAC(indexMAC, valueMAC, plan.indexHash)
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
	if !hmac.Equal(snapshotMAC, expectedSnapshotMAC) {
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

	// The server keeps snapshot records by index MAC, so a snapshot can carry a
	// superseded record next to the current one for the same logical index. The
	// primary device signs the snapshot over one value per index, so the
	// superseded records are collapsed before hashing. A primary that adopted a
	// companion's duplicate-carrying state signs over every record instead; the
	// uncollapsed hash is kept as the fallback for that case.
	var plans mutationPlans
	plans, err = proc.planMutations(ctx, name, encryptedMutations, false)
	if err != nil {
		return
	}
	var fakeIndexesToRemove map[[32]byte][]byte
	var warn []error
	collapsedState := currentState
	warn, err = collapsedState.updateHash(encryptedMutations, plans.previousValueMACs)
	if err != nil {
		err = fmt.Errorf("failed to update state hash: %w", err)
		return
	}

	if validateMACs {
		_, err = proc.validateSnapshotMAC(ctx, name, collapsedState, ss.GetKeyID().GetID(), ss.GetMac())
		if errors.Is(err, ErrMismatchingLTHash) && plans.collapsesAnything() {
			plainState := currentState
			if _, plainErr := plainState.updateHash(encryptedMutations, func([]byte, int) ([][]byte, error) { return nil, nil }); plainErr == nil {
				if _, plainErr = proc.validateSnapshotMAC(ctx, name, plainState, ss.GetKeyID().GetID(), ss.GetMac()); plainErr == nil {
					proc.Log.Warnf("App state %s snapshot v%d only verifies with its superseded records counted", name, collapsedState.Version)
					collapsedState = plainState
					for i := range plans {
						plans[i].previous = nil
					}
					err = nil
				}
			}
		}
		if err != nil {
			if len(warn) > 0 {
				proc.Log.Warnf("Warnings while updating hash for %s: %+v", name, warn)
			}
			err = fmt.Errorf("failed to verify snapshot: %w", err)
			return
		}
	}
	currentState = collapsedState

	var out patchOutput
	out.Mutations = newMutationsInput
	err = proc.decodeMutations(ctx, encryptedMutations, &out, validateMACs, currentState.Version, fakeIndexesToRemove, plans)
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
	plans mutationPlans,
) (newState HashState, warn []error, usedPlans mutationPlans, err error) {
	version := patch.GetVersion().GetVersion()
	newState = currentState
	newState.Version = version
	usedPlans = plans
	warn, err = newState.updateHash(patch.GetMutations(), plans.previousValueMACs)
	if err != nil {
		err = fmt.Errorf("failed to update state hash: %w", err)
		return
	}

	if validateMACs {
		var keys ExpandedAppStateKeys
		keys, err = proc.validateSnapshotMAC(ctx, patchName, newState, patch.GetKeyID().GetID(), patch.GetSnapshotMAC())
		if errors.Is(err, ErrMismatchingLTHash) && plans.collapsesAnything() {
			// A companion running an older version of this library subtracts
			// only the record with the same index MAC. Its patches are in the
			// server's history too, so verify them the way they were made.
			legacyPlans := plans.sameIndexMACOnly(patch.GetMutations())
			legacyState := currentState
			legacyState.Version = version
			if _, legacyErr := legacyState.updateHash(patch.GetMutations(), legacyPlans.previousValueMACs); legacyErr == nil {
				if legacyKeys, legacyErr := proc.validateSnapshotMAC(ctx, patchName, legacyState, patch.GetKeyID().GetID(), patch.GetSnapshotMAC()); legacyErr == nil {
					proc.Log.Warnf("App state %s patch v%d was made without subtracting superseded records; keeping them", patchName, version)
					newState, keys, usedPlans, err = legacyState, legacyKeys, legacyPlans, nil
				}
			}
		}
		if err != nil {
			return
		}
		patchMAC := generatePatchMAC(patch, patchName, keys.PatchMAC, patch.GetVersion().GetVersion())
		if !hmac.Equal(patchMAC, patch.GetPatchMAC()) {
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
		var plans mutationPlans
		plans, err = proc.planMutations(ctx, list.Name, patch.GetMutations(), true)
		if err != nil {
			return
		}
		newState, warn, plans, err = proc.validatePatch(ctx, list.Name, patch, currentState, validateMACs, plans)
		if err != nil {
			if len(warn) > 0 {
				proc.Log.Warnf("Warnings while updating hash for %s: %+v", list.Name, warn)
			}
			return
		}

		out.Mutations = newMutations
		err = proc.decodeMutations(ctx, patch.GetMutations(), &out, validateMACs, newState.Version, fakeIndexesToRemove, plans)
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
