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

	"github.com/gorilla/websocket"
)

type NoiseSocket struct {
	fs           *FrameSocket
	onFrame      FrameHandler
	writeKey     cipher.AEAD
	readKey      cipher.AEAD
	writeCounter uint32
	readCounter  uint32
	writeLock    sync.Mutex
	destroyed    atomic.Bool
	stopConsumer chan struct{}
}

type DisconnectHandler func(socket *NoiseSocket, remote bool)
type FrameHandler func([]byte)

func newNoiseSocket(fs *FrameSocket, writeKey, readKey cipher.AEAD, frameHandler FrameHandler, disconnectHandler DisconnectHandler) (*NoiseSocket, error) {
	ns := &NoiseSocket{
		fs:           fs,
		writeKey:     writeKey,
		readKey:      readKey,
		onFrame:      frameHandler,
		stopConsumer: make(chan struct{}),
	}
	fs.OnDisconnect = func(remote bool) {
		disconnectHandler(ns, remote)
	}
	go ns.consumeFrames(fs.ctx, fs.Frames)
	return ns, nil
}

func (ns *NoiseSocket) consumeFrames(ctx context.Context, frames <-chan []byte) {
	if ctx == nil {
		// ctx being nil implies the connection already closed somehow
		return
	}
	ctxDone := ctx.Done()
	for {
		select {
		case frame := <-frames:
			ns.receiveEncryptedFrame(frame)
		case <-ctxDone:
			return
		case <-ns.stopConsumer:
			return
		}
	}
}

func generateIV(count uint32) []byte {
	iv := make([]byte, 12)
	binary.BigEndian.PutUint32(iv[8:], count)
	return iv
}

func (ns *NoiseSocket) Context() context.Context {
	return ns.fs.Context()
}

func (ns *NoiseSocket) Stop(disconnect bool) {
	if ns.destroyed.CompareAndSwap(false, true) {
		close(ns.stopConsumer)
		ns.fs.OnDisconnect = nil
		if disconnect {
			ns.fs.Close(websocket.CloseNormalClosure)
		}
	}
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
	ns.onFrame(plaintext)
}

func (ns *NoiseSocket) IsConnected() bool {
	return ns.fs.IsConnected()
}
