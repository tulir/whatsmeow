// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types

import (
	waProto "go.mau.fi/whatsmeow/binary/proto"
)

// VerifiedName contains verified WhatsApp business details.
type VerifiedName struct {
	Certificate *waProto.VerifiedNameCertificate
	Details     *waProto.VerifiedNameDetails
}

// UserInfo contains info about a WhatsApp user.
type UserInfo struct {
	VerifiedName *VerifiedName
	Status       string
	PictureID    string
	Devices      []JID
}

// ProfilePictureInfo contains the ID and URL for a WhatsApp user's profile picture or group's photo.
type ProfilePictureInfo struct {
	URL  string // The full URL for the image, can be downloaded with a simple HTTP request.
	ID   string // The ID of the image. This is the same as UserInfo.PictureID.
	Type string // The type of image. Known types include "image" (full res) and "preview" (thumbnail).

	DirectPath string // The path to the image, probably not very useful
}
