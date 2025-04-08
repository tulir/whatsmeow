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
	Container:     nilStore,
}

var _ AllStores = (*NoopStore)(nil)
var _ DeviceContainer = (*NoopStore)(nil)

func (n *NoopStore) PutIdentity(address string, key [32]byte) error {
	return n.Error
}

func (n *NoopStore) DeleteAllIdentities(phone string) error {
	return n.Error
}

func (n *NoopStore) DeleteIdentity(address string) error {
	return n.Error
}

func (n *NoopStore) IsTrustedIdentity(address string, key [32]byte) (bool, error) {
	return false, n.Error
}

func (n *NoopStore) GetSession(address string) ([]byte, error) {
	return nil, n.Error
}

func (n *NoopStore) HasSession(address string) (bool, error) {
	return false, n.Error
}

func (n *NoopStore) PutSession(address string, session []byte) error {
	return n.Error
}

func (n *NoopStore) DeleteAllSessions(phone string) error {
	return n.Error
}

func (n *NoopStore) DeleteSession(address string) error {
	return n.Error
}

func (n *NoopStore) MigratePNToLID(ctx context.Context, pn, lid types.JID) error {
	return n.Error
}

func (n *NoopStore) GetOrGenPreKeys(count uint32) ([]*keys.PreKey, error) {
	return nil, n.Error
}

func (n *NoopStore) GenOnePreKey() (*keys.PreKey, error) {
	return nil, n.Error
}

func (n *NoopStore) GetPreKey(id uint32) (*keys.PreKey, error) {
	return nil, n.Error
}

func (n *NoopStore) RemovePreKey(id uint32) error {
	return n.Error
}

func (n *NoopStore) MarkPreKeysAsUploaded(upToID uint32) error {
	return n.Error
}

func (n *NoopStore) UploadedPreKeyCount() (int, error) {
	return 0, n.Error
}

func (n *NoopStore) PutSenderKey(group, user string, session []byte) error {
	return n.Error
}

func (n *NoopStore) GetSenderKey(group, user string) ([]byte, error) {
	return nil, n.Error
}

func (n *NoopStore) PutAppStateSyncKey(id []byte, key AppStateSyncKey) error {
	return n.Error
}

func (n *NoopStore) GetAppStateSyncKey(id []byte) (*AppStateSyncKey, error) {
	return nil, n.Error
}

func (n *NoopStore) GetLatestAppStateSyncKeyID() ([]byte, error) {
	return nil, n.Error
}

func (n *NoopStore) PutAppStateVersion(name string, version uint64, hash [128]byte) error {
	return n.Error
}

func (n *NoopStore) GetAppStateVersion(name string) (uint64, [128]byte, error) {
	return 0, [128]byte{}, n.Error
}

func (n *NoopStore) DeleteAppStateVersion(name string) error {
	return n.Error
}

func (n *NoopStore) PutAppStateMutationMACs(name string, version uint64, mutations []AppStateMutationMAC) error {
	return n.Error
}

func (n *NoopStore) DeleteAppStateMutationMACs(name string, indexMACs [][]byte) error {
	return n.Error
}

func (n *NoopStore) GetAppStateMutationMAC(name string, indexMAC []byte) (valueMAC []byte, err error) {
	return nil, n.Error
}

func (n *NoopStore) PutPushName(user types.JID, pushName string) (bool, string, error) {
	return false, "", n.Error
}

func (n *NoopStore) PutBusinessName(user types.JID, businessName string) (bool, string, error) {
	return false, "", n.Error
}

func (n *NoopStore) PutContactName(user types.JID, fullName, firstName string) error {
	return n.Error
}

func (n *NoopStore) PutAllContactNames(contacts []ContactEntry) error {
	return n.Error
}

func (n *NoopStore) GetContact(user types.JID) (types.ContactInfo, error) {
	return types.ContactInfo{}, n.Error
}

func (n *NoopStore) GetAllContacts() (map[types.JID]types.ContactInfo, error) {
	return nil, n.Error
}

func (n *NoopStore) PutMutedUntil(chat types.JID, mutedUntil time.Time) error {
	return n.Error
}

func (n *NoopStore) PutPinned(chat types.JID, pinned bool) error {
	return n.Error
}

func (n *NoopStore) PutArchived(chat types.JID, archived bool) error {
	return n.Error
}

func (n *NoopStore) GetChatSettings(chat types.JID) (types.LocalChatSettings, error) {
	return types.LocalChatSettings{}, n.Error
}

func (n *NoopStore) PutMessageSecrets(inserts []MessageSecretInsert) error {
	return n.Error
}

func (n *NoopStore) PutMessageSecret(chat, sender types.JID, id types.MessageID, secret []byte) error {
	return n.Error
}

func (n *NoopStore) GetMessageSecret(chat, sender types.JID, id types.MessageID) ([]byte, error) {
	return nil, n.Error
}

func (n *NoopStore) PutPrivacyTokens(tokens ...PrivacyToken) error {
	return n.Error
}

func (n *NoopStore) GetPrivacyToken(user types.JID) (*PrivacyToken, error) {
	return nil, n.Error
}

func (n *NoopStore) PutDevice(store *Device) error {
	return n.Error
}

func (n *NoopStore) DeleteDevice(store *Device) error {
	return n.Error
}

func (n *NoopStore) GetLIDForPN(ctx context.Context, pn types.JID) (types.JID, error) {
	return types.JID{}, n.Error
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
