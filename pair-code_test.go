// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
)

func TestHandleCodePairNotificationPasskeyHandoff(t *testing.T) {
	cli := &Client{}
	cli.phoneLinkingCache.Store(&phoneLinkingCache{pairingRef: "pairing-ref"})
	node := &waBinary.Node{Content: []waBinary.Node{{
		Tag: "link_code_companion_reg",
		Content: []waBinary.Node{{
			Tag:     "link_code_pairing_ref",
			Content: []byte("pairing-ref"),
		}},
	}}}

	err := cli.handleCodePairNotification(context.Background(), node)
	if err != nil {
		t.Fatalf("expected passkey handoff precursor to be accepted, got %v", err)
	}
}

func TestHandleCodePairNotificationMissingWrappedPrimaryKeyValidation(t *testing.T) {
	tests := []struct {
		name        string
		cachedRef   string
		notifiedRef string
		wantError   string
	}{
		{
			name:        "no pending pairing",
			notifiedRef: "pairing-ref",
			wantError:   "received code pair notification without a pending pairing",
		},
		{
			name:        "pairing ref mismatch",
			cachedRef:   "pairing-ref",
			notifiedRef: "different-ref",
			wantError:   "pairing ref mismatch in code pair notification",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cli := &Client{}
			if test.cachedRef != "" {
				cli.phoneLinkingCache.Store(&phoneLinkingCache{pairingRef: test.cachedRef})
			}
			node := &waBinary.Node{Content: []waBinary.Node{{
				Tag: "link_code_companion_reg",
				Content: []waBinary.Node{{
					Tag:     "link_code_pairing_ref",
					Content: []byte(test.notifiedRef),
				}},
			}}}

			err := cli.handleCodePairNotification(context.Background(), node)
			if err == nil || err.Error() != test.wantError {
				t.Fatalf("expected error %q, got %v", test.wantError, err)
			}
		})
	}
}

func TestHandleCodePairNotificationRejectsShortWrappedPrimaryKey(t *testing.T) {
	cli := &Client{}
	cli.phoneLinkingCache.Store(&phoneLinkingCache{pairingRef: "pairing-ref"})
	node := &waBinary.Node{Content: []waBinary.Node{{
		Tag: "link_code_companion_reg",
		Content: []waBinary.Node{
			{
				Tag:     "link_code_pairing_ref",
				Content: []byte("pairing-ref"),
			},
			{
				Tag:     "link_code_pairing_wrapped_primary_ephemeral_pub",
				Content: make([]byte, 79),
			},
		},
	}}}

	err := cli.handleCodePairNotification(context.Background(), node)
	if err == nil || err.Error() != "unexpected length of link_code_pairing_wrapped_primary_ephemeral_pub: 79" {
		t.Fatalf("expected wrapped primary key length error, got %v", err)
	}
}
