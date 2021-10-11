// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package session

import (
	"fmt"

	"github.com/RadicalApp/libsignal-protocol-go/ecc"
	groupRecord "github.com/RadicalApp/libsignal-protocol-go/groups/state/record"
	"github.com/RadicalApp/libsignal-protocol-go/keys/identity"
	"github.com/RadicalApp/libsignal-protocol-go/protocol"
	"github.com/RadicalApp/libsignal-protocol-go/state/record"
	"github.com/RadicalApp/libsignal-protocol-go/state/store"
)

var _ store.SignalProtocol = (*Session)(nil)

func (sess *Session) GetIdentityKeyPair() *identity.KeyPair {
	return identity.NewKeyPair(
		identity.NewKey(ecc.NewDjbECPublicKey(*sess.IdentityKey.Pub)),
		ecc.NewDjbECPrivateKey(*sess.IdentityKey.Priv),
	)
}

func (sess *Session) GetLocalRegistrationId() uint32 {
	return uint32(sess.RegistrationID)
}

func (sess *Session) SaveIdentity(address *protocol.SignalAddress, identityKey *identity.Key) {
	sess.identityKeysLock.Lock()
	sess.IdentityKeys[address.String()] = identityKey.PublicKey().PublicKey()
	sess.identityKeysLock.Unlock()
}

func (sess *Session) IsTrustedIdentity(address *protocol.SignalAddress, identityKey *identity.Key) bool {
	// TODO implement properly
	return true
}

func (sess *Session) LoadPreKey(preKeyID uint32) *record.PreKey {
	key := sess.GetPreKey(preKeyID)
	return record.NewPreKey(key.KeyID, ecc.NewECKeyPair(
		ecc.NewDjbECPublicKey(*key.Pub),
		ecc.NewDjbECPrivateKey(*key.Priv),
	), nil)
}

func (sess *Session) StorePreKey(preKeyID uint32, preKeyRecord *record.PreKey) {
	panic("implement me")
}

func (sess *Session) ContainsPreKey(preKeyID uint32) bool {
	return sess.GetPreKey(preKeyID) != nil
}

func (sess *Session) LoadSession(address *protocol.SignalAddress) (signalSess *record.Session) {
	sess.sessionsLock.Lock()
	signalSessStruct, ok := sess.Sessions[address.String()]
	sess.sessionsLock.Unlock()
	if !ok {
		return record.NewSession(nil, nil)
	} else {
		var err error
		signalSess, err = record.NewSessionFromStructure(signalSessStruct, nil, nil)
		if err != nil {
			fmt.Println("Error in LoadSession:", err)
		}
	}
	return
}

func (sess *Session) GetSubDeviceSessions(name string) []uint32 {
	panic("implement me")
}

func (sess *Session) StoreSession(remoteAddress *protocol.SignalAddress, record *record.Session) {
	sess.sessionsLock.Lock()
	sess.Sessions[remoteAddress.String()] = record.Structure()
	sess.sessionsLock.Unlock()
}

func (sess *Session) ContainsSession(remoteAddress *protocol.SignalAddress) bool {
	sess.sessionsLock.Lock()
	_, ok := sess.Sessions[remoteAddress.String()]
	sess.sessionsLock.Unlock()
	return ok
}

func (sess *Session) DeleteSession(remoteAddress *protocol.SignalAddress) {
	sess.sessionsLock.Lock()
	delete(sess.Sessions, remoteAddress.String())
	sess.sessionsLock.Unlock()
}

func (sess *Session) DeleteAllSessions() {
	panic("implement me")
}

func (sess *Session) LoadSignedPreKey(signedPreKeyID uint32) *record.SignedPreKey {
	//fmt.Println("LoadSignedPreKey(", signedPreKeyID, ")")
	if signedPreKeyID == 1 {
		return record.NewSignedPreKey(signedPreKeyID, 0, ecc.NewECKeyPair(
			ecc.NewDjbECPublicKey(*sess.SignedPreKey.Pub),
			ecc.NewDjbECPrivateKey(*sess.SignedPreKey.Priv),
		), *sess.SignedPreKey.Signature, nil)
	} else {
		panic("Invalid signed prekey ID")
		//return record.NewSignedPreKey(signedPreKeyID, 0, sess.LoadPreKey(signedPreKeyID).KeyPair(), [64]byte{}, nil)
	}
}

func (sess *Session) LoadSignedPreKeys() []*record.SignedPreKey {
	panic("implement me")
}

func (sess *Session) StoreSignedPreKey(signedPreKeyID uint32, record *record.SignedPreKey) {
	panic("implement me")
}

func (sess *Session) ContainsSignedPreKey(signedPreKeyID uint32) bool {
	panic("implement me")
}

func (sess *Session) RemoveSignedPreKey(signedPreKeyID uint32) {
	panic("implement me")
}

func (sess *Session) StoreSenderKey(senderKeyName *protocol.SenderKeyName, keyRecord *groupRecord.SenderKey) {
	sess.senderKeysLock.Lock()
	groupMap, ok := sess.SenderKeys[senderKeyName.GroupID()]
	if !ok {
		groupMap = make(map[string]*groupRecord.SenderKeyStructure)
		sess.SenderKeys[senderKeyName.GroupID()] = groupMap
	}
	groupMap[senderKeyName.Sender().String()] = keyRecord.Structure()
	sess.senderKeysLock.Unlock()
}

func (sess *Session) LoadSenderKey(senderKeyName *protocol.SenderKeyName) *groupRecord.SenderKey {
	sess.senderKeysLock.Lock()
	defer sess.senderKeysLock.Unlock()
	groupMap, ok := sess.SenderKeys[senderKeyName.GroupID()]
	if !ok {
		groupMap = make(map[string]*groupRecord.SenderKeyStructure)
		sess.SenderKeys[senderKeyName.GroupID()] = groupMap
	}
	senderKeyStruct, ok := groupMap[senderKeyName.Sender().String()]
	if !ok {
		return groupRecord.NewSenderKey(nil, nil)
	}
	senderKey, _ := groupRecord.NewSenderKeyFromStruct(senderKeyStruct, nil, nil)
	return senderKey
}
