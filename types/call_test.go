// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types

import "testing"

func TestCallCodecString(t *testing.T) {
	if CallCodecMLow.String() != "mlow" || CallCodecOpus.String() != "opus" {
		t.Fatalf("codec strings wrong: %q %q", CallCodecMLow, CallCodecOpus)
	}
}
