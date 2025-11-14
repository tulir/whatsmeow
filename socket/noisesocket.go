// Copyright (c) 2025 Tulir Asokan
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

	"github.com/coder/websocket"
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

type DisconnectHandler func(ctx context.Context, socket *NoiseSocket, remote bool)
type FrameHandler func(context.Context, []byte)

func newNoiseSocket(
	ctx context.Context,
	fs *FrameSocket,
	writeKey, readKey cipher.AEAD,
	frameHandler FrameHandler,
	disconnectHandler DisconnectHandler,
) (*NoiseSocket, error) {
	ns := &NoiseSocket{
		fs:           fs,
		writeKey:     writeKey,
		readKey:      readKey,
		onFrame:      frameHandler,
		stopConsumer: make(chan struct{}),
	}
	fs.OnDisconnect = func(ctx context.Context, remote bool) {
		disconnectHandler(ctx, ns, remote)
	}
	go ns.consumeFrames(ctx, fs.Frames)
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
			ns.receiveEncryptedFrame(ctx, frame)
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

func (ns *NoiseSocket) Stop(disconnect bool) {
	if ns.destroyed.CompareAndSwap(false, true) {
		close(ns.stopConsumer)
		ns.fs.OnDisconnect = nil
		if disconnect {
			ns.fs.Close(websocket.StatusNormalClosure)
		}
	}
}

func (ns *NoiseSocket) SendFrame(ctx context.Context, plaintext []byte) error {
	ns.writeLock.Lock()
	defer ns.writeLock.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// Don't reuse plaintext slice for storage as it may be needed for retries
	ciphertext := ns.writeKey.Seal(nil, generateIV(ns.writeCounter), plaintext, nil)
	ns.writeCounter++
	doneChan := make(chan error, 1)
	go func() {
		doneChan <- ns.fs.SendFrame(ciphertext)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case retErr := <-doneChan:
		return retErr
	}
}

func (ns *NoiseSocket) receiveEncryptedFrame(ctx context.Context, ciphertext []byte) {
	plaintext, err := ns.readKey.Open(ciphertext[:0], generateIV(ns.readCounter), ciphertext, nil)
	ns.readCounter++
	if err != nil {
		ns.fs.log.Warnf("Failed to decrypt frame: %v", err)
		return
	}
	ns.onFrame(ctx, plaintext)
}

func (ns *NoiseSocket) IsConnected() bool {
	return ns.fs.IsConnected()
}
