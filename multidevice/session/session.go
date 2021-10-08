// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package session

import (
	"sync"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/multidevice/keys"
)

type Session struct {
	NoiseKey       *keys.KeyPair
	IdentityKey    *keys.KeyPair
	SignedPreKey   *keys.PreKey
	RegistrationID uint16
	AdvSecretKey   []byte

	IdentityKeys map[waBinary.FullJID][32]byte

	PreKeys           map[uint32]*keys.PreKey
	preKeysLock       sync.Mutex
	FirstUnuploadedID uint32
	NextPreKeyID      uint32

	Platform     string
	BusinessName string
	ID           *waBinary.FullJID
}

func NewSession() *Session {
	return &Session{
		IdentityKeys:      map[waBinary.FullJID][32]byte{},
		PreKeys:           make(map[uint32]*keys.PreKey),
		FirstUnuploadedID: 2,
		NextPreKeyID:      2,
	}
}

func (sess *Session) PutIdentity(jid *waBinary.FullJID, key [32]byte) {
	sess.IdentityKeys[*jid] = key
}

func (sess *Session) unlockedGetPreKeys(count uint32) []*keys.PreKey {
	foundKeys := make([]*keys.PreKey, 0, count)
	for _, key := range sess.PreKeys {
		if key.KeyID >= sess.FirstUnuploadedID {
			foundKeys = append(foundKeys, key)
			if uint32(len(foundKeys)) >= count {
				break
			}
		}
	}
	return foundKeys
}

func (sess *Session) GetPreKeys(count uint32) []*keys.PreKey {
	sess.preKeysLock.Lock()
	defer sess.preKeysLock.Unlock()
	return sess.unlockedGetPreKeys(count)
}

func (sess *Session) GetPreKey(id uint32) *keys.PreKey {
	sess.preKeysLock.Lock()
	defer sess.preKeysLock.Unlock()
	return sess.PreKeys[id]
}

func (sess *Session) RemovePreKey(id uint32) {
	sess.preKeysLock.Lock()
	delete(sess.PreKeys, id)
	sess.preKeysLock.Unlock()
}

func (sess *Session) GetOrGenPreKeys(count uint32) []*keys.PreKey {
	sess.preKeysLock.Lock()
	defer sess.preKeysLock.Unlock()
	for ; sess.NextPreKeyID < sess.FirstUnuploadedID+count; sess.NextPreKeyID++ {
		sess.PreKeys[sess.NextPreKeyID] = keys.NewPreKey(sess.NextPreKeyID)
	}
	return sess.unlockedGetPreKeys(count)
}

func (sess *Session) ServerHasPreKeys() bool {
	return sess.FirstUnuploadedID > 2
}

func (sess *Session) MarkPreKeysAsUploaded(upTo uint32) {
	sess.preKeysLock.Lock()
	sess.FirstUnuploadedID = upTo + 1
	sess.preKeysLock.Unlock()
}
