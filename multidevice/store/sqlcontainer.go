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

	"go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/multidevice/keys"
	waLog "go.mau.fi/whatsmeow/multidevice/log"
)

type SQLContainer struct {
	db      *sql.DB
	dialect string
	log     waLog.Logger
}

var EnableSQLiteForeignKeys = true

func NewSQLContainer(db *sql.DB, dialect string, log waLog.Logger) *SQLContainer {
	if EnableSQLiteForeignKeys && dialect == "sqlite3" {
		_, _ = db.Exec("PRAGMA foreign_keys = ON;")
	}
	return &SQLContainer{
		db:      db,
		dialect: dialect,
		log:     log,
	}
}

type upgradeFunc func(*sql.Tx, *SQLContainer) error

var Upgrades = [...]upgradeFunc{
	func(tx *sql.Tx, _ *SQLContainer) error {
		_, err := tx.Exec(`CREATE TABLE whatsmeow_device (
    		jid TEXT PRIMARY KEY,

    		registration_id INTEGER NOT NULL,

    		noise_key    bytea NOT NULL CHECK ( length(noise_key) = 32 ),
    		identity_key bytea NOT NULL CHECK ( length(identity_key) = 32 ),

    		signed_pre_key     bytea   NOT NULL CHECK ( length(signed_pre_key) = 32 ),
    		signed_pre_key_id  INTEGER NOT NULL,
    		signed_pre_key_sig bytea   NOT NULL CHECK ( length(signed_pre_key_sig) = 64 ),

    		adv_key         bytea NOT NULL,
    		adv_details     bytea NOT NULL,
    		adv_account_sig bytea NOT NULL CHECK ( length(adv_account_sig) = 64 ),
    		adv_device_sig  bytea NOT NULL CHECK ( length(adv_device_sig) = 64 ),

    		platform      TEXT NOT NULL DEFAULT '',
    		business_name TEXT NOT NULL DEFAULT ''
		)`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`CREATE TABLE whatsmeow_identity_keys (
    		our_jid  TEXT,
    		their_id TEXT,
    		identity bytea NOT NULL CHECK ( length(identity) = 32 ),

    		PRIMARY KEY (our_jid, their_id),
    		FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
		)`)
		_, err = tx.Exec(`CREATE TABLE whatsmeow_pre_keys (
    		jid      TEXT,
    		key_id   INTEGER          CHECK ( key_id > 0 AND key_id < 16777216 ),
    		key      bytea   NOT NULL CHECK ( length(key) = 32 ),
    		uploaded BOOLEAN NOT NULL,

    		PRIMARY KEY (jid, key_id),
    		FOREIGN KEY (jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
		)`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`CREATE TABLE whatsmeow_sessions (
    		our_jid  TEXT,
    		their_id TEXT,
    		session  bytea,

    		PRIMARY KEY (our_jid, their_id),
    		FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
		)`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`CREATE TABLE whatsmeow_sender_keys (
    		our_jid    TEXT,
    		chat_id    TEXT,
    		sender_id  TEXT,
    		sender_key bytea NOT NULL,

    		PRIMARY KEY (our_jid, chat_id, sender_id),
    		FOREIGN KEY (our_jid) REFERENCES whatsmeow_device(jid) ON DELETE CASCADE ON UPDATE CASCADE
		)`)
		if err != nil {
			return err
		}
		return nil
	},
}

func (c *SQLContainer) getVersion() (int, error) {
	_, err := c.db.Exec("CREATE TABLE IF NOT EXISTS whatsmeow_version (version INTEGER)")
	if err != nil {
		return -1, err
	}

	version := 0
	row := c.db.QueryRow("SELECT version FROM whatsmeow_version LIMIT 1")
	if row != nil {
		_ = row.Scan(&version)
	}
	return version, nil
}

func (c *SQLContainer) setVersion(tx *sql.Tx, version int) error {
	_, err := tx.Exec("DELETE FROM whatsmeow_version")
	if err != nil {
		return err
	}
	_, err = tx.Exec("INSERT INTO whatsmeow_version (version) VALUES ($1)", version)
	return err
}

// Upgrade upgrades the database from the current to the latest version available.
func (c *SQLContainer) Upgrade() error {
	version, err := c.getVersion()
	if err != nil {
		return err
	}

	for ; version < len(Upgrades); version++ {
		var tx *sql.Tx
		tx, err = c.db.Begin()
		if err != nil {
			return err
		}

		migrateFunc := Upgrades[version]
		err = migrateFunc(tx, c)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		if err = c.setVersion(tx, version+1); err != nil {
			return err
		}

		if err = tx.Commit(); err != nil {
			return err
		}
	}

	return nil
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
	return &Device{
		Log:       c.log,
		Container: c,
	}
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
