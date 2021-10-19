// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package lthash implements a summation based hash algorithm that maintains the
// integrity of a piece of data over a series of mutations. You can add/remove
// mutations, and it'll return a hash equal to if the same series of mutations
// was made sequentially.
package lthash

import (
	"encoding/binary"

	"go.mau.fi/whatsmeow/util/hkdfutil"
)

type LTHash struct {
	HKDFInfo []byte
	HKDFSize uint8
}

// WAPatchIntegrity is a LTHash instance initialized with the details used for verifying integrity of WhatsApp app state sync patches.
var WAPatchIntegrity = LTHash{[]byte("WhatsApp Patch Integrity"), 128}

func (lth LTHash) SubtractThenAdd(base []byte, subtract, add [][]byte) []byte {
	output := make([]byte, len(base))
	copy(output, base)
	lth.SubtractThenAddInPlace(output, subtract, add)
	return output
}

func (lth LTHash) SubtractThenAddInPlace(base []byte, subtract, add [][]byte) {
	lth.multipleOp(base, subtract, true)
	lth.multipleOp(base, add, false)
}

func (lth LTHash) multipleOp(base []byte, input [][]byte, subtract bool) {
	for _, item := range input {
		performPointwiseWithOverflow(base, hkdfutil.SHA256(item, nil, lth.HKDFInfo, lth.HKDFSize), subtract)
	}
}

func performPointwiseWithOverflow(base, input []byte, subtract bool) []byte {
	for i := 0; i < len(base); i += 2 {
		x := binary.LittleEndian.Uint16(base[i : i+2])
		y := binary.LittleEndian.Uint16(input[i : i+2])
		var result uint16
		if subtract {
			result = x - y
		} else {
			result = x + y
		}
		binary.LittleEndian.PutUint16(base[i:i+2], result)
	}
	return base
}
