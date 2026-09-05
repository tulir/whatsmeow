// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"errors"
	"fmt"
	"testing"

	"go.mau.fi/whatsmeow/appstate"
)

func TestIsAppStateHashMismatch(t *testing.T) {
	wrap := func(err error) error {
		return fmt.Errorf("failed to decode app state regular patches: %w", fmt.Errorf("failed to verify patch v5: %w", err))
	}
	cases := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil", nil, false},
		{"unrelated", errors.New("connection reset by peer"), false},
		{"lthash", wrap(appstate.ErrMismatchingLTHash), true},
		{"patch mac", wrap(appstate.ErrMismatchingPatchMAC), true},
		{"content mac", wrap(appstate.ErrMismatchingContentMAC), true},
		{"index mac", wrap(appstate.ErrMismatchingIndexMAC), false},
		{"missing key", wrap(appstate.ErrKeyNotFound), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAppStateHashMismatch(tc.err); got != tc.expected {
				t.Errorf("isAppStateHashMismatch(%v) = %t, want %t", tc.err, got, tc.expected)
			}
		})
	}
}
