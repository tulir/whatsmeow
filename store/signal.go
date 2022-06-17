// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"go.mau.fi/libsignal/ecc"
	groupRecord "go.mau.fi/libsignal/groups/state/record"
	"go.mau.fi/libsignal/keys/identity"
	"go.mau.fi/libsignal/protocol"
	"go.mau.fi/libsignal/serialize"
	"go.mau.fi/libsignal/state/record"
	"go.mau.fi/libsignal/state/store"

	"go.mau.fi/whatsmeow/util/keys"
)

var SignalProtobufSerializer = serialize.NewProtoBufSerializer()

var _ store.SignalProtocol = (*Device)(nil)

func (device *Device) GetIdentityKeyPair() *identity.KeyPair {
	return identity.NewKeyPair(
		identity.NewKey(ecc.NewDjbECPublicKey(*device.IdentityKey.Pub)),
		ecc.NewDjbECPrivateKey(*device.IdentityKey.Priv),
	)
}

func (device *Device) GetLocalRegistrationId() uint32 {
	return device.RegistrationID
}

func (device *Device) SaveIdentity(address *protocol.SignalAddress, identityKey *identity.Key) {
	for i := 0; ; i++ {
		err := device.Identities.PutIdentity(address.String(), identityKey.PublicKey().PublicKey())
		if err == nil || !device.handleDatabaseError(i, err, "save identity of %s", address.String()) {
			break
		}
	}
}

func (device *Device) IsTrustedIdentity(address *protocol.SignalAddress, identityKey *identity.Key) bool {
	for i := 0; ; i++ {
		isTrusted, err := device.Identities.IsTrustedIdentity(address.String(), identityKey.PublicKey().PublicKey())
		if err == nil || !device.handleDatabaseError(i, err, "check if %s's identity is trusted", address.String()) {
			return isTrusted
		}
	}
}

func (device *Device) LoadPreKey(id uint32) *record.PreKey {
	var preKey *keys.PreKey
	for i := 0; ; i++ {
		var err error
		preKey, err = device.PreKeys.GetPreKey(id)
		if err == nil || !device.handleDatabaseError(i, err, "load prekey %d", id) {
			break
		}
	}
	if preKey == nil {
		return nil
	}
	return record.NewPreKey(preKey.KeyID, ecc.NewECKeyPair(
		ecc.NewDjbECPublicKey(*preKey.Pub),
		ecc.NewDjbECPrivateKey(*preKey.Priv),
	), nil)
}

func (device *Device) RemovePreKey(id uint32) {
	for i := 0; ; i++ {
		err := device.PreKeys.RemovePreKey(id)
		if err == nil || !device.handleDatabaseError(i, err, "remove prekey %d", id) {
			break
		}
	}
}

func (device *Device) StorePreKey(preKeyID uint32, preKeyRecord *record.PreKey) {
	panic("not implemented")
}

func (device *Device) ContainsPreKey(preKeyID uint32) bool {
	panic("not implemented")
}

func (device *Device) LoadSession(address *protocol.SignalAddress) *record.Session {
	var rawSess []byte
	for i := 0; ; i++ {
		var err error
		rawSess, err = device.Sessions.GetSession(address.String())
		if err == nil || !device.handleDatabaseError(i, err, "load session with %s", address.String()) {
			break
		}
	}
	if rawSess == nil {
		return record.NewSession(SignalProtobufSerializer.Session, SignalProtobufSerializer.State)
	}
	sess, err := record.NewSessionFromBytes(rawSess, SignalProtobufSerializer.Session, SignalProtobufSerializer.State)
	if err != nil {
		device.Log.Errorf("Failed to deserialize session with %s: %v", address.String(), err)
		return record.NewSession(SignalProtobufSerializer.Session, SignalProtobufSerializer.State)
	}
	return sess
}

func (device *Device) GetSubDeviceSessions(name string) []uint32 {
	panic("not implemented")
}

func (device *Device) StoreSession(address *protocol.SignalAddress, record *record.Session) {
	for i := 0; ; i++ {
		err := device.Sessions.PutSession(address.String(), record.Serialize())
		if err == nil || !device.handleDatabaseError(i, err, "store session with %s", address.String()) {
			return
		}
	}
}

func (device *Device) ContainsSession(remoteAddress *protocol.SignalAddress) bool {
	for i := 0; ; i++ {
		hasSession, err := device.Sessions.HasSession(remoteAddress.String())
		if err == nil || !device.handleDatabaseError(i, err, "store has session for %s", remoteAddress.String()) {
			return hasSession
		}
	}
}

func (device *Device) DeleteSession(remoteAddress *protocol.SignalAddress) {
	panic("not implemented")
}

func (device *Device) DeleteAllSessions() {
	panic("not implemented")
}

func (device *Device) LoadSignedPreKey(signedPreKeyID uint32) *record.SignedPreKey {
	if signedPreKeyID == device.SignedPreKey.KeyID {
		return record.NewSignedPreKey(signedPreKeyID, 0, ecc.NewECKeyPair(
			ecc.NewDjbECPublicKey(*device.SignedPreKey.Pub),
			ecc.NewDjbECPrivateKey(*device.SignedPreKey.Priv),
		), *device.SignedPreKey.Signature, nil)
	}
	return nil
}

func (device *Device) LoadSignedPreKeys() []*record.SignedPreKey {
	panic("not implemented")
}

func (device *Device) StoreSignedPreKey(signedPreKeyID uint32, record *record.SignedPreKey) {
	panic("not implemented")
}

func (device *Device) ContainsSignedPreKey(signedPreKeyID uint32) bool {
	panic("not implemented")
}

func (device *Device) RemoveSignedPreKey(signedPreKeyID uint32) {
	panic("not implemented")
}

func (device *Device) StoreSenderKey(senderKeyName *protocol.SenderKeyName, keyRecord *groupRecord.SenderKey) {
	for i := 0; ; i++ {
		err := device.SenderKeys.PutSenderKey(senderKeyName.GroupID(), senderKeyName.Sender().String(), keyRecord.Serialize())
		if err == nil || !device.handleDatabaseError(i, err, "store sender key from %s", senderKeyName.Sender().String()) {
			return
		}
	}
}

func (device *Device) LoadSenderKey(senderKeyName *protocol.SenderKeyName) *groupRecord.SenderKey {
	var rawKey []byte
	for i := 0; ; i++ {
		var err error
		rawKey, err = device.SenderKeys.GetSenderKey(senderKeyName.GroupID(), senderKeyName.Sender().String())
		if err == nil || !device.handleDatabaseError(i, err, "load sender key from %s for %s", senderKeyName.Sender().String(), senderKeyName.GroupID()) {
			break
		}
	}
	if rawKey == nil {
		return groupRecord.NewSenderKey(SignalProtobufSerializer.SenderKeyRecord, SignalProtobufSerializer.SenderKeyState)
	}
	key, err := groupRecord.NewSenderKeyFromBytes(rawKey, SignalProtobufSerializer.SenderKeyRecord, SignalProtobufSerializer.SenderKeyState)
	if err != nil {
		device.Log.Errorf("Failed to deserialize sender key from %s for %s: %v", senderKeyName.Sender().String(), senderKeyName.GroupID(), err)
		return groupRecord.NewSenderKey(SignalProtobufSerializer.SenderKeyRecord, SignalProtobufSerializer.SenderKeyState)
	}
	return key
}
