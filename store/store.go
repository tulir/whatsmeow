// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/keys"
	waLog "go.mau.fi/whatsmeow/log"
)

type IdentityStore interface {
	PutIdentity(address string, key [32]byte) error
	IsTrustedIdentity(address string, key [32]byte) (bool, error)
}

type SessionStore interface {
	GetSession(address string) ([]byte, error)
	HasSession(address string) (bool, error)
	PutSession(address string, session []byte) error
}

type PreKeyStore interface {
	GetOrGenPreKeys(count uint32) ([]*keys.PreKey, error)
	GenOnePreKey() (*keys.PreKey, error)
	GetPreKey(id uint32) (*keys.PreKey, error)
	RemovePreKey(id uint32) error
	MarkPreKeysAsUploaded(upToID uint32) error
	UploadedPreKeyCount() (int, error)
}

type SenderKeyStore interface {
	PutSenderKey(group, user string, session []byte) error
	GetSenderKey(group, user string) ([]byte, error)
}

type DeviceContainer interface {
	PutDevice(store *Device) error
	DeleteDevice(store *Device) error
}

type Device struct {
	Log waLog.Logger

	NoiseKey       *keys.KeyPair
	IdentityKey    *keys.KeyPair
	SignedPreKey   *keys.PreKey
	Account        *waProto.ADVSignedDeviceIdentity
	RegistrationID uint32
	AdvSecretKey   []byte

	Platform     string
	BusinessName string
	ID           *waBinary.FullJID

	Initialized bool
	Identities  IdentityStore
	Sessions    SessionStore
	PreKeys     PreKeyStore
	SenderKeys  SenderKeyStore
	Container   DeviceContainer
}

func (device *Device) Save() error {
	return device.Container.PutDevice(device)
}

func (device *Device) Delete() error {
	return device.Container.DeleteDevice(device)
}
