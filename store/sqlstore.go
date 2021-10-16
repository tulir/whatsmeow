// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"go.mau.fi/whatsmeow/keys"
)

var ErrInvalidLength = errors.New("database returned byte array with illegal length")

type SQLStore struct {
	*SQLContainer
	JID string

	preKeyLock sync.Mutex
}

var _ IdentityStore = (*SQLStore)(nil)
var _ SessionStore = (*SQLStore)(nil)
var _ PreKeyStore = (*SQLStore)(nil)
var _ SenderKeyStore = (*SQLStore)(nil)

const (
	putIdentityQuery = `
		INSERT INTO whatsmeow_identity_keys (our_jid, their_id, identity) VALUES ($1, $2, $3)
		ON CONFLICT (our_jid, their_id) DO UPDATE SET identity=$3
	`
	getIdentityQuery = `SELECT identity FROM whatsmeow_identity_keys WHERE our_jid=$1 AND their_id=$2`
)

func (s *SQLStore) PutIdentity(address string, key [32]byte) error {
	_, err := s.db.Exec(putIdentityQuery, s.JID, address, key[:])
	return err
}

func (s *SQLStore) IsTrustedIdentity(address string, key [32]byte) (bool, error) {
	var existingIdentity []byte
	err := s.db.QueryRow(getIdentityQuery, s.JID, address).Scan(&existingIdentity)
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

const (
	getSessionQuery = `SELECT session FROM whatsmeow_sessions WHERE our_jid=$1 AND their_id=$2`
	hasSessionQuery = `SELECT true FROM whatsmeow_sessions WHERE our_jid=$1 AND their_id=$2`
	putSessionQuery = `
		INSERT INTO whatsmeow_sessions (our_jid, their_id, session) VALUES ($1, $2, $3)
		ON CONFLICT (our_jid, their_id) DO UPDATE SET session=$3
	`
)

func (s *SQLStore) GetSession(address string) (session []byte, err error) {
	err = s.db.QueryRow(getSessionQuery, s.JID, address).Scan(&session)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return
}

func (s *SQLStore) HasSession(address string) (has bool, err error) {
	err = s.db.QueryRow(hasSessionQuery, s.JID, address).Scan(&has)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return
}

func (s *SQLStore) PutSession(address string, session []byte) error {
	_, err := s.db.Exec(putSessionQuery, s.JID, address, session)
	return err
}

const (
	getLastPreKeyIDQuery        = `SELECT MAX(key_id) FROM whatsmeow_pre_keys WHERE jid=$1`
	insertPreKeyQuery           = `INSERT INTO whatsmeow_pre_keys (jid, key_id, key, uploaded) VALUES ($1, $2, $3, $4)`
	getUnuploadedPreKeysQuery   = `SELECT key_id, key FROM whatsmeow_pre_keys WHERE jid=$1 AND uploaded=false ORDER BY key_id LIMIT $2`
	getPreKeyQuery              = `SELECT key_id, key FROM whatsmeow_pre_keys WHERE jid=$1 AND key_id=$2`
	deletePreKeyQuery           = `DELETE FROM whatsmeow_pre_keys WHERE jid=$1 AND key_id=$2`
	markPreKeysAsUploadedQuery  = `UPDATE whatsmeow_pre_keys SET uploaded=true WHERE jid=$1 AND key_id<=$2`
	getUploadedPreKeyCountQuery = `SELECT COUNT(*) FROM whatsmeow_pre_keys WHERE jid=$1 AND uploaded=true`
)

func (s *SQLStore) genOnePreKey(id uint32, markUploaded bool) (*keys.PreKey, error) {
	key := keys.NewPreKey(id)
	_, err := s.db.Exec(insertPreKeyQuery, s.JID, key.KeyID, key.Priv[:], markUploaded)
	return key, err
}

func (s *SQLStore) getNextPreKeyID() (uint32, error) {
	var lastKeyID sql.NullInt32
	err := s.db.QueryRow(getLastPreKeyIDQuery, s.JID).Scan(&lastKeyID)
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

	res, err := s.db.Query(getUnuploadedPreKeysQuery, s.JID, count)
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
	return scanPreKey(s.db.QueryRow(getPreKeyQuery, s.JID, id))
}

func (s *SQLStore) RemovePreKey(id uint32) error {
	_, err := s.db.Exec(deletePreKeyQuery, s.JID, id)
	return err
}

func (s *SQLStore) MarkPreKeysAsUploaded(upToID uint32) error {
	_, err := s.db.Exec(markPreKeysAsUploadedQuery, s.JID, upToID)
	return err
}

func (s *SQLStore) UploadedPreKeyCount() (count int, err error) {
	err = s.db.QueryRow(getUploadedPreKeyCountQuery, s.JID).Scan(&count)
	return
}

const (
	getSenderKeyQuery = `SELECT sender_key FROM whatsmeow_sender_keys WHERE our_jid=$1 AND chat_id=$2 AND sender_id=$3`
	putSenderKeyQuery = `
		INSERT INTO whatsmeow_sender_keys (our_jid, chat_id, sender_id, sender_key) VALUES ($1, $2, $3, $4)
		ON CONFLICT (our_jid, chat_id, sender_id) DO UPDATE SET sender_key=$4
	`
)

func (s *SQLStore) PutSenderKey(group, user string, session []byte) error {
	_, err := s.db.Exec(putSenderKeyQuery, s.JID, group, user, session)
	return err
}

func (s *SQLStore) GetSenderKey(group, user string) (key []byte, err error) {
	err = s.db.QueryRow(getSenderKeyQuery, s.JID, group, user).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return
}
