// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package store contains interfaces for storing data needed for WhatsApp multidevice.
package store

import (
	waProto "go.mau.fi/whatsmeow/binary/proto"
	waBinary "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/util/keys"
	waLog "go.mau.fi/whatsmeow/util/log"
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

type AppStateSyncKey struct {
	Data        []byte
	Fingerprint []byte
	Timestamp   int64
}

type AppStateSyncKeyStore interface {
	PutAppStateSyncKey(id []byte, key AppStateSyncKey) error
	GetAppStateSyncKey(id []byte) (*AppStateSyncKey, error)
}

type AppStateMutationMAC struct {
	IndexMAC []byte
	ValueMAC []byte
}

type AppStateStore interface {
	PutAppStateVersion(name string, version uint64, hash [128]byte) error
	GetAppStateVersion(name string) (uint64, [128]byte, error)
	DeleteAppStateVersion(name string) error

	PutAppStateMutationMACs(name string, version uint64, mutations []AppStateMutationMAC) error
	DeleteAppStateMutationMACs(name string, indexMACs [][]byte) error
	GetAppStateMutationMAC(name string, indexMAC []byte) (valueMAC []byte, err error)
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
	RegistrationID uint32
	AdvSecretKey   []byte

	ID           *waBinary.JID
	Account      *waProto.ADVSignedDeviceIdentity
	Platform     string
	BusinessName string

	Initialized  bool
	Identities   IdentityStore
	Sessions     SessionStore
	PreKeys      PreKeyStore
	SenderKeys   SenderKeyStore
	AppStateKeys AppStateSyncKeyStore
	AppState     AppStateStore
	Container    DeviceContainer
}

func (device *Device) Save() error {
	return device.Container.PutDevice(device)
}

func (device *Device) Delete() error {
	return device.Container.DeleteDevice(device)
}
