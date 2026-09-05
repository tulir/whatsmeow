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

	"go.mau.fi/libsignal/signalerror"
)

func TestGetRetryReasonFromError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, RetryReasonUnknownError},
		{"unrelated", errors.New("something else"), RetryReasonUnknownError},
		{"bad mac", signalerror.ErrBadMAC, RetryReasonSignalErrorBadMac},
		{"no session", signalerror.ErrNoSessionForUser, RetryReasonSignalErrorNoSession},
		{"no sender key", signalerror.ErrNoSenderKeyForUser, RetryReasonSignalErrorNoSession},
		{"wrong version", signalerror.ErrWrongMessageVersion, RetryReasonSignalErrorInvalidMessage},
		{"old version", signalerror.ErrOldMessageVersion, RetryReasonSignalErrorInvalidMessage},
		{"unknown version", signalerror.ErrUnknownMessageVersion, RetryReasonSignalErrorInvalidMessage},
		{"incomplete message", signalerror.ErrIncompleteMessage, RetryReasonSignalErrorInvalidMessage},
		{"invalid signature", signalerror.ErrInvalidSignature, RetryReasonSignalErrorInvalidSignature},
		{"sender key verification", signalerror.ErrSenderKeyStateVerificationFailed, RetryReasonSignalErrorInvalidSignature},
		{"no signed prekey", signalerror.ErrNoSignedPreKey, RetryReasonSignalErrorInvalidKey},
		{"no sender key state", signalerror.ErrNoSenderKeyStateForID, RetryReasonSignalErrorInvalidKeyID},
		{"too far into future", signalerror.ErrTooFarIntoFuture, RetryReasonSignalErrorFutureMessage},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := getRetryReasonFromError(test.err); got != test.want {
				t.Errorf("getRetryReasonFromError(%v) = %d, want %d", test.err, got, test.want)
			}
			if test.err == nil {
				return
			}
			wrapped := fmt.Errorf("failed to decrypt message: %w", test.err)
			if got := getRetryReasonFromError(wrapped); got != test.want {
				t.Errorf("getRetryReasonFromError(%v) = %d, want %d", wrapped, got, test.want)
			}
		})
	}
}
