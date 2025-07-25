// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"context"
	"fmt"

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

func (device *Device) GetLocalRegistrationID() uint32 {
	return device.RegistrationID
}

func (device *Device) SaveIdentity(ctx context.Context, address *protocol.SignalAddress, identityKey *identity.Key) error {
	addrString := address.String()
	err := device.Identities.PutIdentity(ctx, addrString, identityKey.PublicKey().PublicKey())
	if err != nil {
		return fmt.Errorf("failed to save identity of %s: %w", addrString, err)
	}
	return nil
}

func (device *Device) IsTrustedIdentity(ctx context.Context, address *protocol.SignalAddress, identityKey *identity.Key) (bool, error) {
	addrString := address.String()
	isTrusted, err := device.Identities.IsTrustedIdentity(ctx, addrString, identityKey.PublicKey().PublicKey())
	if err != nil {
		return false, fmt.Errorf("failed to check if %s's identity is trusted: %w", addrString, err)
	}
	return isTrusted, nil
}

func (device *Device) LoadPreKey(ctx context.Context, id uint32) (*record.PreKey, error) {
	preKey, err := device.PreKeys.GetPreKey(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load prekey %d: %w", id, err)
	}
	if preKey == nil {
		return nil, nil
	}
	return record.NewPreKey(preKey.KeyID, ecc.NewECKeyPair(
		ecc.NewDjbECPublicKey(*preKey.Pub),
		ecc.NewDjbECPrivateKey(*preKey.Priv),
	), nil), nil
}

func (device *Device) RemovePreKey(ctx context.Context, id uint32) error {
	err := device.PreKeys.RemovePreKey(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to remove prekey %d: %w", id, err)
	}
	return nil
}

func (device *Device) StorePreKey(ctx context.Context, preKeyID uint32, preKeyRecord *record.PreKey) error {
	panic("not implemented")
}

func (device *Device) ContainsPreKey(ctx context.Context, preKeyID uint32) (bool, error) {
	panic("not implemented")
}

func (device *Device) LoadSession(ctx context.Context, address *protocol.SignalAddress) (*record.Session, error) {
	addrString := address.String()
	rawSess, err := device.Sessions.GetSession(ctx, addrString)
	if err != nil {
		return nil, fmt.Errorf("failed to load session with %s: %w", addrString, err)
	}
	if rawSess == nil {
		return record.NewSession(SignalProtobufSerializer.Session, SignalProtobufSerializer.State), nil
	}
	sess, err := record.NewSessionFromBytes(rawSess, SignalProtobufSerializer.Session, SignalProtobufSerializer.State)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize session with %s: %w", addrString, err)
	}
	return sess, nil
}

func (device *Device) GetSubDeviceSessions(ctx context.Context, name string) ([]uint32, error) {
	panic("not implemented")
}

func (device *Device) StoreSession(ctx context.Context, address *protocol.SignalAddress, record *record.Session) error {
	addrString := address.String()
	err := device.Sessions.PutSession(ctx, addrString, record.Serialize())
	if err != nil {
		return fmt.Errorf("failed to store session with %s: %w", addrString, err)
	}
	return nil
}

func (device *Device) ContainsSession(ctx context.Context, remoteAddress *protocol.SignalAddress) (bool, error) {
	addrString := remoteAddress.String()
	hasSession, err := device.Sessions.HasSession(ctx, addrString)
	if err != nil {
		return false, fmt.Errorf("failed to check if store has session for %s: %w", addrString, err)
	}
	return hasSession, nil
}

func (device *Device) DeleteSession(ctx context.Context, remoteAddress *protocol.SignalAddress) error {
	panic("not implemented")
}

func (device *Device) DeleteAllSessions(ctx context.Context) error {
	panic("not implemented")
}

func (device *Device) LoadSignedPreKey(ctx context.Context, signedPreKeyID uint32) (*record.SignedPreKey, error) {
	if signedPreKeyID == device.SignedPreKey.KeyID {
		return record.NewSignedPreKey(signedPreKeyID, 0, ecc.NewECKeyPair(
			ecc.NewDjbECPublicKey(*device.SignedPreKey.Pub),
			ecc.NewDjbECPrivateKey(*device.SignedPreKey.Priv),
		), *device.SignedPreKey.Signature, nil), nil
	}
	return nil, nil
}

func (device *Device) LoadSignedPreKeys(ctx context.Context) ([]*record.SignedPreKey, error) {
	panic("not implemented")
}

func (device *Device) StoreSignedPreKey(ctx context.Context, signedPreKeyID uint32, record *record.SignedPreKey) error {
	panic("not implemented")
}

func (device *Device) ContainsSignedPreKey(ctx context.Context, signedPreKeyID uint32) (bool, error) {
	panic("not implemented")
}

func (device *Device) RemoveSignedPreKey(ctx context.Context, signedPreKeyID uint32) error {
	panic("not implemented")
}

func (device *Device) StoreSenderKey(ctx context.Context, senderKeyName *protocol.SenderKeyName, keyRecord *groupRecord.SenderKey) error {
	groupID := senderKeyName.GroupID()
	senderString := senderKeyName.Sender().String()
	err := device.SenderKeys.PutSenderKey(ctx, groupID, senderString, keyRecord.Serialize())
	if err != nil {
		return fmt.Errorf("failed to store sender key from %s for %s: %w", senderString, groupID, err)
	}
	return nil
}

func (device *Device) LoadSenderKey(ctx context.Context, senderKeyName *protocol.SenderKeyName) (*groupRecord.SenderKey, error) {
	groupID := senderKeyName.GroupID()
	senderString := senderKeyName.Sender().String()
	rawKey, err := device.SenderKeys.GetSenderKey(ctx, groupID, senderString)
	if err != nil {
		return nil, fmt.Errorf("failed to load sender key from %s for %s: %w", senderString, groupID, err)
	}
	if rawKey == nil {
		return groupRecord.NewSenderKey(SignalProtobufSerializer.SenderKeyRecord, SignalProtobufSerializer.SenderKeyState), nil
	}
	key, err := groupRecord.NewSenderKeyFromBytes(rawKey, SignalProtobufSerializer.SenderKeyRecord, SignalProtobufSerializer.SenderKeyState)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize sender key from %s for %s: %w", senderString, groupID, err)
	}
	return key, nil
}
