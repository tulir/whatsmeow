// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package hkdfutil contains a simple wrapper for golang.org/x/crypto/hkdf that reads a specified number of bytes.
package hkdfutil

import (
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/hkdf"
)

func SHA256(key, salt, info []byte, length uint8) []byte {
	data := make([]byte, length)
	h := hkdf.New(sha256.New, key, salt, info)
	n, err := h.Read(data)
	if err != nil {
		// Length is limited to 255 by being uint8, so these errors can't actually happen
		panic(fmt.Errorf("failed to expand key: %w", err))
	} else if uint8(n) != length {
		panic(fmt.Errorf("didn't read enough bytes (got %d, wanted %d)", n, length))
	}
	return data
}
