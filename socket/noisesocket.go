// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"context"
	"crypto/cipher"
	"encoding/binary"
	"sync"
	"sync/atomic"
)

type NoiseSocket struct {
	fs           *FrameSocket
	OnFrame      func([]byte)
	writeKey     cipher.AEAD
	readKey      cipher.AEAD
	writeCounter uint32
	readCounter  uint32
	writeLock    sync.Mutex
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

func (ns *NoiseSocket) Context() context.Context {
	return ns.fs.Context()
}

func (ns *NoiseSocket) Close() {
	ns.fs.Close()
}

func (ns *NoiseSocket) SendFrame(plaintext []byte) error {
	ns.writeLock.Lock()
	ciphertext := ns.writeKey.Seal(nil, generateIV(ns.writeCounter), plaintext, nil)
	ns.writeCounter++
	err := ns.fs.SendFrame(ciphertext)
	ns.writeLock.Unlock()
	return err
}

func (ns *NoiseSocket) receiveEncryptedFrame(ciphertext []byte) {
	count := atomic.AddUint32(&ns.readCounter, 1) - 1
	plaintext, err := ns.readKey.Open(nil, generateIV(count), ciphertext, nil)
	if err != nil {
		ns.fs.log.Warnf("Failed to decrypt frame: %v", err)
		return
	}
	ns.OnFrame(plaintext)
}

func (ns *NoiseSocket) SetOnDisconnect(onDisconnect func()) {
	ns.fs.OnDisconnect = onDisconnect
}

func (ns *NoiseSocket) IsConnected() bool {
	return ns.fs.IsConnected()
}

func (ns *NoiseSocket) SetOnFrame(onFrame func([]byte)) {
	ns.OnFrame = onFrame
}

func (ns *NoiseSocket) GetOnFrame() func([]byte) {
	return ns.OnFrame
}

func (ns *NoiseSocket) ConsumeNextFrame() (output <-chan []byte, cancel func()) {
	return ConsumeNextFrame(ns)
}
