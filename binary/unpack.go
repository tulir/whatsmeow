// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package binary

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

// Unpack unpacks the given decrypted data from the WhatsApp web API.
//
// It checks the first byte to decide whether to uncompress the data with zlib or just return as-is
// (without the first byte). There's currently no corresponding Pack function because Marshal
// already returns the data with a leading zero (i.e. not compressed).
func Unpack(data []byte) ([]byte, error) {
	dataType, data := data[0], data[1:]
	if 2&dataType > 0 {
		if decompressor, err := zlib.NewReader(bytes.NewReader(data)); err != nil {
			return nil, fmt.Errorf("failed to create zlib reader: %w", err)
		} else if data, err = io.ReadAll(decompressor); err != nil {
			return nil, err
		}
	}
	return data, nil
}
