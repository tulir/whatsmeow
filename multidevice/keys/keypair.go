// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys

import (
	"fmt"

	"github.com/RadicalApp/libsignal-protocol-go/ecc"

	"go.mau.fi/whatsmeow/crypto/curve25519"
)

type KeyPair struct {
	Pub  *[32]byte
	Priv *[32]byte
}

func NewKeyPair() (*KeyPair, error) {
	var kp KeyPair
	var err error
	kp.Priv, kp.Pub, err = curve25519.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate curve25519 keypair: %w", err)
	}
	return &kp, nil
}

func (kp *KeyPair) CreateSignedPreKey(keyID int) (*PreKey, error) {
	newKey, err := NewPreKey(keyID)
	if err != nil {
		return nil, err
	}
	newKey.Signature = kp.Sign(&newKey.KeyPair)
	return newKey, nil
}

func (kp *KeyPair) Sign(keyToSign *KeyPair) []byte {
	pubKeyForSignature := make([]byte, 33)
	pubKeyForSignature[0] = ecc.DjbType
	copy(pubKeyForSignature[1:], keyToSign.Pub[:])

	signature := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(*kp.Priv), pubKeyForSignature)
	return signature[:]
}

type PreKey struct {
	KeyPair
	KeyID     int
	Signature []byte
}

func NewPreKey(keyID int) (*PreKey, error) {
	if keyID <= 0 {
		return nil, fmt.Errorf("invalid prekey ID %d", keyID)
	}
	kp, err := NewKeyPair()
	if err != nil {
		return nil, err
	}
	return &PreKey{
		KeyPair: *kp,
		KeyID:   keyID,
	}, nil
}
