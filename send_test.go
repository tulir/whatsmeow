// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"testing"

	"go.mau.fi/libsignal/keys/prekey"

	"go.mau.fi/whatsmeow/types"
)

func TestDropBundlesForExistingSessions(t *testing.T) {
	gained := types.JID{User: "1", Device: 0, Server: types.DefaultUserServer}
	missing := types.JID{User: "2", Device: 0, Server: types.DefaultUserServer}
	sessionAddressToJID := map[string]types.JID{
		gained.SignalAddress().String():  gained,
		missing.SignalAddress().String(): missing,
	}
	bundles := map[types.JID]*prekey.Bundle{
		gained:  {},
		missing: {},
	}
	existingSessions := map[string]bool{
		gained.SignalAddress().String():  true,
		missing.SignalAddress().String(): false,
	}

	dropBundlesForExistingSessions(bundles, existingSessions, sessionAddressToJID)

	if _, ok := bundles[gained]; ok {
		t.Error("bundle for an address that gained a session while unlocked wasn't dropped")
	}
	if _, ok := bundles[missing]; !ok {
		t.Error("bundle for an address that still has no session was dropped")
	}
}
