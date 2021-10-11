// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package session

import (
	"sort"
	"sync"

	groupRecord "github.com/RadicalApp/libsignal-protocol-go/groups/state/record"
	"github.com/RadicalApp/libsignal-protocol-go/state/record"
	waProto "go.mau.fi/whatsmeow/binary/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/multidevice/keys"
)

type Session struct {
	NoiseKey       *keys.KeyPair
	IdentityKey    *keys.KeyPair
	SignedPreKey   *keys.PreKey
	Account        *waProto.ADVSignedDeviceIdentity
	RegistrationID uint16
	AdvSecretKey   []byte

	IdentityKeys     map[string][32]byte
	identityKeysLock sync.Mutex

	Sessions     map[string]*record.SessionStructure
	sessionsLock sync.Mutex

	PreKeys           map[uint32]*keys.PreKey
	preKeysLock       sync.Mutex
	FirstUnuploadedID uint32
	NextPreKeyID      uint32

	SenderKeys     map[string]map[string]*groupRecord.SenderKeyStructure
	senderKeysLock sync.Mutex

	Platform     string
	BusinessName string
	ID           *waBinary.FullJID
}

func NewSession() *Session {
	return &Session{
		IdentityKeys:      map[string][32]byte{},
		PreKeys:           make(map[uint32]*keys.PreKey),
		Sessions:          make(map[string]*record.SessionStructure),
		SenderKeys:        make(map[string]map[string]*groupRecord.SenderKeyStructure),
		FirstUnuploadedID: 1,
		NextPreKeyID:      1,
	}
}

func (sess *Session) PutIdentity(jid waBinary.FullJID, key [32]byte) {
	sess.identityKeysLock.Lock()
	sess.IdentityKeys[jid.SignalAddress().String()] = key
	sess.identityKeysLock.Unlock()
}

type preKeyArray []*keys.PreKey

func (p preKeyArray) Len() int {
	return len(p)
}

func (p preKeyArray) Less(i, j int) bool {
	return p[i].KeyID < p[j].KeyID
}

func (p preKeyArray) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (sess *Session) unlockedGetPreKeys(count uint32) []*keys.PreKey {
	foundKeys := make(preKeyArray, 0, count)
	for _, key := range sess.PreKeys {
		if key.KeyID >= sess.FirstUnuploadedID {
			foundKeys = append(foundKeys, key)
			if uint32(len(foundKeys)) >= count {
				break
			}
		}
	}
	sort.Sort(foundKeys)
	return foundKeys
}

func (sess *Session) GetPreKeys(count uint32) []*keys.PreKey {
	sess.preKeysLock.Lock()
	defer sess.preKeysLock.Unlock()
	return sess.unlockedGetPreKeys(count)
}

func (sess *Session) GetPreKey(id uint32) *keys.PreKey {
	sess.preKeysLock.Lock()
	key, ok := sess.PreKeys[id]
	sess.preKeysLock.Unlock()
	if !ok {
		return nil
	}
	return key
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
	return sess.FirstUnuploadedID > 1
}

func (sess *Session) MarkPreKeysAsUploaded(upTo uint32) {
	sess.preKeysLock.Lock()
	sess.FirstUnuploadedID = upTo + 1
	sess.preKeysLock.Unlock()
}
