// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package session

import (
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/multidevice/keys"
)

type Session struct {
	NoiseKey          *keys.KeyPair
	SignedIdentityKey *keys.KeyPair
	SignedPreKey      *keys.SignedKeyPair
	RegistrationID    uint16
	AdvSecretKey      []byte

	identityKeys map[waBinary.FullJID][32]byte

	Platform     string
	BusinessName string
	ID           *waBinary.FullJID
}
