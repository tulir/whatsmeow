// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package sqlstore contains an SQL-backed implementation of the interfaces in the store package.
package sqlstore

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/util/keys"
)

// ErrInvalidLength is returned by some database getters if the database returned a byte array with an unexpected length.
// This should be impossible, as the database schema contains CHECK()s for all the relevant columns.
var ErrInvalidLength = errors.New("database returned byte array with illegal length")

// PostgresArrayWrapper is a function to wrap array values before passing them to the sql package.
//
// When using github.com/lib/pq, you should set
//
//	whatsmeow.PostgresArrayWrapper = pq.Array
var PostgresArrayWrapper func(interface{}) interface {
	driver.Valuer
	sql.Scanner
}

type SQLStore struct {
	*Container
	JID string

	preKeyLock sync.Mutex

	contactCache     map[types.JID]*types.ContactInfo
	contactCacheLock sync.Mutex
}

// NewSQLStore creates a new SQLStore with the given database container and user JID.
// It contains implementations of all the different stores in the store package.
//
// In general, you should use Container.NewDevice or Container.GetDevice instead of this.
func NewSQLStore(c *Container, jid types.JID) *SQLStore {
	return &SQLStore{
		Container:    c,
		JID:          jid.String(),
		contactCache: make(map[types.JID]*types.ContactInfo),
	}
}

var _ store.IdentityStore = (*SQLStore)(nil)
var _ store.SessionStore = (*SQLStore)(nil)
var _ store.PreKeyStore = (*SQLStore)(nil)
var _ store.SenderKeyStore = (*SQLStore)(nil)
var _ store.AppStateSyncKeyStore = (*SQLStore)(nil)
var _ store.AppStateStore = (*SQLStore)(nil)
var _ store.ContactStore = (*SQLStore)(nil)

const ()

func (s *SQLStore) PutIdentity(address string, key [32]byte) error {
	_, err := s.db.Exec(s.query.PutIdentity(), s.JID, address, key[:])
	return err
}

func (s *SQLStore) DeleteAllIdentities(phone string) error {
	_, err := s.db.Exec(s.query.DeleteAllIdentities(), s.JID, phone+":%")
	return err
}

func (s *SQLStore) DeleteIdentity(address string) error {
	_, err := s.db.Exec(s.query.DeleteAllIdentities(), s.JID, address)
	return err
}

func (s *SQLStore) IsTrustedIdentity(address string, key [32]byte) (bool, error) {
	var existingIdentity []byte
	err := s.db.QueryRow(s.query.GetIdentity(), s.JID, address).Scan(&existingIdentity)
	if errors.Is(err, sql.ErrNoRows) {
		// Trust if not known, it'll be saved automatically later
		return true, nil
	} else if err != nil {
		return false, err
	} else if len(existingIdentity) != 32 {
		return false, ErrInvalidLength
	}
	return *(*[32]byte)(existingIdentity) == key, nil
}

func (s *SQLStore) GetSession(address string) (session []byte, err error) {
	err = s.db.QueryRow(s.query.GetSession(), s.JID, address).Scan(&session)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return
}

func (s *SQLStore) HasSession(address string) (has bool, err error) {
	err = s.db.QueryRow(s.query.HasSession(), s.JID, address).Scan(&has)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return
}

func (s *SQLStore) PutSession(address string, session []byte) error {
	_, err := s.db.Exec(s.query.PutSession(), s.JID, address, session)
	return err
}

func (s *SQLStore) DeleteAllSessions(phone string) error {
	_, err := s.db.Exec(s.query.DeleteAllSessions(), s.JID, phone+":%")
	return err
}

func (s *SQLStore) DeleteSession(address string) error {
	_, err := s.db.Exec(s.query.DeleteSession(), s.JID, address)
	return err
}

func (s *SQLStore) genOnePreKey(id uint32, markUploaded bool) (*keys.PreKey, error) {
	key := keys.NewPreKey(id)
	_, err := s.db.Exec(s.query.InsertPreKey(), s.JID, key.KeyID, key.Priv[:], markUploaded)
	return key, err
}

func (s *SQLStore) getNextPreKeyID() (uint32, error) {
	var lastKeyID sql.NullInt32
	err := s.db.QueryRow(s.query.GetLastPreKeyID(), s.JID).Scan(&lastKeyID)
	if err != nil {
		return 0, fmt.Errorf("failed to query next prekey ID: %w", err)
	}
	return uint32(lastKeyID.Int32) + 1, nil
}

func (s *SQLStore) GenOnePreKey() (*keys.PreKey, error) {
	s.preKeyLock.Lock()
	defer s.preKeyLock.Unlock()
	nextKeyID, err := s.getNextPreKeyID()
	if err != nil {
		return nil, err
	}
	return s.genOnePreKey(nextKeyID, true)
}

func (s *SQLStore) GetOrGenPreKeys(count uint32) ([]*keys.PreKey, error) {
	s.preKeyLock.Lock()
	defer s.preKeyLock.Unlock()

	res, err := s.db.Query(s.query.GetUnUploadedPreKeys(), s.JID, count)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing prekeys: %w", err)
	}
	newKeys := make([]*keys.PreKey, count)
	var existingCount uint32
	for res.Next() {
		var key *keys.PreKey
		key, err = scanPreKey(res)
		if err != nil {
			return nil, err
		} else if key != nil {
			newKeys[existingCount] = key
			existingCount++
		}
	}

	if existingCount < uint32(len(newKeys)) {
		var nextKeyID uint32
		nextKeyID, err = s.getNextPreKeyID()
		if err != nil {
			return nil, err
		}
		for i := existingCount; i < count; i++ {
			newKeys[i], err = s.genOnePreKey(nextKeyID, false)
			if err != nil {
				return nil, fmt.Errorf("failed to generate prekey: %w", err)
			}
			nextKeyID++
		}
	}

	return newKeys, nil
}

func scanPreKey(row scannable) (*keys.PreKey, error) {
	var priv []byte
	var id uint32
	err := row.Scan(&id, &priv)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else if len(priv) != 32 {
		return nil, ErrInvalidLength
	}
	return &keys.PreKey{
		KeyPair: *keys.NewKeyPairFromPrivateKey(*(*[32]byte)(priv)),
		KeyID:   id,
	}, nil
}

func (s *SQLStore) GetPreKey(id uint32) (*keys.PreKey, error) {
	return scanPreKey(s.db.QueryRow(s.query.GetPreKey(), s.JID, id))
}

func (s *SQLStore) RemovePreKey(id uint32) error {
	_, err := s.db.Exec(s.query.DeletePreKey(), s.JID, id)
	return err
}

func (s *SQLStore) MarkPreKeysAsUploaded(upToID uint32) error {
	_, err := s.db.Exec(s.query.MarkPreKeysAsUploaded(), s.JID, upToID)
	return err
}

func (s *SQLStore) UploadedPreKeyCount() (count int, err error) {
	err = s.db.QueryRow(s.query.GetUploadedPreKeyCount(), s.JID).Scan(&count)
	return
}

func (s *SQLStore) PutSenderKey(group, user string, session []byte) error {
	_, err := s.db.Exec(s.query.PutSenderKey(), s.JID, group, user, session)
	return err
}

func (s *SQLStore) GetSenderKey(group, user string) (key []byte, err error) {
	err = s.db.QueryRow(s.query.GetSenderKey(), s.JID, group, user).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return
}

func (s *SQLStore) PutAppStateSyncKey(id []byte, key store.AppStateSyncKey) error {
	_, err := s.db.Exec(s.query.PutAppStateSyncKey(), s.JID, id, key.Data, key.Timestamp, key.Fingerprint)
	return err
}

func (s *SQLStore) GetAppStateSyncKey(id []byte) (*store.AppStateSyncKey, error) {
	var key store.AppStateSyncKey
	err := s.db.QueryRow(s.query.GetAppStateSyncKey(), s.JID, id).Scan(&key.Data, &key.Timestamp, &key.Fingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &key, err
}

func (s *SQLStore) PutAppStateVersion(name string, version uint64, hash [128]byte) error {
	_, err := s.db.Exec(s.query.PutAppStateVersion(), s.JID, name, version, hash[:])
	return err
}

func (s *SQLStore) GetAppStateVersion(name string) (version uint64, hash [128]byte, err error) {
	var uncheckedHash []byte
	err = s.db.QueryRow(s.query.GetAppStateVersion(), s.JID, name).Scan(&version, &uncheckedHash)
	if errors.Is(err, sql.ErrNoRows) {
		// version will be 0 and hash will be an empty array, which is the correct initial state
		err = nil
	} else if err != nil {
		// There's an error, just return it
	} else if len(uncheckedHash) != 128 {
		// This shouldn't happen
		err = ErrInvalidLength
	} else {
		// No errors, convert hash slice to array
		hash = *(*[128]byte)(uncheckedHash)
	}
	return
}

func (s *SQLStore) DeleteAppStateVersion(name string) error {
	_, err := s.db.Exec(s.query.DeleteAppStateVersion(), s.JID, name)
	return err
}

type execable interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func (s *SQLStore) putAppStateMutationMACs(tx execable, name string, version uint64, mutations []store.AppStateMutationMAC) error {
	var values []any
	for _, mutation := range mutations {
		values = append(values, s.JID, name, version, mutation.IndexMAC, mutation.ValueMAC)
	}
	_, err := tx.Exec(s.query.PutAppStateMutationMACs(s.query.PlaceholderCreate(5, len(mutations))), values...)
	return err
}

const mutationBatchSize = 400

func (s *SQLStore) PutAppStateMutationMACs(name string, version uint64, mutations []store.AppStateMutationMAC) error {
	if len(mutations) > mutationBatchSize {
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}
		for i := 0; i < len(mutations); i += mutationBatchSize {
			var mutationSlice []store.AppStateMutationMAC
			if len(mutations) > i+mutationBatchSize {
				mutationSlice = mutations[i : i+mutationBatchSize]
			} else {
				mutationSlice = mutations[i:]
			}
			err = s.putAppStateMutationMACs(tx, name, version, mutationSlice)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
		}
		err = tx.Commit()
		if err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
		return nil
	} else if len(mutations) > 0 {
		return s.putAppStateMutationMACs(s.db, name, version, mutations)
	}
	return nil
}

func (s *SQLStore) DeleteAppStateMutationMACs(name string, indexMACs [][]byte) (err error) {
	if len(indexMACs) == 0 {
		return
	}
	if s.dialect == "postgres" && PostgresArrayWrapper != nil {
		_, err = s.db.Exec(s.query.DeleteAppStateMutationMACs(""), s.JID, name, PostgresArrayWrapper(indexMACs))
	} else {
		args := []any{s.JID, name}
		for _, item := range indexMACs {
			args = append(args, item)
		}
		_, err = s.db.Exec(s.query.DeleteAppStateMutationMACs(s.query.PlaceholderCreate(len(indexMACs), 1)), args...)
	}
	return
}

func (s *SQLStore) GetAppStateMutationMAC(name string, indexMAC []byte) (valueMAC []byte, err error) {
	err = s.db.QueryRow(s.query.GetAppStateMutationMAC(), s.JID, name, indexMAC).Scan(&valueMAC)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return
}

func (s *SQLStore) PutPushName(user types.JID, pushName string) (bool, string, error) {
	s.contactCacheLock.Lock()
	defer s.contactCacheLock.Unlock()

	cached, err := s.getContact(user)
	if err != nil {
		return false, "", err
	}
	if cached.PushName != pushName {
		_, err = s.db.Exec(s.query.PutPushName(), s.JID, user, pushName)
		if err != nil {
			return false, "", err
		}
		previousName := cached.PushName
		cached.PushName = pushName
		cached.Found = true
		return true, previousName, nil
	}
	return false, "", nil
}

func (s *SQLStore) PutBusinessName(user types.JID, businessName string) (bool, string, error) {
	s.contactCacheLock.Lock()
	defer s.contactCacheLock.Unlock()

	cached, err := s.getContact(user)
	if err != nil {
		return false, "", err
	}
	if cached.BusinessName != businessName {
		_, err = s.db.Exec(s.query.PutBusinessName(), s.JID, user, businessName)
		if err != nil {
			return false, "", err
		}
		previousName := cached.BusinessName
		cached.BusinessName = businessName
		cached.Found = true
		return true, previousName, nil
	}
	return false, "", nil
}

func (s *SQLStore) PutContactName(user types.JID, firstName, fullName string) error {
	s.contactCacheLock.Lock()
	defer s.contactCacheLock.Unlock()

	cached, err := s.getContact(user)
	if err != nil {
		return err
	}
	if cached.FirstName != firstName || cached.FullName != fullName {
		_, err = s.db.Exec(s.query.PutContactName(), s.JID, user, firstName, fullName)
		if err != nil {
			return err
		}
		cached.FirstName = firstName
		cached.FullName = fullName
		cached.Found = true
	}
	return nil
}

const contactBatchSize = 300

func (s *SQLStore) putContactNamesBatch(tx execable, contacts []store.ContactEntry) error {
	var values []any
	handledContacts := make(map[types.JID]struct{}, len(contacts))
	for _, contact := range contacts {
		if contact.JID.IsEmpty() {
			s.log.Warnf("Empty contact info in mass insert: %+v", contact)
			continue
		}
		// The whole query will break if there are duplicates, so make sure there aren't any duplicates
		_, alreadyHandled := handledContacts[contact.JID]
		if alreadyHandled {
			s.log.Warnf("Duplicate contact info for %s in mass insert", contact.JID)
			continue
		}
		handledContacts[contact.JID] = struct{}{}
		values = append(values, s.JID, contact.JID.String(), contact.FirstName, contact.FullName)
	}
	_, err := tx.Exec(s.query.PutManyContactNames(s.query.PlaceholderCreate(4, len(contacts))), values...)
	return err
}

func (s *SQLStore) PutAllContactNames(contacts []store.ContactEntry) error {
	if len(contacts) > contactBatchSize {
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}
		for i := 0; i < len(contacts); i += contactBatchSize {
			var contactSlice []store.ContactEntry
			if len(contacts) > i+contactBatchSize {
				contactSlice = contacts[i : i+contactBatchSize]
			} else {
				contactSlice = contacts[i:]
			}
			err = s.putContactNamesBatch(tx, contactSlice)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
		}
		err = tx.Commit()
		if err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
	} else if len(contacts) > 0 {
		err := s.putContactNamesBatch(s.db, contacts)
		if err != nil {
			return err
		}
	} else {
		return nil
	}
	s.contactCacheLock.Lock()
	// Just clear the cache, fetching pushnames and business names would be too much effort
	s.contactCache = make(map[types.JID]*types.ContactInfo)
	s.contactCacheLock.Unlock()
	return nil
}

func (s *SQLStore) getContact(user types.JID) (*types.ContactInfo, error) {
	cached, ok := s.contactCache[user]
	if ok {
		return cached, nil
	}

	var first, full, push, business sql.NullString
	err := s.db.QueryRow(s.query.GetContact(), s.JID, user).Scan(&first, &full, &push, &business)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	info := &types.ContactInfo{
		Found:        err == nil,
		FirstName:    first.String,
		FullName:     full.String,
		PushName:     push.String,
		BusinessName: business.String,
	}
	s.contactCache[user] = info
	return info, nil
}

func (s *SQLStore) GetContact(user types.JID) (types.ContactInfo, error) {
	s.contactCacheLock.Lock()
	info, err := s.getContact(user)
	s.contactCacheLock.Unlock()
	if err != nil {
		return types.ContactInfo{}, err
	}
	return *info, nil
}

func (s *SQLStore) GetAllContacts() (map[types.JID]types.ContactInfo, error) {
	s.contactCacheLock.Lock()
	defer s.contactCacheLock.Unlock()
	rows, err := s.db.Query(s.query.GetAllContacts(), s.JID)
	if err != nil {
		return nil, err
	}
	output := make(map[types.JID]types.ContactInfo, len(s.contactCache))
	for rows.Next() {
		var jid types.JID
		var first, full, push, business sql.NullString
		err = rows.Scan(&jid, &first, &full, &push, &business)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		info := types.ContactInfo{
			Found:        true,
			FirstName:    first.String,
			FullName:     full.String,
			PushName:     push.String,
			BusinessName: business.String,
		}
		output[jid] = info
		s.contactCache[jid] = &info
	}
	return output, nil
}

func (s *SQLStore) PutMutedUntil(chat types.JID, mutedUntil time.Time) error {
	var val int64
	if !mutedUntil.IsZero() {
		val = mutedUntil.Unix()
	}
	_, err := s.db.Exec(s.query.PutChatSetting("muted_until"), s.JID, chat, val)
	return err
}

func (s *SQLStore) PutPinned(chat types.JID, pinned bool) error {
	_, err := s.db.Exec(s.query.PutChatSetting("pinned"), s.JID, chat, pinned)
	return err
}

func (s *SQLStore) PutArchived(chat types.JID, archived bool) error {
	_, err := s.db.Exec(s.query.PutChatSetting("archived"), s.JID, chat, archived)
	return err
}

func (s *SQLStore) GetChatSettings(chat types.JID) (settings types.LocalChatSettings, err error) {
	var mutedUntil int64
	err = s.db.QueryRow(s.query.GetChatSettings(), s.JID, chat).Scan(&mutedUntil, &settings.Pinned, &settings.Archived)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	} else if err != nil {
		return
	} else {
		settings.Found = true
	}
	if mutedUntil != 0 {
		settings.MutedUntil = time.Unix(mutedUntil, 0)
	}
	return
}

func (s *SQLStore) PutMessageSecrets(inserts []store.MessageSecretInsert) (err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	for _, insert := range inserts {
		_, err = tx.Exec(s.query.PutMsgSecret(), s.JID, insert.Chat.ToNonAD(), insert.Sender.ToNonAD(), insert.ID, insert.Secret)
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return
}

func (s *SQLStore) PutMessageSecret(chat, sender types.JID, id types.MessageID, secret []byte) (err error) {
	_, err = s.db.Exec(s.query.PutMsgSecret(), s.JID, chat.ToNonAD(), sender.ToNonAD(), id, secret)
	return
}

func (s *SQLStore) GetMessageSecret(chat, sender types.JID, id types.MessageID) (secret []byte, err error) {
	err = s.db.QueryRow(s.query.GetMsgSecret(), s.JID, chat.ToNonAD(), sender.ToNonAD(), id).Scan(&secret)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return
}

func (s *SQLStore) PutPrivacyTokens(tokens ...store.PrivacyToken) error {
	args := make([]any, 0)
	for _, token := range tokens {
		args = append(args, s.JID, token.User.ToNonAD().String(), token.Token, token.Timestamp.Unix())
	}
	placeholders := s.query.PlaceholderCreate(4, len(tokens))
	_, err := s.db.Exec(s.query.PutPrivacyTokens(placeholders), args...)
	return err
}

func (s *SQLStore) GetPrivacyToken(user types.JID) (*store.PrivacyToken, error) {
	var token store.PrivacyToken
	token.User = user.ToNonAD()
	var ts int64
	err := s.db.QueryRow(s.query.GetPrivacyToken(), s.JID, token.User).Scan(&token.Token, &ts)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else {
		token.Timestamp = time.Unix(ts, 0)
		return &token, nil
	}
}
