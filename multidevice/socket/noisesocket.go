// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"crypto/cipher"
	"encoding/binary"
	"sync/atomic"
)

type NoiseSocket struct {
	fs           *FrameSocket
	OnFrame    func([]byte)
	writeKey     cipher.AEAD
	readKey      cipher.AEAD
	writeCounter uint32
	readCounter  uint32
}

func newNoiseSocket(fs *FrameSocket, writeKey, readKey cipher.AEAD) (*NoiseSocket, error) {
	ns := &NoiseSocket{
		fs:       fs,
		writeKey: writeKey,
		readKey:  readKey,
	}
	fs.OnFrame = ns.receiveEncryptedFrame
	return ns, nil
}

func generateIV(count uint32) []byte {
	iv := make([]byte, 12)
	binary.BigEndian.PutUint32(iv[8:], count)
	return iv
}

func (ns *NoiseSocket) SendFrame(plaintext []byte) error {
	count := atomic.AddUint32(&ns.writeCounter, 1)
	ciphertext := ns.writeKey.Seal(nil, generateIV(count), plaintext, nil)
	return ns.fs.SendFrame(ciphertext)
}

func (ns *NoiseSocket) receiveEncryptedFrame(ciphertext []byte) {
	count := atomic.AddUint32(&ns.writeCounter, 1)
	plaintext, err := ns.readKey.Open(nil, generateIV(count), ciphertext, nil)
	if err != nil {
		ns.fs.log.Warnln("Failed to decrypt frame:", err)
		return
	}
	ns.OnFrame(plaintext)
}
