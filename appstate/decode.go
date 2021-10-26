// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package appstate

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/util/cbcutil"
)

type PatchList struct {
	Name           WAPatchName
	HasMorePatches bool
	Patches        []*waProto.SyncdPatch
}

type DownloadExternalFunc func(*waProto.ExternalBlobReference) (*waProto.SyncdMutations, error)

func ParsePatchList(node *waBinary.Node, downloadExternal DownloadExternalFunc) (*PatchList, error) {
	collection := node.GetChildByTag("sync", "collection")
	ag := collection.AttrGetter()
	patchesNode := collection.GetChildByTag("patches")
	patchNodes := patchesNode.GetChildren()
	patches := make([]*waProto.SyncdPatch, 0, len(patchNodes))
	for i, patchNode := range patchNodes {
		rawPatch, ok := patchNode.Content.([]byte)
		if patchNode.Tag != "patch" || !ok {
			continue
		}
		var patch waProto.SyncdPatch
		err := proto.Unmarshal(rawPatch, &patch)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal patch #%d: %w", i+1, err)
		}
		if patch.GetExternalMutations() != nil && downloadExternal != nil {
			downloaded, err := downloadExternal(patch.GetExternalMutations())
			if downloaded != nil {
				patch.Mutations = downloaded.GetMutations()
			} else if err != nil {
				return nil, fmt.Errorf("failed to download external mutations: %w", err)
			}
		}
		patches = append(patches, &patch)
	}
	list := &PatchList{
		Name:           WAPatchName(ag.String("name")),
		HasMorePatches: ag.OptionalBool("has_more_patches"),
		Patches:        patches,
	}
	return list, ag.Error()
}

type patchOutput struct {
	RemovedMACs [][]byte
	AddedMACs   []store.AppStateMutationMAC
	Mutations   []Mutation
}

func (proc *Processor) decodePatch(patch *waProto.SyncdPatch, out *patchOutput, validateMACs bool) error {
	for i, mutation := range patch.Mutations {
		keyID := mutation.GetRecord().GetKeyId().GetId()
		keys, err := proc.getAppStateKey(keyID)
		if err != nil {
			return fmt.Errorf("failed to get key %X to decode mutation", keyID)
		}
		content := mutation.GetRecord().GetValue().GetBlob()
		content, valueMAC := content[:len(content)-32], content[len(content)-32:]
		if validateMACs {
			expectedValueMAC := generateContentMAC(mutation.GetOperation(), content, keyID, keys.ValueMAC)
			if !bytes.Equal(expectedValueMAC, valueMAC) {
				return fmt.Errorf("failed to verify mutation #%d: %w", i+1, ErrMismatchingContentMAC)
			}
		}
		iv, content := content[:16], content[16:]
		plaintext, err := cbcutil.Decrypt(keys.ValueEncryption, iv, content)
		if err != nil {
			return fmt.Errorf("failed to decrypt mutation #%d: %w", i+1, err)
		}
		var syncAction waProto.SyncActionData
		err = proto.Unmarshal(plaintext, &syncAction)
		if err != nil {
			return fmt.Errorf("failed to unmarshal mutation #%d: %w", i+1, err)
		}
		indexMAC := mutation.GetRecord().GetIndex().GetBlob()
		if validateMACs {
			expectedIndexMAC := concatAndHMAC(sha256.New, keys.Index, syncAction.Index)
			if !bytes.Equal(expectedIndexMAC, indexMAC) {
				return fmt.Errorf("failed to verify mutation #%d: %w", i+1, ErrMismatchingIndexMAC)
			}
		}
		var index []string
		err = json.Unmarshal(syncAction.GetIndex(), &index)
		if err != nil {
			return fmt.Errorf("failed to unmarshal index of mutation #%d: %w", i+1, err)
		}
		if mutation.GetOperation() == waProto.SyncdMutation_REMOVE {
			out.RemovedMACs = append(out.RemovedMACs, indexMAC)
		} else if mutation.GetOperation() == waProto.SyncdMutation_SET {
			out.AddedMACs = append(out.AddedMACs, store.AppStateMutationMAC{
				IndexMAC: indexMAC,
				ValueMAC: valueMAC,
			})
		}
		out.Mutations = append(out.Mutations, Mutation{
			Operation: mutation.GetOperation(),
			Action:    syncAction.GetValue(),
			Index:     index,
			IndexMAC:  indexMAC,
			ValueMAC:  valueMAC,
		})
	}
	return nil
}

func (proc *Processor) DecodePatches(list *PatchList, initialState HashState, validateMACs bool) (newMutations []Mutation, currentState HashState, err error) {
	currentState = initialState
	var expectedLength int
	for _, patch := range list.Patches {
		expectedLength += len(patch.GetMutations())
	}
	newMutations = make([]Mutation, 0, expectedLength)

	for _, patch := range list.Patches {
		version := patch.GetVersion().GetVersion()
		currentState.Version = version
		err = currentState.updateHash(patch, func(indexMAC []byte, maxIndex int) ([]byte, error) {
			for i := maxIndex - 1; i >= 0; i-- {
				if bytes.Equal(patch.Mutations[i].GetRecord().GetIndex().GetBlob(), indexMAC) {
					value := patch.Mutations[i].GetRecord().GetValue().GetBlob()
					return value[len(value)-32:], nil
				}
			}
			// Previous value not found in current patch, look in the database
			return proc.Store.AppState.GetAppStateMutationMAC(string(list.Name), indexMAC)
		})
		if err != nil {
			err = fmt.Errorf("failed to update state hash: %w", err)
			return
		}

		if validateMACs {
			var keys ExpandedAppStateKeys
			keys, err = proc.getAppStateKey(patch.GetKeyId().GetId())
			if err != nil {
				err = fmt.Errorf("failed to get key %X to verify patch v%d MACs", patch.GetKeyId().GetId(), version)
				return
			}
			snapshotMAC := currentState.generateSnapshotMAC(list.Name, keys.SnapshotMAC)
			if !bytes.Equal(snapshotMAC, patch.GetSnapshotMac()) {
				err = fmt.Errorf("failed to verify patch v%d: %w", version, ErrMismatchingLTHash)
				return
			}
			patchMAC := generatePatchMAC(patch, list.Name, keys.PatchMAC)
			if !bytes.Equal(patchMAC, patch.GetPatchMac()) {
				err = fmt.Errorf("failed to verify patch v%d: %w", version, ErrMismatchingPatchMAC)
				return
			}
		}

		var out patchOutput
		out.Mutations = newMutations
		err = proc.decodePatch(patch, &out, validateMACs)
		if err != nil {
			err = fmt.Errorf("failed to decode patch v%d: %w", version, err)
			return
		}
		err = proc.Store.AppState.PutAppStateVersion(string(list.Name), currentState.Version, currentState.Hash)
		if err != nil {
			proc.Log.Errorf("Failed to update app state version in the database: %v", err)
		}
		err = proc.Store.AppState.DeleteAppStateMutationMACs(string(list.Name), out.RemovedMACs)
		if err != nil {
			proc.Log.Errorf("Failed to remove deleted mutation MACs from the database: %v", err)
		}
		err = proc.Store.AppState.PutAppStateMutationMACs(string(list.Name), version, out.AddedMACs)
		if err != nil {
			proc.Log.Errorf("Failed to insert added mutation MACs to the database: %v", err)
		}
		newMutations = out.Mutations
	}
	return
}
