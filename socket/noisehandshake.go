// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"crypto/cipher"
	"crypto/sha256"
	"fmt"
	"io"
	"sync/atomic"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"

	"go.mau.fi/whatsmeow/util/gcmutil"
)

type NoiseHandshake struct {
	hash    []byte
	salt    []byte
	key     cipher.AEAD
	counter uint32
}

func NewNoiseHandshake() *NoiseHandshake {
	return &NoiseHandshake{}
}

func sha256Slice(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

func (nh *NoiseHandshake) Start(pattern string, header []byte) {
	data := []byte(pattern)
	if len(data) == 32 {
		nh.hash = data
	} else {
		nh.hash = sha256Slice(data)
	}
	nh.salt = nh.hash
	var err error
	nh.key, err = gcmutil.Prepare(nh.hash)
	if err != nil {
		panic(err)
	}
	nh.Authenticate(header)
}

func (nh *NoiseHandshake) Authenticate(data []byte) {
	nh.hash = sha256Slice(append(nh.hash, data...))
}

func (nh *NoiseHandshake) postIncrementCounter() uint32 {
	count := atomic.AddUint32(&nh.counter, 1)
	return count - 1
}

func (nh *NoiseHandshake) Encrypt(plaintext []byte) []byte {
	ciphertext := nh.key.Seal(nil, generateIV(nh.postIncrementCounter()), plaintext, nh.hash)
	nh.Authenticate(ciphertext)
	return ciphertext
}

func (nh *NoiseHandshake) Decrypt(ciphertext []byte) (plaintext []byte, err error) {
	plaintext, err = nh.key.Open(nil, generateIV(nh.postIncrementCounter()), ciphertext, nh.hash)
	if err == nil {
		nh.Authenticate(ciphertext)
	}
	return
}

func (nh *NoiseHandshake) Finish(fs *FrameSocket, frameHandler FrameHandler, disconnectHandler DisconnectHandler) (*NoiseSocket, error) {
	if write, read, err := nh.extractAndExpand(nh.salt, nil); err != nil {
		return nil, fmt.Errorf("failed to extract final keys: %w", err)
	} else if writeKey, err := gcmutil.Prepare(write); err != nil {
		return nil, fmt.Errorf("failed to create final write cipher: %w", err)
	} else if readKey, err := gcmutil.Prepare(read); err != nil {
		return nil, fmt.Errorf("failed to create final read cipher: %w", err)
	} else if ns, err := newNoiseSocket(fs, writeKey, readKey, frameHandler, disconnectHandler); err != nil {
		return nil, fmt.Errorf("failed to create noise socket: %w", err)
	} else {
		return ns, nil
	}
}

func (nh *NoiseHandshake) MixSharedSecretIntoKey(priv, pub [32]byte) error {
	secret, err := curve25519.X25519(priv[:], pub[:])
	if err != nil {
		return fmt.Errorf("failed to do x25519 scalar multiplication: %w", err)
	}
	return nh.MixIntoKey(secret)
}

func (nh *NoiseHandshake) MixIntoKey(data []byte) error {
	nh.counter = 0
	write, read, err := nh.extractAndExpand(nh.salt, data)
	if err != nil {
		return fmt.Errorf("failed to extract keys for mixing: %w", err)
	}
	nh.salt = write
	nh.key, err = gcmutil.Prepare(read)
	if err != nil {
		return fmt.Errorf("failed to create new cipher while mixing keys: %w", err)
	}
	return nil
}

func (nh *NoiseHandshake) extractAndExpand(salt, data []byte) (write []byte, read []byte, err error) {
	h := hkdf.New(sha256.New, data, salt, nil)
	write = make([]byte, 32)
	read = make([]byte, 32)

	if _, err = io.ReadFull(h, write); err != nil {
		err = fmt.Errorf("failed to read write key: %w", err)
	} else if _, err = io.ReadFull(h, read); err != nil {
		err = fmt.Errorf("failed to read read key: %w", err)
	}
	return
}
