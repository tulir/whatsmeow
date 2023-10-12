// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package sqlstore contains an SQL-backed implementation of the interfaces in the store package.
package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/lib/pq"
	"strings"
	"sync"
	"time"

	_ "context"
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
//var PostgresArrayWrapper func(interface{}) interface {
//	driver.Valuer
//	sql.Scanner
//}

type SQLStore struct {
	*Container
	businessId string
	JID        string

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
		businessId:   c.businessId,
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

const (
	putIdentityQuery = `
		INSERT INTO whatsmeow_identity_keys (business_id, our_jid, their_id, identity) VALUES ($1, $2, $3, $4)
		ON CONFLICT (business_id, our_jid, their_id) DO UPDATE SET identity=excluded.identity
	`
	deleteAllIdentitiesQuery = `DELETE FROM whatsmeow_identity_keys WHERE business_id=$1 AND our_jid=$2 AND their_id LIKE $3`
	deleteIdentityQuery      = `DELETE FROM whatsmeow_identity_keys WHERE business_id=$1 AND our_jid=$2 AND their_id=$3`
	getIdentityQuery         = `SELECT identity FROM whatsmeow_identity_keys WHERE business_id=$1 AND our_jid=$2 AND their_id=$3`
)

func (s *SQLStore) PutIdentity(address string, key [32]byte) error {
	row, err := s.dbPool.Query(context.Background(), putIdentityQuery, s.businessId, s.JID, address, key[:])
	defer row.Close()
	return err
}

func (s *SQLStore) DeleteAllIdentities(phone string) error {
	row, err := s.dbPool.Query(context.Background(), deleteAllIdentitiesQuery, s.businessId, s.JID, phone+":%")
	defer row.Close()
	return err
}

func (s *SQLStore) DeleteIdentity(address string) error {
	row, err := s.dbPool.Query(context.Background(), deleteAllIdentitiesQuery, s.businessId, s.JID, address)
	defer row.Close()
	return err
}

func (s *SQLStore) IsTrustedIdentity(address string, key [32]byte) (bool, error) {
	var existingIdentity []byte

	row := s.dbPool.QueryRow(context.Background(), getIdentityQuery, s.businessId, s.JID, address)
	err := row.Scan(&existingIdentity)
	if errors.Is(err, pgx.ErrNoRows) {
		// Trust if not known, it'll be saved automatically later
		return true, nil
	} else if err != nil {
		return false, err
	} else if len(existingIdentity) != 32 {
		return false, ErrInvalidLength
	}
	return *(*[32]byte)(existingIdentity) == key, nil
}

const (
	getSessionQuery = `SELECT session FROM whatsmeow_sessions WHERE business_id=$1 AND our_jid=$2 AND their_id=$3`
	hasSessionQuery = `SELECT true FROM whatsmeow_sessions WHERE business_id=$1 AND our_jid=$2 AND their_id=$3`
	putSessionQuery = `
		INSERT INTO whatsmeow_sessions (business_id, our_jid, their_id, session) VALUES ($1, $2, $3, $4)
		ON CONFLICT (business_id, our_jid, their_id) DO UPDATE SET session=excluded.session
	`
	deleteAllSessionsQuery = `DELETE FROM whatsmeow_sessions WHERE business_id=$1 AND our_jid=$2 AND their_id LIKE $3`
	deleteSessionQuery     = `DELETE FROM whatsmeow_sessions WHERE business_id=$1 AND our_jid=$2 AND their_id=$3`
)

func (s *SQLStore) GetSession(address string) (session []byte, err error) {
	row := s.dbPool.QueryRow(context.Background(), getSessionQuery, s.businessId, s.JID, address)
	err = row.Scan(&session)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return
}

func (s *SQLStore) HasSession(address string) (has bool, err error) {
	row := s.dbPool.QueryRow(context.Background(), hasSessionQuery, s.businessId, s.JID, address)
	err = row.Scan(&has)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return
}

func (s *SQLStore) PutSession(address string, session []byte) error {
	row, err := s.dbPool.Query(context.Background(), putSessionQuery, s.businessId, s.JID, address, session)
	defer row.Close()
	return err
}

func (s *SQLStore) DeleteAllSessions(phone string) error {
	row, err := s.dbPool.Query(context.Background(), deleteAllSessionsQuery, s.businessId, s.JID, phone+":%")
	defer row.Close()
	return err
}

func (s *SQLStore) DeleteSession(address string) error {
	row, err := s.dbPool.Query(context.Background(), deleteSessionQuery, s.businessId, s.JID, address)
	defer row.Close()
	return err
}

const (
	getLastPreKeyIDQuery        = `SELECT MAX(key_id) FROM whatsmeow_pre_keys WHERE business_id=$1 AND jid=$2`
	insertPreKeyQuery           = `INSERT INTO whatsmeow_pre_keys (business_id, jid, key_id, key, uploaded) VALUES ($1, $2, $3, $4, $5)`
	getUnuploadedPreKeysQuery   = `SELECT key_id, key FROM whatsmeow_pre_keys WHERE business_id=$1 AND jid=$2 AND uploaded=false ORDER BY key_id LIMIT $3`
	getPreKeyQuery              = `SELECT key_id, key FROM whatsmeow_pre_keys WHERE business_id=$1 AND jid=$2 AND key_id=$3`
	deletePreKeyQuery           = `DELETE FROM whatsmeow_pre_keys WHERE business_id=$1 AND jid=$2 AND key_id=$3`
	markPreKeysAsUploadedQuery  = `UPDATE whatsmeow_pre_keys SET uploaded=true WHERE business_id=$1 AND jid=$2 AND key_id<=$3`
	getUploadedPreKeyCountQuery = `SELECT COUNT(*) FROM whatsmeow_pre_keys WHERE business_id=$1 AND jid=$2 AND uploaded=true`
)

func (s *SQLStore) genOnePreKey(id uint32, markUploaded bool) (*keys.PreKey, error) {
	key := keys.NewPreKey(id)
	row, err := s.dbPool.Query(context.Background(), insertPreKeyQuery, s.businessId, s.JID, key.KeyID, key.Priv[:], markUploaded)
	defer row.Close()
	return key, err
}

func (s *SQLStore) getNextPreKeyID() (uint32, error) {
	var lastKeyID sql.NullInt32
	row := s.dbPool.QueryRow(context.Background(), getLastPreKeyIDQuery, s.businessId, s.JID)
	err := row.Scan(&lastKeyID)
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

	res, err := s.dbPool.Query(context.Background(), getUnuploadedPreKeysQuery, s.businessId, s.JID, count)
	defer res.Close()
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
	if errors.Is(err, pgx.ErrNoRows) {
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
	return scanPreKey(s.dbPool.QueryRow(context.Background(), getPreKeyQuery, s.businessId, s.JID, id))
}

func (s *SQLStore) RemovePreKey(id uint32) error {
	row, err := s.dbPool.Query(context.Background(), deletePreKeyQuery, s.businessId, s.JID, id)
	defer row.Close()
	return err
}

func (s *SQLStore) MarkPreKeysAsUploaded(upToID uint32) error {
	row, err := s.dbPool.Query(context.Background(), markPreKeysAsUploadedQuery, s.businessId, s.JID, upToID)
	defer row.Close()
	return err
}

func (s *SQLStore) UploadedPreKeyCount() (count int, err error) {
	row := s.dbPool.QueryRow(context.Background(), getUploadedPreKeyCountQuery, s.businessId, s.JID)
	err = row.Scan(&count)
	return
}

const (
	getSenderKeyQuery = `SELECT sender_key FROM whatsmeow_sender_keys WHERE business_id=$1 AND our_jid=$2 AND chat_id=$3 AND sender_id=$4`
	putSenderKeyQuery = `
		INSERT INTO whatsmeow_sender_keys (business_id, our_jid, chat_id, sender_id, sender_key) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (business_id, our_jid, chat_id, sender_id) DO UPDATE SET sender_key=excluded.sender_key
	`
)

func (s *SQLStore) PutSenderKey(group, user string, session []byte) error {
	row, err := s.dbPool.Query(context.Background(), putSenderKeyQuery, s.businessId, s.JID, group, user, session)
	defer row.Close()
	return err
}

func (s *SQLStore) GetSenderKey(group, user string) (key []byte, err error) {
	row := s.dbPool.QueryRow(context.Background(), getSenderKeyQuery, s.businessId, s.JID, group, user)
	err = row.Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return
}

const (
	putAppStateSyncKeyQuery = `
		INSERT INTO whatsmeow_app_state_sync_keys (business_id, jid, key_id, key_data, timestamp, fingerprint) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (business_id, jid, key_id) DO UPDATE
			SET key_data=excluded.key_data, timestamp=excluded.timestamp, fingerprint=excluded.fingerprint
			WHERE excluded.timestamp > whatsmeow_app_state_sync_keys.timestamp
	`
	getAppStateSyncKeyQuery         = `SELECT key_data, timestamp, fingerprint FROM whatsmeow_app_state_sync_keys WHERE business_id=$1 AND jid=$2 AND key_id=$3`
	getLatestAppStateSyncKeyIDQuery = `SELECT key_id FROM whatsmeow_app_state_sync_keys WHERE business_id=$1 AND jid=$2 ORDER BY timestamp DESC LIMIT 1`
)

func (s *SQLStore) PutAppStateSyncKey(id []byte, key store.AppStateSyncKey) error {
	row, err := s.dbPool.Query(context.Background(), putAppStateSyncKeyQuery, s.businessId, s.JID, id, key.Data, key.Timestamp, key.Fingerprint)
	defer row.Close()
	return err
}

func (s *SQLStore) GetAppStateSyncKey(id []byte) (*store.AppStateSyncKey, error) {
	var key store.AppStateSyncKey
	row := s.dbPool.QueryRow(context.Background(), getAppStateSyncKeyQuery, s.businessId, s.JID, id)
	err := row.Scan(&key.Data, &key.Timestamp, &key.Fingerprint)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &key, err
}

func (s *SQLStore) GetLatestAppStateSyncKeyID() ([]byte, error) {
	var keyID []byte
	row := s.dbPool.QueryRow(context.Background(), getLatestAppStateSyncKeyIDQuery, s.businessId, s.JID)
	err := row.Scan(&keyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return keyID, err
}

const (
	putAppStateVersionQuery = `
		INSERT INTO whatsmeow_app_state_version (business_id, jid, name, version, hash) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (business_id, jid, name) DO UPDATE SET version=excluded.version, hash=excluded.hash
	`
	getAppStateVersionQuery                 = `SELECT version, hash FROM whatsmeow_app_state_version WHERE business_id=$1 AND jid=$2 AND name=$3`
	deleteAppStateVersionQuery              = `DELETE FROM whatsmeow_app_state_version WHERE business_id=$1 AND jid=$2 AND name=$3`
	putAppStateMutationMACsQuery            = `INSERT INTO whatsmeow_app_state_mutation_macs (business_id, jid, name, version, index_mac, value_mac) VALUES `
	deleteAppStateMutationMACsQueryPostgres = `DELETE FROM whatsmeow_app_state_mutation_macs WHERE business_id=$1 AND jid=$2 AND name=$3 AND index_mac=ANY($4::bytea[])`
	deleteAppStateMutationMACsQueryGeneric  = `DELETE FROM whatsmeow_app_state_mutation_macs WHERE business_id=$1 AND jid=$2 AND name=$3 AND index_mac IN `
	getAppStateMutationMACQuery             = `SELECT value_mac FROM whatsmeow_app_state_mutation_macs WHERE business_id=$1 AND jid=$2 AND name=$3 AND index_mac=$4 ORDER BY version DESC LIMIT 1`
)

func (s *SQLStore) PutAppStateVersion(name string, version uint64, hash [128]byte) error {
	row, err := s.dbPool.Query(context.Background(), putAppStateVersionQuery, s.businessId, s.JID, name, version, hash[:])
	defer row.Close()
	return err
}

func (s *SQLStore) GetAppStateVersion(name string) (version uint64, hash [128]byte, err error) {
	var uncheckedHash []byte
	row := s.dbPool.QueryRow(context.Background(), getAppStateVersionQuery, s.businessId, s.JID, name)
	err = row.Scan(&version, &uncheckedHash)
	if errors.Is(err, pgx.ErrNoRows) {
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
	row, err := s.dbPool.Query(context.Background(), deleteAppStateVersionQuery, s.businessId, s.JID, name)
	defer row.Close()
	return err
}

type execable interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func (s *SQLStore) putAppStateMutationMACs(tx pgx.Tx, name string, version uint64, mutations []store.AppStateMutationMAC) error {
	values := make([]interface{}, 4+len(mutations)*2)
	queryParts := make([]string, len(mutations))
	values[0] = s.businessId
	values[1] = s.JID
	values[2] = name
	values[3] = version
	placeholderSyntax := "($1, $2, $3, $4, $%d, $%d)"
	for i, mutation := range mutations {
		baseIndex := 4 + i*2
		values[baseIndex] = mutation.IndexMAC
		values[baseIndex+1] = mutation.ValueMAC
		queryParts[i] = fmt.Sprintf(placeholderSyntax, baseIndex+1, baseIndex+2)
	}
	_, err := tx.Exec(context.Background(), putAppStateMutationMACsQuery+strings.Join(queryParts, ","), values...)
	return err
}

const mutationBatchSize = 400

func (s *SQLStore) PutAppStateMutationMACs(name string, version uint64, mutations []store.AppStateMutationMAC) error {
	if len(mutations) > 0 {
		tx, err := s.dbPool.Begin(context.Background())
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
				_ = tx.Rollback(context.Background())
				return err
			}
		}
		err = tx.Commit(context.Background())
		if err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
		return nil
	}
	return nil
}

func (s *SQLStore) DeleteAppStateMutationMACs(name string, indexMACs [][]byte) (err error) {
	if len(indexMACs) == 0 {
		return
	}
	var row pgx.Rows = nil
	row, err = s.dbPool.Query(context.Background(), deleteAppStateMutationMACsQueryPostgres, s.businessId, s.JID, name, pq.Array(indexMACs))
	defer row.Close()
	return
}

func (s *SQLStore) GetAppStateMutationMAC(name string, indexMAC []byte) (valueMAC []byte, err error) {
	row := s.dbPool.QueryRow(context.Background(), getAppStateMutationMACQuery, s.businessId, s.JID, name, indexMAC)
	err = row.Scan(&valueMAC)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return
}

const (
	putContactNameQuery = `
		INSERT INTO whatsmeow_contacts (business_id, our_jid, their_jid, first_name, full_name) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (business_id, our_jid, their_jid) DO UPDATE SET first_name=excluded.first_name, full_name=excluded.full_name
	`
	putManyContactNamesQuery = `
		INSERT INTO whatsmeow_contacts (business_id, our_jid, their_jid, first_name, full_name)
		VALUES (%s)
		ON CONFLICT (business_id, our_jid, their_jid) DO UPDATE SET first_name=excluded.first_name, full_name=excluded.full_name
	`
	putPushNameQuery = `
		INSERT INTO whatsmeow_contacts (business_id, our_jid, their_jid, push_name) VALUES ($1, $2, $3, $4)
		ON CONFLICT (business_id, our_jid, their_jid) DO UPDATE SET push_name=excluded.push_name
	`
	putBusinessNameQuery = `
		INSERT INTO whatsmeow_contacts (business_id, our_jid, their_jid, business_name) VALUES ($1, $2, $3, $4)
		ON CONFLICT (business_id, our_jid, their_jid) DO UPDATE SET business_name=excluded.business_name
	`
	getContactQuery = `
		SELECT business_id, first_name, full_name, push_name, business_name FROM whatsmeow_contacts WHERE business_id=$1 AND our_jid=$2 AND their_jid=$3
	`
	getAllContactsQuery = `
		SELECT business_id, their_jid, first_name, full_name, push_name, business_name FROM whatsmeow_contacts WHERE business_id=$1 our_jid=$2
	`
)

func (s *SQLStore) PutPushName(user types.JID, pushName string) (bool, string, error) {
	s.contactCacheLock.Lock()
	defer s.contactCacheLock.Unlock()

	cached, err := s.getContact(user)
	if err != nil {
		return false, "", err
	}
	if cached.PushName != pushName {
		var row pgx.Rows = nil
		row, err = s.dbPool.Query(context.Background(), putPushNameQuery, s.businessId, s.JID, user, pushName)
		defer row.Close()
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
		var row pgx.Rows = nil
		row, err = s.dbPool.Query(context.Background(), putBusinessNameQuery, s.businessId, s.JID, user, businessName)
		defer row.Close()
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
		var row pgx.Rows = nil
		row, err = s.dbPool.Query(context.Background(), putContactNameQuery, s.businessId, s.JID, user, firstName, fullName)
		defer row.Close()
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

func (s *SQLStore) putContactNamesBatch(tx pgx.Tx, contacts []store.ContactEntry) error {
	values := make([]interface{}, 2, 2+len(contacts)*3)
	queryParts := make([]string, 0, len(contacts))
	values[0] = s.businessId
	values[1] = s.JID
	placeholderSyntax := "($1, $2, $%d, $%d, $%d)"
	i := 0
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
		baseIndex := i*3 + 2
		values = append(values, contact.JID.String(), contact.FirstName, contact.FullName)
		queryParts = append(queryParts, fmt.Sprintf(placeholderSyntax, baseIndex+1, baseIndex+2, baseIndex+3))
		i++
	}
	_, err := tx.Exec(context.Background(), fmt.Sprintf(putManyContactNamesQuery, strings.Join(queryParts, ",")), values...)
	return err
}

func (s *SQLStore) PutAllContactNames(contacts []store.ContactEntry) error {
	if len(contacts) > contactBatchSize {
		tx, err := s.dbPool.Begin(context.Background())
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
				_ = tx.Rollback(context.Background())
				return err
			}
		}
		err = tx.Commit(context.Background())
		if err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
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
	row := s.dbPool.QueryRow(context.Background(), getContactQuery, s.businessId, s.JID, user)
	err := row.Scan(&first, &full, &push, &business)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
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
	rows, err := s.dbPool.Query(context.Background(), getAllContactsQuery, s.businessId, s.JID)
	defer rows.Close()
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

const (
	putChatSettingQuery = `
		INSERT INTO whatsmeow_chat_settings (business_id, our_jid, chat_jid, %[1]s) VALUES ($1, $2, $3, $4)
		ON CONFLICT (business_id, our_jid, chat_jid) DO UPDATE SET %[1]s=excluded.%[1]s
	`
	getChatSettingsQuery = `
		SELECT muted_until, pinned, archived FROM whatsmeow_chat_settings WHERE business_id=$1 AND our_jid=$2 AND chat_jid=$3
	`
)

func (s *SQLStore) PutMutedUntil(chat types.JID, mutedUntil time.Time) error {
	var val int64
	if !mutedUntil.IsZero() {
		val = mutedUntil.Unix()
	}
	row, err := s.dbPool.Query(context.Background(), fmt.Sprintf(putChatSettingQuery, "muted_until"), s.businessId, s.JID, chat, val)
	defer row.Close()
	return err
}

func (s *SQLStore) PutPinned(chat types.JID, pinned bool) error {
	row, err := s.dbPool.Query(context.Background(), fmt.Sprintf(putChatSettingQuery, "pinned"), s.businessId, s.JID, chat, pinned)
	defer row.Close()
	return err
}

func (s *SQLStore) PutArchived(chat types.JID, archived bool) error {
	row, err := s.dbPool.Query(context.Background(), fmt.Sprintf(putChatSettingQuery, "archived"), s.businessId, s.JID, chat, archived)
	defer row.Close()
	return err
}

func (s *SQLStore) GetChatSettings(chat types.JID) (settings types.LocalChatSettings, err error) {
	var mutedUntil int64
	row := s.dbPool.QueryRow(context.Background(), getChatSettingsQuery, s.businessId, s.JID, chat)
	err = row.Scan(&mutedUntil, &settings.Pinned, &settings.Archived)
	if errors.Is(err, pgx.ErrNoRows) {
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

const (
	putMsgSecret = `
		INSERT INTO whatsmeow_message_secrets (business_id, our_jid, chat_jid, sender_jid, message_id, key)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (business_id, our_jid, chat_jid, sender_jid, message_id) DO NOTHING
	`
	getMsgSecret = `
		SELECT key FROM whatsmeow_message_secrets WHERE business_id=$1 AND our_jid=$2 AND chat_jid=$3 AND sender_jid=$4 AND message_id=$5
	`
)

func (s *SQLStore) PutMessageSecrets(inserts []store.MessageSecretInsert) (err error) {
	tx, err := s.dbPool.Begin(context.Background())
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	for _, insert := range inserts {
		_, err = tx.Exec(context.Background(), putMsgSecret, s.businessId, s.JID, insert.Chat.ToNonAD(), insert.Sender.ToNonAD(), insert.ID, insert.Secret)
	}
	err = tx.Commit(context.Background())
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return
}

func (s *SQLStore) PutMessageSecret(chat, sender types.JID, id types.MessageID, secret []byte) (err error) {
	var row pgx.Rows = nil
	row, err = s.dbPool.Query(context.Background(), putMsgSecret, s.businessId, s.JID, chat.ToNonAD(), sender.ToNonAD(), id, secret)
	defer row.Close()
	return
}

func (s *SQLStore) GetMessageSecret(chat, sender types.JID, id types.MessageID) (secret []byte, err error) {
	row := s.dbPool.QueryRow(context.Background(), getMsgSecret, s.businessId, s.JID, chat.ToNonAD(), sender.ToNonAD(), id)
	err = row.Scan(&secret)
	if errors.Is(err, pgx.ErrNoRows) {
		err = nil
	}
	return
}

const (
	putPrivacyTokens = `
		INSERT INTO whatsmeow_privacy_tokens (business_id, our_jid, their_jid, token, timestamp)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (business_id, our_jid, their_jid) DO UPDATE SET token=EXCLUDED.token, timestamp=EXCLUDED.timestamp
	`
	getPrivacyToken = `SELECT token, timestamp FROM whatsmeow_privacy_tokens WHERE business_id=$1 AND our_jid=$2 AND their_jid=$3`
)

func (s *SQLStore) PutPrivacyTokens(tokens ...store.PrivacyToken) error {
	args := make([]any, 2+len(tokens)*3)
	placeholders := make([]string, len(tokens))
	args[0] = s.businessId
	args[1] = s.JID
	for i, token := range tokens {
		args[i*3+2] = token.User.ToNonAD().String()
		args[i*3+3] = token.Token
		args[i*3+4] = token.Timestamp.Unix()
		placeholders[i] = fmt.Sprintf("($1, $2, $%d, $%d, $%d)", i*3+3, i*3+4, i*3+5)
	}
	query := strings.ReplaceAll(putPrivacyTokens, "($1, $2, $3, $4, $5)", strings.Join(placeholders, ","))
	row, err := s.dbPool.Query(context.Background(), query, args...)
	defer row.Close()
	return err
}

func (s *SQLStore) GetPrivacyToken(user types.JID) (*store.PrivacyToken, error) {
	var token store.PrivacyToken
	token.User = user.ToNonAD()
	var ts int64
	row := s.dbPool.QueryRow(context.Background(), getPrivacyToken, s.businessId, s.JID, token.User)
	err := row.Scan(&token.Token, &ts)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	} else {
		token.Timestamp = time.Unix(ts, 0)
		return &token, nil
	}
}
