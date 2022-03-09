// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types

type Presence string

const (
	PresenceAvailable   Presence = "available"
	PresenceUnavailable Presence = "unavailable"
)

type ChatPresence string

const (
	ChatPresenceComposing ChatPresence = "composing"
	ChatPresencePaused    ChatPresence = "paused"
)

type ChatPresenceMedia string

const (
	ChatPresenceMediaText  ChatPresenceMedia = ""
	ChatPresenceMediaAudio ChatPresenceMedia = "audio"
)
