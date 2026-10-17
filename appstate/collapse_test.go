// Copyright (c) 2026 Tulir Asokan
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
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/appstate/lthash"
	"go.mau.fi/whatsmeow/proto/waServerSync"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/util/cbcutil"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// memAppState is an in-memory app state store that behaves like the SQL one:
// several versions of the same index MAC may coexist, the newest wins.
type memAppState struct {
	keys    map[string]*store.AppStateSyncKey
	version map[string]uint64
	hash    map[string][128]byte
	macs    map[string][]memMAC
}

type memMAC struct {
	version uint64
	mac     store.AppStateMutationMAC
}

func newMemAppState() *memAppState {
	return &memAppState{
		keys:    make(map[string]*store.AppStateSyncKey),
		version: make(map[string]uint64),
		hash:    make(map[string][128]byte),
		macs:    make(map[string][]memMAC),
	}
}

func (m *memAppState) PutAppStateSyncKey(ctx context.Context, id []byte, key store.AppStateSyncKey) error {
	m.keys[string(id)] = &key
	return nil
}
func (m *memAppState) GetAppStateSyncKey(ctx context.Context, id []byte) (*store.AppStateSyncKey, error) {
	return m.keys[string(id)], nil
}
func (m *memAppState) GetLatestAppStateSyncKeyID(ctx context.Context) ([]byte, error) {
	return nil, nil
}
func (m *memAppState) GetAllAppStateSyncKeys(ctx context.Context) ([]*store.AppStateSyncKey, error) {
	return nil, nil
}

func (m *memAppState) PutAppStateVersion(ctx context.Context, name string, version uint64, hash [128]byte) error {
	m.version[name] = version
	m.hash[name] = hash
	return nil
}
func (m *memAppState) GetAppStateVersion(ctx context.Context, name string) (uint64, [128]byte, error) {
	return m.version[name], m.hash[name], nil
}
func (m *memAppState) DeleteAppStateVersion(ctx context.Context, name string) error {
	delete(m.version, name)
	delete(m.hash, name)
	delete(m.macs, name)
	return nil
}
func (m *memAppState) PutAppStateMutationMACs(ctx context.Context, name string, version uint64, mutations []store.AppStateMutationMAC) error {
	for _, mut := range mutations {
		m.macs[name] = append(m.macs[name], memMAC{version: version, mac: mut})
	}
	return nil
}
func (m *memAppState) DeleteAppStateMutationMACs(ctx context.Context, name string, indexMACs [][]byte) error {
	kept := m.macs[name][:0]
	for _, entry := range m.macs[name] {
		drop := false
		for _, mac := range indexMACs {
			if bytes.Equal(mac, entry.mac.IndexMAC) {
				drop = true
			}
		}
		if !drop {
			kept = append(kept, entry)
		}
	}
	m.macs[name] = kept
	return nil
}
func (m *memAppState) GetAppStateMutationMAC(ctx context.Context, name string, indexMAC []byte) ([]byte, error) {
	var best *memMAC
	for i := range m.macs[name] {
		entry := &m.macs[name][i]
		if bytes.Equal(entry.mac.IndexMAC, indexMAC) && (best == nil || entry.version > best.version) {
			best = entry
		}
	}
	if best == nil {
		return nil, nil
	}
	return best.mac.ValueMAC, nil
}
func (m *memAppState) GetAppStateMutationMACsByIndexHash(ctx context.Context, name string, indexHash []byte) ([]store.AppStateMutationMAC, error) {
	latest := make(map[string]memMAC)
	for _, entry := range m.macs[name] {
		if !bytes.Equal(entry.mac.IndexHash, indexHash) {
			continue
		}
		if cur, ok := latest[string(entry.mac.IndexMAC)]; !ok || entry.version > cur.version {
			latest[string(entry.mac.IndexMAC)] = entry
		}
	}
	var out []store.AppStateMutationMAC
	for _, entry := range latest {
		out = append(out, entry.mac)
	}
	return out, nil
}

func (m *memAppState) hasValue(name string, valueMAC []byte) bool {
	for _, entry := range m.macs[name] {
		if bytes.Equal(entry.mac.ValueMAC, valueMAC) {
			return true
		}
	}
	return false
}

func (m *memAppState) has(name string, indexMAC []byte) bool {
	for _, entry := range m.macs[name] {
		if bytes.Equal(entry.mac.IndexMAC, indexMAC) {
			return true
		}
	}
	return false
}

type testKey struct {
	id   []byte
	keys ExpandedAppStateKeys
}

func newTestProcessor(t *testing.T) (*Processor, *memAppState) {
	t.Helper()
	mem := newMemAppState()
	proc := NewProcessor(&store.Device{AppStateKeys: mem, AppState: mem}, waLog.Noop)
	return proc, mem
}

func addTestKey(t *testing.T, mem *memAppState, seed byte) testKey {
	t.Helper()
	data := bytes.Repeat([]byte{seed}, 32)
	id := []byte{seed, 0, 0, 0, 0, 1}
	mem.keys[string(id)] = &store.AppStateSyncKey{Data: data}
	return testKey{id: id, keys: expandAppStateKeys(data)}
}

// encryptSet builds a SET mutation the way EncodePatch does, under the given key.
func encryptSet(t *testing.T, key testKey, index []string, value *waSyncAction.SyncActionValue) *waServerSync.SyncdMutation {
	t.Helper()
	indexBytes, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	content, err := proto.Marshal(&waSyncAction.SyncActionData{
		Index:   indexBytes,
		Value:   value,
		Padding: []byte{},
		Version: proto.Int32(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := cbcutil.Encrypt(key.keys.ValueEncryption, nil, content)
	if err != nil {
		t.Fatal(err)
	}
	valueMAC := generateContentMAC(waServerSync.SyncdMutation_SET, encrypted, key.id, key.keys.ValueMAC)
	return &waServerSync.SyncdMutation{
		Operation: waServerSync.SyncdMutation_SET.Enum(),
		Record: &waServerSync.SyncdRecord{
			Index: &waServerSync.SyncdIndex{Blob: concatAndHMAC(sha256.New, key.keys.Index, indexBytes)},
			Value: &waServerSync.SyncdValue{Blob: append(encrypted, valueMAC...)},
			KeyID: &waServerSync.KeyId{ID: key.id},
		},
	}
}

func valueMACOf(m *waServerSync.SyncdMutation) []byte {
	blob := m.GetRecord().GetValue().GetBlob()
	return blob[len(blob)-32:]
}

func indexMACOf(m *waServerSync.SyncdMutation) []byte {
	return m.GetRecord().GetIndex().GetBlob()
}

func testIndexHash(t *testing.T, index []string) []byte {
	t.Helper()
	indexBytes, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	return indexHash(indexBytes)
}

// phoneHash is the primary device's LTHash: one value MAC per logical index.
func phoneHash(add ...[]byte) (hash [128]byte) {
	lthash.WAPatchIntegrity.SubtractThenAddInPlace(hash[:], nil, add)
	return
}

func markRead(ts int64) *waSyncAction.SyncActionValue {
	return &waSyncAction.SyncActionValue{MarkChatAsReadAction: &waSyncAction.MarkChatAsReadAction{Read: proto.Bool(true)}, Timestamp: proto.Int64(ts)}
}

func archive(on bool) *waSyncAction.SyncActionValue {
	return &waSyncAction.SyncActionValue{ArchiveChatAction: &waSyncAction.ArchiveChatAction{Archived: proto.Bool(on)}}
}

var (
	indexL = []string{IndexMarkChatAsRead, "120363000000000000@g.us"}
	indexM = []string{IndexArchive, "5511999999999@s.whatsapp.net"}
)

func snapshotOf(version uint64, hash [128]byte, key testKey, records ...*waServerSync.SyncdMutation) *waServerSync.SyncdSnapshot {
	ss := &waServerSync.SyncdSnapshot{
		Version: &waServerSync.SyncdVersion{Version: proto.Uint64(version)},
		KeyID:   &waServerSync.KeyId{ID: key.id},
	}
	for _, record := range records {
		ss.Records = append(ss.Records, record.GetRecord())
	}
	state := HashState{Version: version, Hash: hash}
	ss.Mac = state.generateSnapshotMAC(WAPatchRegularLow, key.keys.SnapshotMAC)
	return ss
}

func patchOf(version uint64, hash [128]byte, key testKey, mutations ...*waServerSync.SyncdMutation) *waServerSync.SyncdPatch {
	state := HashState{Version: version, Hash: hash}
	patch := &waServerSync.SyncdPatch{
		Version:     &waServerSync.SyncdVersion{Version: proto.Uint64(version)},
		KeyID:       &waServerSync.KeyId{ID: key.id},
		Mutations:   mutations,
		SnapshotMAC: state.generateSnapshotMAC(WAPatchRegularLow, key.keys.SnapshotMAC),
	}
	patch.PatchMAC = generatePatchMAC(patch, WAPatchRegularLow, key.keys.PatchMAC, version)
	return patch
}

// The server keeps snapshot records by index MAC, so after a key rotation the
// same logical index appears twice. The primary signs over one value per
// index; a companion summing every record never matches.
func TestDecodeSnapshotCollapsesSupersededRecords(t *testing.T) {
	proc, mem := newTestProcessor(t)
	keyA, keyB := addTestKey(t, mem, 1), addTestKey(t, mem, 2)
	oldL := encryptSet(t, keyA, indexL, markRead(1))
	m := encryptSet(t, keyA, indexM, archive(true))
	newL := encryptSet(t, keyB, indexL, markRead(2))

	expected := phoneHash(valueMACOf(m), valueMACOf(newL))
	ss := snapshotOf(320, expected, keyB, oldL, m, newL)

	mutations, state, err := proc.DecodePatches(context.Background(), &PatchList{Name: WAPatchRegularLow, Snapshot: ss}, HashState{}, true)
	if err != nil {
		t.Fatalf("DecodePatches: %v", err)
	}
	if state.Version != 320 || state.Hash != expected {
		t.Fatalf("state after snapshot: version %d, hash matches phone: %v", state.Version, state.Hash == expected)
	}
	if len(mutations) != 3 {
		t.Fatalf("expected the 3 records to be decoded, got %d", len(mutations))
	}
	if mem.has(string(WAPatchRegularLow), indexMACOf(oldL)) {
		t.Error("superseded record was kept in the store")
	}
	if !mem.has(string(WAPatchRegularLow), indexMACOf(newL)) || !mem.has(string(WAPatchRegularLow), indexMACOf(m)) {
		t.Error("current records are missing from the store")
	}
	stored, err := mem.GetAppStateMutationMACsByIndexHash(context.Background(), string(WAPatchRegularLow), testIndexHash(t, indexL))
	if err != nil || len(stored) != 1 || !bytes.Equal(stored[0].IndexMAC, indexMACOf(newL)) {
		t.Errorf("records by index hash: %v, %v", stored, err)
	}
}

// A primary that adopted a companion's state may sign over every record. That
// snapshot must still verify, with the superseded records kept.
func TestDecodeSnapshotAcceptsUncollapsedHash(t *testing.T) {
	proc, mem := newTestProcessor(t)
	keyA, keyB := addTestKey(t, mem, 1), addTestKey(t, mem, 2)
	oldL := encryptSet(t, keyA, indexL, markRead(1))
	newL := encryptSet(t, keyB, indexL, markRead(2))

	expected := phoneHash(valueMACOf(oldL), valueMACOf(newL))
	ss := snapshotOf(7, expected, keyB, oldL, newL)

	_, state, err := proc.DecodePatches(context.Background(), &PatchList{Name: WAPatchRegularLow, Snapshot: ss}, HashState{}, true)
	if err != nil {
		t.Fatalf("DecodePatches: %v", err)
	}
	if state.Hash != expected {
		t.Fatal("state hash does not match the uncollapsed snapshot MAC")
	}
	if !mem.has(string(WAPatchRegularLow), indexMACOf(oldL)) {
		t.Error("record counted by the snapshot MAC was dropped from the store")
	}
}

func TestDecodeSnapshotStillRejectsWrongMAC(t *testing.T) {
	proc, mem := newTestProcessor(t)
	keyA, keyB := addTestKey(t, mem, 1), addTestKey(t, mem, 2)
	oldL := encryptSet(t, keyA, indexL, markRead(1))
	newL := encryptSet(t, keyB, indexL, markRead(2))

	ss := snapshotOf(7, phoneHash(valueMACOf(oldL)), keyB, oldL, newL)
	_, _, err := proc.DecodePatches(context.Background(), &PatchList{Name: WAPatchRegularLow, Snapshot: ss}, HashState{}, true)
	if !errors.Is(err, ErrMismatchingLTHash) {
		t.Fatalf("expected ErrMismatchingLTHash, got %v", err)
	}
}

func seedState(t *testing.T, mem *memAppState, version uint64, hash [128]byte, withHash bool, records ...*waServerSync.SyncdMutation) {
	t.Helper()
	name := string(WAPatchRegularLow)
	mem.version[name] = version
	mem.hash[name] = hash
	for _, record := range records {
		mac := store.AppStateMutationMAC{IndexMAC: indexMACOf(record), ValueMAC: valueMACOf(record)}
		if withHash {
			proc := NewProcessor(&store.Device{AppStateKeys: mem, AppState: mem}, waLog.Noop)
			_, _, _, action, _, err := proc.decodeMutation(context.Background(), record, 0, false)
			if err != nil {
				t.Fatal(err)
			}
			mac.IndexHash = indexHash(action.GetIndex())
		}
		mem.macs[name] = append(mem.macs[name], memMAC{version: version, mac: mac})
	}
}

// The primary subtracts every record it holds for the index being SET, even
// one written under an earlier key.
func TestDecodePatchSupersedesRecordUnderRotatedKey(t *testing.T) {
	proc, mem := newTestProcessor(t)
	keyA, keyB := addTestKey(t, mem, 1), addTestKey(t, mem, 2)
	oldL := encryptSet(t, keyA, indexL, markRead(1))
	m := encryptSet(t, keyA, indexM, archive(true))
	before := phoneHash(valueMACOf(oldL), valueMACOf(m))
	seedState(t, mem, 323, before, true, oldL, m)

	newL := encryptSet(t, keyB, indexL, markRead(2))
	after := phoneHash(valueMACOf(m), valueMACOf(newL))
	patch := patchOf(324, after, keyB, newL)

	_, state, err := proc.DecodePatches(context.Background(), &PatchList{Name: WAPatchRegularLow, Patches: []*waServerSync.SyncdPatch{patch}}, HashState{Version: 323, Hash: before}, true)
	if err != nil {
		t.Fatalf("DecodePatches: %v", err)
	}
	if state.Hash != after {
		t.Fatal("state hash does not follow the primary's rule")
	}
	if mem.has(string(WAPatchRegularLow), indexMACOf(oldL)) {
		t.Error("superseded record was kept in the store")
	}
	if !mem.has(string(WAPatchRegularLow), indexMACOf(newL)) {
		t.Error("new record missing from the store")
	}
}

// Rows stored before index hashes existed are still found by index MAC.
func TestDecodePatchSupersedesLegacyRowByIndexMAC(t *testing.T) {
	proc, mem := newTestProcessor(t)
	keyA := addTestKey(t, mem, 1)
	oldL := encryptSet(t, keyA, indexL, markRead(1))
	before := phoneHash(valueMACOf(oldL))
	seedState(t, mem, 10, before, false, oldL)

	newL := encryptSet(t, keyA, indexL, markRead(2))
	after := phoneHash(valueMACOf(newL))
	patch := patchOf(11, after, keyA, newL)

	_, state, err := proc.DecodePatches(context.Background(), &PatchList{Name: WAPatchRegularLow, Patches: []*waServerSync.SyncdPatch{patch}}, HashState{Version: 10, Hash: before}, true)
	if err != nil {
		t.Fatalf("DecodePatches: %v", err)
	}
	if state.Hash != after {
		t.Fatal("legacy row was not subtracted")
	}
}

// Two SETs of one index inside a patch, under different keys: the second
// supersedes the first, which already superseded the stored one.
func TestDecodePatchCollapsesWithinPatch(t *testing.T) {
	proc, mem := newTestProcessor(t)
	keyA, keyB := addTestKey(t, mem, 1), addTestKey(t, mem, 2)
	storedL := encryptSet(t, keyA, indexL, markRead(1))
	before := phoneHash(valueMACOf(storedL))
	seedState(t, mem, 1, before, true, storedL)

	midL := encryptSet(t, keyA, indexL, markRead(2))
	lastL := encryptSet(t, keyB, indexL, markRead(3))
	after := phoneHash(valueMACOf(lastL))
	patch := patchOf(2, after, keyB, midL, lastL)

	_, state, err := proc.DecodePatches(context.Background(), &PatchList{Name: WAPatchRegularLow, Patches: []*waServerSync.SyncdPatch{patch}}, HashState{Version: 1, Hash: before}, true)
	if err != nil {
		t.Fatalf("DecodePatches: %v", err)
	}
	if state.Hash != after {
		t.Fatal("in-patch supersession not applied")
	}
	if mem.has(string(WAPatchRegularLow), indexMACOf(storedL)) || mem.has(string(WAPatchRegularLow), indexMACOf(midL)) {
		t.Error("superseded records kept in the store")
	}
}

// Our own patches must use the primary's rule too, or the primary rejects them.
func TestEncodePatchSubtractsRecordUnderRotatedKey(t *testing.T) {
	proc, mem := newTestProcessor(t)
	keyA, keyB := addTestKey(t, mem, 1), addTestKey(t, mem, 2)
	oldL := encryptSet(t, keyA, indexL, markRead(1))
	before := phoneHash(valueMACOf(oldL))
	seedState(t, mem, 5, before, true, oldL)

	raw, err := proc.EncodePatch(context.Background(), keyB.id, HashState{Version: 5, Hash: before}, PatchInfo{
		Type:      WAPatchRegularLow,
		Mutations: []MutationInfo{{Index: indexL, Version: 3, Value: markRead(2)}},
	})
	if err != nil {
		t.Fatalf("EncodePatch: %v", err)
	}
	var patch waServerSync.SyncdPatch
	if err = proto.Unmarshal(raw, &patch); err != nil {
		t.Fatal(err)
	}
	after := phoneHash(valueMACOf(patch.GetMutations()[0]))
	want := (&HashState{Version: 6, Hash: after}).generateSnapshotMAC(WAPatchRegularLow, keyB.keys.SnapshotMAC)
	if !bytes.Equal(patch.GetSnapshotMAC(), want) {
		t.Fatal("snapshot MAC of the encoded patch keeps the record written under the old key")
	}
}

// A companion on an older version of this library writes a SET that subtracts
// nothing when the index lives under a rotated key. That patch is in the
// server's history: it has to verify, and the superseded record stays until
// the primary's next SET of the index subtracts every record it holds.
func TestDecodePatchAcceptsPatchMadeWithoutCollapsing(t *testing.T) {
	proc, mem := newTestProcessor(t)
	keyA, keyB := addTestKey(t, mem, 1), addTestKey(t, mem, 2)
	oldL := encryptSet(t, keyA, indexL, markRead(1))
	before := phoneHash(valueMACOf(oldL))
	seedState(t, mem, 322, before, true, oldL)

	companionL := encryptSet(t, keyB, indexL, markRead(2))
	afterCompanion := phoneHash(valueMACOf(oldL), valueMACOf(companionL))
	companionPatch := patchOf(323, afterCompanion, keyB, companionL)

	primaryL := encryptSet(t, keyB, indexL, markRead(3))
	afterPrimary := phoneHash(valueMACOf(primaryL))
	primaryPatch := patchOf(324, afterPrimary, keyB, primaryL)

	list := &PatchList{Name: WAPatchRegularLow, Patches: []*waServerSync.SyncdPatch{companionPatch, primaryPatch}}
	_, state, err := proc.DecodePatches(context.Background(), list, HashState{Version: 322, Hash: before}, true)
	if err != nil {
		t.Fatalf("DecodePatches: %v", err)
	}
	if state.Version != 324 || state.Hash != afterPrimary {
		t.Fatal("state does not follow the primary after the companion's patch")
	}
	// companionL and primaryL share a key, hence an index MAC: compare values.
	if mem.hasValue(string(WAPatchRegularLow), valueMACOf(oldL)) || mem.hasValue(string(WAPatchRegularLow), valueMACOf(companionL)) {
		t.Error("superseded records kept after the primary's SET")
	}
	if !mem.hasValue(string(WAPatchRegularLow), valueMACOf(primaryL)) {
		t.Error("primary's record missing")
	}
}

func TestDecodePatchKeepsSupersededRecordAfterLegacyPatch(t *testing.T) {
	proc, mem := newTestProcessor(t)
	keyA, keyB := addTestKey(t, mem, 1), addTestKey(t, mem, 2)
	oldL := encryptSet(t, keyA, indexL, markRead(1))
	before := phoneHash(valueMACOf(oldL))
	seedState(t, mem, 1, before, true, oldL)

	companionL := encryptSet(t, keyB, indexL, markRead(2))
	patch := patchOf(2, phoneHash(valueMACOf(oldL), valueMACOf(companionL)), keyB, companionL)
	_, _, err := proc.DecodePatches(context.Background(), &PatchList{Name: WAPatchRegularLow, Patches: []*waServerSync.SyncdPatch{patch}}, HashState{Version: 1, Hash: before}, true)
	if err != nil {
		t.Fatalf("DecodePatches: %v", err)
	}
	if !mem.has(string(WAPatchRegularLow), indexMACOf(oldL)) {
		t.Error("record still counted by the hash was dropped from the store")
	}
}
