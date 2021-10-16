// Copyright (c) 2021 Tulir Asokan
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
	err := device.Identities.PutIdentity(address.String(), identityKey.PublicKey().PublicKey())
	if err != nil {
		device.Log.Errorf("Failed to save identity of %s: %v", address.String(), err)
	}
}

func (device *Device) IsTrustedIdentity(address *protocol.SignalAddress, identityKey *identity.Key) bool {
	isTrusted, err := device.Identities.IsTrustedIdentity(address.String(), identityKey.PublicKey().PublicKey())
	if err != nil {
		device.Log.Errorf("Failed to check if %s's identity is trusted: %v", address.String(), err)
	}
	return isTrusted
}

func (device *Device) LoadPreKey(id uint32) *record.PreKey {
	preKey, err := device.PreKeys.GetPreKey(id)
	if err != nil {
		device.Log.Errorf("Failed to load prekey %d: %v", id, err)
		return nil
	} else if preKey == nil {
		return nil
	}
	return record.NewPreKey(preKey.KeyID, ecc.NewECKeyPair(
		ecc.NewDjbECPublicKey(*preKey.Pub),
		ecc.NewDjbECPrivateKey(*preKey.Priv),
	), nil)
}

func (device *Device) RemovePreKey(id uint32) {
	err := device.PreKeys.RemovePreKey(id)
	if err != nil {
		device.Log.Errorf("Failed to remove prekey %d: %v", id, err)
	}
}

func (device *Device) StorePreKey(preKeyID uint32, preKeyRecord *record.PreKey) {
	panic("not implemented")
}

func (device *Device) ContainsPreKey(preKeyID uint32) bool {
	panic("not implemented")
}

func (device *Device) LoadSession(address *protocol.SignalAddress) *record.Session {
	rawSess, err := device.Sessions.GetSession(address.String())
	if err != nil {
		device.Log.Errorf("Failed to load session with %s: %v", address.String(), err)
		return record.NewSession(SignalProtobufSerializer.Session, SignalProtobufSerializer.State)
	} else if rawSess == nil {
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
	err := device.Sessions.PutSession(address.String(), record.Serialize())
	if err != nil {
		device.Log.Errorf("Failed to store session with %s: %v", address.String(), err)
	}
}

func (device *Device) ContainsSession(remoteAddress *protocol.SignalAddress) bool {
	hasSession, err := device.Sessions.HasSession(remoteAddress.String())
	if err != nil {
		device.Log.Warnf("Failed to check if store has session for %s: %v", remoteAddress.String(), err)
	}
	return hasSession
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
	err := device.SenderKeys.PutSenderKey(senderKeyName.GroupID(), senderKeyName.Sender().String(), keyRecord.Serialize())
	if err != nil {
		device.Log.Errorf("Failed to store sender key from %s for %s: %v", senderKeyName.Sender().String(), senderKeyName.GroupID(), err)
	}
}

func (device *Device) LoadSenderKey(senderKeyName *protocol.SenderKeyName) *groupRecord.SenderKey {
	rawKey, err := device.SenderKeys.GetSenderKey(senderKeyName.GroupID(), senderKeyName.Sender().String())
	if err != nil {
		device.Log.Errorf("Failed to load sender key from %s for %s: %v", senderKeyName.Sender().String(), senderKeyName.GroupID(), err)
		return groupRecord.NewSenderKey(SignalProtobufSerializer.SenderKeyRecord, SignalProtobufSerializer.SenderKeyState)
	} else if rawKey == nil {
		return groupRecord.NewSenderKey(SignalProtobufSerializer.SenderKeyRecord, SignalProtobufSerializer.SenderKeyState)
	}
	key, err := groupRecord.NewSenderKeyFromBytes(rawKey, SignalProtobufSerializer.SenderKeyRecord, SignalProtobufSerializer.SenderKeyState)
	if err != nil {
		device.Log.Errorf("Failed to deserialize sender key from %s for %s: %v", senderKeyName.Sender().String(), senderKeyName.GroupID(), err)
		return groupRecord.NewSenderKey(SignalProtobufSerializer.SenderKeyRecord, SignalProtobufSerializer.SenderKeyState)
	}
	return key
}
