// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sqlstore

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"math/rand/v2"

	"go.mau.fi/util/random"

	"github.com/snaril/whatsmeow/store"
	"github.com/snaril/whatsmeow/util/keys"
	waLog "github.com/snaril/whatsmeow/util/log"
)

var Container *container

func init() {
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	Container = &container{
		log: dbLog,
	}
}

type container struct {
	ctx                  context.Context
	db                   gdb.DB
	log                  waLog.Logger
	DatabaseErrorHandler func(device *store.Device, action string, attemptIndex int, err error) (retry bool)
}

func (c *container) NewDevice() *store.Device {
	device := &store.Device{
		Log:       c.log,
		Container: c,

		DatabaseErrorHandler: c.DatabaseErrorHandler,

		NoiseKey:       keys.NewKeyPair(),
		IdentityKey:    keys.NewKeyPair(),
		RegistrationID: rand.Uint32(),
		AdvSecretKey:   random.Bytes(32),
	}
	device.SignedPreKey = device.IdentityKey.CreateSignedPreKey(1)
	return device
}

func New(ctx context.Context, log waLog.Logger) (*container, error) {
	db := g.DB()
	container := &container{
		ctx:                  ctx,
		db:                   db,
		log:                  log,
		DatabaseErrorHandler: nil,
	}
	return container, nil
}

func (c *container) GetAllDevices() ([]*store.Device, error) {

	return nil, nil
}

func (c *container) GetFirstDevice() (*store.Device, error) {
	devices, err := c.GetAllDevices()
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return c.NewDevice(), nil
	} else {
		return devices[0], nil
	}
}

func (c *container) Close() error {
	return nil
}

func (c *container) PutDevice(device *store.Device) error {

	return nil
}

func (c *container) DeleteDevice(store *store.Device) error {

	return nil
}
