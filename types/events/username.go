// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package events

import "go.mau.fi/whatsmeow/types"

// UsernameChange is emitted when a username notification is received from WhatsApp.
type UsernameChange struct {
	Mex MexNotificationData `json:"-"`
	LID types.JID           `json:"lid"`
	// Username is the current value supplied by the server.
	Username string `json:"username"`
}
