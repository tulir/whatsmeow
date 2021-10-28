// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"errors"
)

// Miscellaneous errors
var (
	ErrNoSession            = errors.New("can't encrypt message for device: no signal session established")
	ErrIQUnexpectedResponse = errors.New("unexpected info query response")
	ErrIQError              = errors.New("info query returned error")
	ErrIQTimedOut           = errors.New("info query timed out")
	ErrIQDisconnected       = errors.New("websocket disconnected before info query returned response")
	ErrNotConnected         = errors.New("websocket not connected")
	ErrNotLoggedIn          = errors.New("the store doesn't contain a device JID")

	ErrAlreadyConnected = errors.New("websocket is already connected")

	ErrQRAlreadyConnected = errors.New("GetQRChannel must be called before connecting")
	ErrQRStoreContainsID  = errors.New("GetQRChannel can only be called when there's no user ID in the client's Store")

	ErrNoPushName = errors.New("can't send presence without PushName set")
)

var (
	// ErrProfilePictureUnauthorized is returned by GetProfilePictureInfo when trying to get the profile picture of a user
	// whose privacy settings prevent you from seeing their profile picture.
	ErrProfilePictureUnauthorized = errors.New("the user has hidden their profile picture from you")
)

// Some errors that Client.SendMessage can return
var (
	ErrBroadcastListUnsupported = errors.New("sending to broadcast lists is not yet supported")
	ErrUnknownServer            = errors.New("can't send message to unknown server")
	ErrRecipientADJID           = errors.New("message recipient must be normal (non-AD) JID")
)

// Some errors that Client.Download can return
var (
	ErrMediaDownloadFailedWith404 = errors.New("download failed with status code 404")
	ErrMediaDownloadFailedWith410 = errors.New("download failed with status code 410")
	ErrNoURLPresent               = errors.New("no url present")
	ErrFileLengthMismatch         = errors.New("file length does not match")
	ErrTooShortFile               = errors.New("file too short")
	ErrInvalidMediaHMAC           = errors.New("invalid media hmac")
	ErrInvalidMediaEncSHA256      = errors.New("hash of media ciphertext doesn't match")
	ErrInvalidMediaSHA256         = errors.New("hash of media plaintext doesn't match")
	ErrUnknownMediaType           = errors.New("unknown media type")
	ErrNothingDownloadableFound   = errors.New("didn't find any attachments in message")
)
