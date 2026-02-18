// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"context"
	"errors"
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/util/keys"
)

type NoopStore struct {
	Error error
}

var nilStore = &NoopStore{Error: errors.New("store is nil")}
var nilKey = &keys.KeyPair{Priv: &[32]byte{}, Pub: &[32]byte{}}
var NoopDevice = &Device{
	ID:          &types.EmptyJID,
	NoiseKey:    nilKey,
	IdentityKey: nilKey,

	Identities:    nilStore,
	Sessions:      nilStore,
	PreKeys:       nilStore,
	SenderKeys:    nilStore,
	AppStateKeys:  nilStore,
	AppState:      nilStore,
	Contacts:      nilStore,
	ChatSettings:  nilStore,
	MsgSecrets:    nilStore,
	PrivacyTokens: nilStore,
	EventBuffer:   nilStore,
	LIDs:          nilStore,
	Container:     nilStore,
}

var _ AllStores = (*NoopStore)(nil)
var _ DeviceContainer = (*NoopStore)(nil)

func (n *NoopStore) PutIdentity(ctx context.Context, address string, key [32]byte) error {
	return n.Error
}

func (n *NoopStore) DeleteAllIdentities(ctx context.Context, phone string) error {
	return n.Error
}

func (n *NoopStore) DeleteIdentity(ctx context.Context, address string) error {
	return n.Error
}

func (n *NoopStore) IsTrustedIdentity(ctx context.Context, address string, key [32]byte) (bool, error) {
	return false, n.Error
}

func (n *NoopStore) GetSession(ctx context.Context, address string) ([]byte, error) {
	return nil, n.Error
}

func (n *NoopStore) HasSession(ctx context.Context, address string) (bool, error) {
	return false, n.Error
}

func (n *NoopStore) GetManySessions(ctx context.Context, addresses []string) (map[string][]byte, error) {
	return nil, n.Error
}

func (n *NoopStore) PutSession(ctx context.Context, address string, session []byte) error {
	return n.Error
}

func (n *NoopStore) PutManySessions(ctx context.Context, sessions map[string][]byte) error {
	return n.Error
}

func (n *NoopStore) DeleteAllSessions(ctx context.Context, phone string) error {
	return n.Error
}

func (n *NoopStore) DeleteSession(ctx context.Context, address string) error {
	return n.Error
}

func (n *NoopStore) MigratePNToLID(ctx context.Context, pn, lid types.JID) error {
	return n.Error
}

func (n *NoopStore) GetOrGenPreKeys(ctx context.Context, count uint32) ([]*keys.PreKey, error) {
	return nil, n.Error
}

func (n *NoopStore) GenOnePreKey(ctx context.Context) (*keys.PreKey, error) {
	return nil, n.Error
}

func (n *NoopStore) GetPreKey(ctx context.Context, id uint32) (*keys.PreKey, error) {
	return nil, n.Error
}

func (n *NoopStore) RemovePreKey(ctx context.Context, id uint32) error {
	return n.Error
}

func (n *NoopStore) MarkPreKeysAsUploaded(ctx context.Context, upToID uint32) error {
	return n.Error
}

func (n *NoopStore) UploadedPreKeyCount(ctx context.Context) (int, error) {
	return 0, n.Error
}

func (n *NoopStore) PutSenderKey(ctx context.Context, group, user string, session []byte) error {
	return n.Error
}

func (n *NoopStore) GetSenderKey(ctx context.Context, group, user string) ([]byte, error) {
	return nil, n.Error
}

func (n *NoopStore) PutAppStateSyncKey(ctx context.Context, id []byte, key AppStateSyncKey) error {
	return n.Error
}

func (n *NoopStore) GetAppStateSyncKey(ctx context.Context, id []byte) (*AppStateSyncKey, error) {
	return nil, n.Error
}

func (n *NoopStore) GetLatestAppStateSyncKeyID(ctx context.Context) ([]byte, error) {
	return nil, n.Error
}

func (n *NoopStore) GetAllAppStateSyncKeys(ctx context.Context) ([]*AppStateSyncKey, error) {
	return nil, nil
}

func (n *NoopStore) PutAppStateVersion(ctx context.Context, name string, version uint64, hash [128]byte) error {
	return n.Error
}

func (n *NoopStore) GetAppStateVersion(ctx context.Context, name string) (uint64, [128]byte, error) {
	return 0, [128]byte{}, n.Error
}

func (n *NoopStore) DeleteAppStateVersion(ctx context.Context, name string) error {
	return n.Error
}

func (n *NoopStore) PutAppStateMutationMACs(ctx context.Context, name string, version uint64, mutations []AppStateMutationMAC) error {
	return n.Error
}

func (n *NoopStore) DeleteAppStateMutationMACs(ctx context.Context, name string, indexMACs [][]byte) error {
	return n.Error
}

func (n *NoopStore) GetAppStateMutationMAC(ctx context.Context, name string, indexMAC []byte) (valueMAC []byte, err error) {
	return nil, n.Error
}

func (n *NoopStore) PutPushName(ctx context.Context, user types.JID, pushName string) (bool, string, error) {
	return false, "", n.Error
}

func (n *NoopStore) PutBusinessName(ctx context.Context, user types.JID, businessName string) (bool, string, error) {
	return false, "", n.Error
}

func (n *NoopStore) PutContactName(ctx context.Context, user types.JID, fullName, firstName string) error {
	return n.Error
}

func (n *NoopStore) PutAllContactNames(ctx context.Context, contacts []ContactEntry) error {
	return n.Error
}

func (n *NoopStore) PutManyRedactedPhones(ctx context.Context, entries []RedactedPhoneEntry) error {
	return n.Error
}

func (n *NoopStore) GetContact(ctx context.Context, user types.JID) (types.ContactInfo, error) {
	return types.ContactInfo{}, n.Error
}

func (n *NoopStore) GetAllContacts(ctx context.Context) (map[types.JID]types.ContactInfo, error) {
	return nil, n.Error
}

func (n *NoopStore) PutMutedUntil(ctx context.Context, chat types.JID, mutedUntil time.Time) error {
	return n.Error
}

func (n *NoopStore) PutPinned(ctx context.Context, chat types.JID, pinned bool) error {
	return n.Error
}

func (n *NoopStore) PutArchived(ctx context.Context, chat types.JID, archived bool) error {
	return n.Error
}

func (n *NoopStore) GetChatSettings(ctx context.Context, chat types.JID) (types.LocalChatSettings, error) {
	return types.LocalChatSettings{}, n.Error
}

func (n *NoopStore) PutMessageSecrets(ctx context.Context, inserts []MessageSecretInsert) error {
	return n.Error
}

func (n *NoopStore) PutMessageSecret(ctx context.Context, chat, sender types.JID, id types.MessageID, secret []byte) error {
	return n.Error
}

func (n *NoopStore) GetMessageSecret(ctx context.Context, chat, sender types.JID, id types.MessageID) ([]byte, types.JID, error) {
	return nil, types.EmptyJID, n.Error
}

func (n *NoopStore) PutPrivacyTokens(ctx context.Context, tokens ...PrivacyToken) error {
	return n.Error
}

func (n *NoopStore) GetPrivacyToken(ctx context.Context, user types.JID) (*PrivacyToken, error) {
	return nil, n.Error
}

func (n *NoopStore) PutDevice(ctx context.Context, store *Device) error {
	return n.Error
}

func (n *NoopStore) DeleteDevice(ctx context.Context, store *Device) error {
	return n.Error
}

func (n *NoopStore) GetBufferedEvent(ctx context.Context, ciphertextHash [32]byte) (*BufferedEvent, error) {
	return nil, nil
}

func (n *NoopStore) PutBufferedEvent(ctx context.Context, ciphertextHash [32]byte, plaintext []byte, serverTimestamp time.Time) error {
	return nil
}

func (n *NoopStore) DoDecryptionTxn(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (n *NoopStore) ClearBufferedEventPlaintext(ctx context.Context, ciphertextHash [32]byte) error {
	return nil
}

func (n *NoopStore) DeleteOldBufferedHashes(ctx context.Context) error {
	return nil
}

func (n *NoopStore) GetLIDForPN(ctx context.Context, pn types.JID) (types.JID, error) {
	return types.JID{}, n.Error
}

func (n *NoopStore) GetManyLIDsForPNs(ctx context.Context, pns []types.JID) (map[types.JID]types.JID, error) {
	return nil, n.Error
}

func (n *NoopStore) GetPNForLID(ctx context.Context, lid types.JID) (types.JID, error) {
	return types.JID{}, n.Error
}

func (n *NoopStore) PutManyLIDMappings(ctx context.Context, mappings []LIDMapping) error {
	return n.Error
}

func (n *NoopStore) PutLIDMapping(ctx context.Context, lid types.JID, jid types.JID) error {
	return n.Error
}

func (n *NoopStore) DeleteOldOutgoingEvents(ctx context.Context) error {
	return nil
}

func (n *NoopStore) GetOutgoingEvent(ctx context.Context, chatJID, altChatJID types.JID, id types.MessageID) (string, []byte, error) {
	return "", nil, nil
}

func (n *NoopStore) AddOutgoingEvent(ctx context.Context, chatJID types.JID, id types.MessageID, format string, plaintext []byte) error {
	return nil
}
