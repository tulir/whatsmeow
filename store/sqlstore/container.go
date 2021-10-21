// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sqlstore

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	mathRand "math/rand"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/util/keys"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type Container struct {
	db      *sql.DB
	dialect string
	log     waLog.Logger
}

var _ store.DeviceContainer = (*Container)(nil)

func New(dialect, address string, log waLog.Logger) (*Container, error) {
	db, err := sql.Open(dialect, address)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	container := NewWithDB(db, dialect, log)
	err = container.Upgrade()
	if err != nil {
		return nil, fmt.Errorf("failed to upgrade database: %w", err)
	}
	return container, nil
}

func NewWithDB(db *sql.DB, dialect string, log waLog.Logger) *Container {
	if log == nil {
		log = waLog.Noop
	}
	return &Container{
		db:      db,
		dialect: dialect,
		log:     log,
	}
}

const getAllDevicesQuery = `
SELECT jid, registration_id, noise_key, identity_key,
       signed_pre_key, signed_pre_key_id, signed_pre_key_sig,
       adv_key, adv_details, adv_account_sig, adv_device_sig,
       platform, business_name, push_name
FROM whatsmeow_device
`

const getDeviceQuery = getAllDevicesQuery + " WHERE jid=$1"

type scannable interface {
	Scan(dest ...interface{}) error
}

func (c *Container) scanDevice(row scannable) (*store.Device, error) {
	var device store.Device
	device.Log = c.log
	device.SignedPreKey = &keys.PreKey{}
	var noisePriv, identityPriv, preKeyPriv, preKeySig []byte
	var account waProto.ADVSignedDeviceIdentity

	err := row.Scan(
		&device.ID, &device.RegistrationID, &noisePriv, &identityPriv,
		&preKeyPriv, &device.SignedPreKey.KeyID, &preKeySig,
		&device.AdvSecretKey, &account.Details, &account.AccountSignature, &account.DeviceSignature,
		&device.Platform, &device.BusinessName, &device.PushName)
	if err != nil {
		return nil, fmt.Errorf("failed to scan session: %w", err)
	} else if len(noisePriv) != 32 || len(identityPriv) != 32 || len(preKeyPriv) != 32 || len(preKeySig) != 64 {
		return nil, ErrInvalidLength
	}

	device.NoiseKey = keys.NewKeyPairFromPrivateKey(*(*[32]byte)(noisePriv))
	device.IdentityKey = keys.NewKeyPairFromPrivateKey(*(*[32]byte)(identityPriv))
	device.SignedPreKey.KeyPair = *keys.NewKeyPairFromPrivateKey(*(*[32]byte)(preKeyPriv))
	device.SignedPreKey.Signature = (*[64]byte)(preKeySig)

	innerStore := &SQLStore{Container: c, JID: device.ID.String()}
	device.Identities = innerStore
	device.Sessions = innerStore
	device.PreKeys = innerStore
	device.SenderKeys = innerStore
	device.AppStateKeys = innerStore
	device.AppState = innerStore
	device.Container = c
	device.Initialized = true

	return &device, nil
}

func (c *Container) GetAllDevices() ([]*store.Device, error) {
	res, err := c.db.Query(getAllDevicesQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	sessions := make([]*store.Device, 0)
	for res.Next() {
		sess, scanErr := c.scanDevice(res)
		if scanErr != nil {
			return sessions, scanErr
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

func (c *Container) GetDevice(jid types.JID) (*store.Device, error) {
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
									  platform, business_name, push_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (jid) DO UPDATE SET platform=$12, business_name=$13, push_name=$14
	`
	deleteDeviceQuery = `DELETE FROM whatsmeow_device WHERE jid=$1`
)

func (c *Container) NewDevice() *store.Device {
	device := &store.Device{
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

func (c *Container) PutDevice(device *store.Device) error {
	if device.ID == nil {
		return ErrDeviceIDMustBeSet
	}
	_, err := c.db.Exec(insertDeviceQuery,
		device.ID.String(), device.RegistrationID, device.NoiseKey.Priv[:], device.IdentityKey.Priv[:],
		device.SignedPreKey.Priv[:], device.SignedPreKey.KeyID, device.SignedPreKey.Signature[:],
		device.AdvSecretKey, device.Account.Details, device.Account.AccountSignature, device.Account.DeviceSignature,
		device.Platform, device.BusinessName, device.PushName)

	if !device.Initialized {
		innerStore := &SQLStore{Container: c, JID: device.ID.String()}
		device.Identities = innerStore
		device.Sessions = innerStore
		device.PreKeys = innerStore
		device.SenderKeys = innerStore
		device.AppStateKeys = innerStore
		device.AppState = innerStore
		device.Initialized = true
	}
	return err
}

func (c *Container) DeleteDevice(store *store.Device) error {
	if store.ID == nil {
		return ErrDeviceIDMustBeSet
	}
	_, err := c.db.Exec(deleteDeviceQuery, store.ID.String())
	return err
}
