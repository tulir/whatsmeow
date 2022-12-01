// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gcmutil

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
)

func Prepare(secretKey []byte) (gcm cipher.AEAD, err error) {
	var block cipher.Block
	if block, err = aes.NewCipher(secretKey); err != nil {
		err = fmt.Errorf("failed to initialize AES cipher: %w", err)
	} else if gcm, err = cipher.NewGCM(block); err != nil {
		err = fmt.Errorf("failed to initialize GCM: %w", err)
	}
	return
}

func Decrypt(secretKey, iv, ciphertext, additionalData []byte) ([]byte, error) {
	if gcm, err := Prepare(secretKey); err != nil {
		return nil, err
	} else if plaintext, decryptErr := gcm.Open(nil, iv, ciphertext, additionalData); decryptErr != nil {
		return nil, decryptErr
	} else {
		return plaintext, nil
	}
}

func Encrypt(secretKey, iv, plaintext, additionalData []byte) ([]byte, error) {
	if gcm, err := Prepare(secretKey); err != nil {
		return nil, err
	} else {
		return gcm.Seal(nil, iv, plaintext, additionalData), nil
	}
}
