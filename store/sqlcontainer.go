// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	mathRand "math/rand"

	"go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/util/keys"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type SQLContainer struct {
	db      *sql.DB
	dialect string
	log     waLog.Logger
}

var EnableSQLiteForeignKeys = true

func NewSQLContainer(dialect, address string, log waLog.Logger) (*SQLContainer, error) {
	db, err := sql.Open(dialect, address)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	container := NewSQLContainerWithDB(db, dialect, log)
	err = container.Upgrade()
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade database: %w", err)
	}
	return container, nil
}

func NewSQLContainerWithDB(db *sql.DB, dialect string, log waLog.Logger) *SQLContainer {
	if EnableSQLiteForeignKeys && dialect == "sqlite3" {
		_, _ = db.Exec("PRAGMA foreign_keys = ON;")
	}
	return &SQLContainer{
		db:      db,
		dialect: dialect,
		log:     log,
	}
}

const getAllDevicesQuery = `
SELECT jid, registration_id, noise_key, identity_key,
       signed_pre_key, signed_pre_key_id, signed_pre_key_sig,
       adv_key, adv_details, adv_account_sig, adv_device_sig,
       platform, business_name
FROM whatsmeow_device
`

const getDeviceQuery = getAllDevicesQuery + " WHERE jid=$1"

type scannable interface {
	Scan(dest ...interface{}) error
}

func (c *SQLContainer) scanDevice(row scannable) (*Device, error) {
	var store Device
	store.Log = c.log
	store.SignedPreKey = &keys.PreKey{}
	var jid string
	var noisePriv, identityPriv, preKeyPriv, preKeySig []byte
	var account waProto.ADVSignedDeviceIdentity

	err := row.Scan(
		&jid, &store.RegistrationID, &noisePriv, &identityPriv,
		&preKeyPriv, &store.SignedPreKey.KeyID, &preKeySig,
		&store.AdvSecretKey, &account.Details, &account.AccountSignature, &account.DeviceSignature,
		&store.Platform, &store.BusinessName)
	if err != nil {
		return nil, fmt.Errorf("failed to scan session: %w", err)
	} else if len(noisePriv) != 32 || len(identityPriv) != 32 || len(preKeyPriv) != 32 || len(preKeySig) != 64 {
		return nil, ErrInvalidLength
	}

	store.NoiseKey = keys.NewKeyPairFromPrivateKey(*(*[32]byte)(noisePriv))
	store.IdentityKey = keys.NewKeyPairFromPrivateKey(*(*[32]byte)(identityPriv))
	store.SignedPreKey.KeyPair = *keys.NewKeyPairFromPrivateKey(*(*[32]byte)(preKeyPriv))
	store.SignedPreKey.Signature = (*[64]byte)(preKeySig)

	jidVal, err := binary.ParseJID(jid)
	if err != nil {
		return nil, fmt.Errorf("invalid JID in database: %w", err)
	}
	store.ID = &jidVal

	innerStore := &SQLStore{SQLContainer: c, JID: jid}
	store.Identities = innerStore
	store.Sessions = innerStore
	store.PreKeys = innerStore
	store.SenderKeys = innerStore
	store.AppStateKeys = innerStore
	store.AppState = innerStore
	store.Container = c
	store.Initialized = true

	return &store, nil
}

func (c *SQLContainer) GetAllDevices() ([]*Device, error) {
	res, err := c.db.Query(getAllDevicesQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	sessions := make([]*Device, 0)
	for res.Next() {
		sess, scanErr := c.scanDevice(res)
		if scanErr != nil {
			return sessions, scanErr
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

func (c *SQLContainer) GetDevice(jid string) (*Device, error) {
	sess, err := c.scanDevice(c.db.QueryRow(getDeviceQuery, jid))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return sess, err
}

const (
	insertDeviceQuery = `
		INSERT INTO whatsmeow_device (jid, registration_id, noise_key, identity_key,
									  signed_pre_key, signed_pre_key_id, signed_pre_key_sig,
									  adv_key, adv_details, adv_account_sig, adv_device_sig,
									  platform, business_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	deleteDeviceQuery = `DELETE FROM whatsmeow_device WHERE jid=$1`
)

func (c *SQLContainer) NewDevice() *Device {
	device := &Device{
		Log:       c.log,
		Container: c,

		NoiseKey:       keys.NewKeyPair(),
		IdentityKey:    keys.NewKeyPair(),
		RegistrationID: mathRand.Uint32(),
		AdvSecretKey:   make([]byte, 32),
	}
	_, err := rand.Read(device.AdvSecretKey)
	if err != nil {
		panic(err)
	}
	device.SignedPreKey = device.IdentityKey.CreateSignedPreKey(1)
	return device
}

var ErrDeviceIDMustBeSet = errors.New("device JID must be known before accessing database")

func (c *SQLContainer) PutDevice(store *Device) error {
	if store.ID == nil {
		return ErrDeviceIDMustBeSet
	}
	_, err := c.db.Exec(insertDeviceQuery,
		store.ID.String(), store.RegistrationID, store.NoiseKey.Priv[:], store.IdentityKey.Priv[:],
		store.SignedPreKey.Priv[:], store.SignedPreKey.KeyID, store.SignedPreKey.Signature[:],
		store.AdvSecretKey, store.Account.Details, store.Account.AccountSignature, store.Account.DeviceSignature,
		store.Platform, store.BusinessName)

	if !store.Initialized {
		innerStore := &SQLStore{SQLContainer: c, JID: store.ID.String()}
		store.Identities = innerStore
		store.Sessions = innerStore
		store.PreKeys = innerStore
		store.SenderKeys = innerStore
		store.AppStateKeys = innerStore
		store.AppState = innerStore
		store.Initialized = true
	}
	return err
}

func (c *SQLContainer) DeleteDevice(store *Device) error {
	if store.ID == nil {
		return ErrDeviceIDMustBeSet
	}
	_, err := c.db.Exec(deleteDeviceQuery, store.ID.String())
	return err
}
