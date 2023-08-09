// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"errors"
	"fmt"

	waBinary "go.mau.fi/whatsmeow/binary"
)

// Miscellaneous errors
var (
	ErrNoSession       = errors.New("can't encrypt message for device: no signal session established")
	ErrIQTimedOut      = errors.New("info query timed out")
	ErrNotConnected    = errors.New("websocket not connected")
	ErrNotLoggedIn     = errors.New("the store doesn't contain a device JID")
	ErrMessageTimedOut = errors.New("timed out waiting for message send response")

	ErrAlreadyConnected = errors.New("websocket is already connected")

	ErrQRAlreadyConnected = errors.New("GetQRChannel must be called before connecting")
	ErrQRStoreContainsID  = errors.New("GetQRChannel can only be called when there's no user ID in the client's Store")

	ErrNoPushName = errors.New("can't send presence without PushName set")

	ErrNoPrivacyToken = errors.New("no privacy token stored")

	ErrAppStateUpdate = errors.New("server returned error updating app state")
)

// Errors that happen while confirming device pairing
var (
	ErrPairInvalidDeviceIdentityHMAC = errors.New("invalid device identity HMAC in pair success message")
	ErrPairInvalidDeviceSignature    = errors.New("invalid device signature in pair success message")
	ErrPairRejectedLocally           = errors.New("local PrePairCallback rejected pairing")
)

// PairProtoError is included in an events.PairError if the pairing failed due to a protobuf error.
type PairProtoError struct {
	Message  string
	ProtoErr error
}

func (err *PairProtoError) Error() string {
	return fmt.Sprintf("%s: %v", err.Message, err.ProtoErr)
}

func (err *PairProtoError) Unwrap() error {
	return err.ProtoErr
}

// PairDatabaseError is included in an events.PairError if the pairing failed due to being unable to save the credentials to the device store.
type PairDatabaseError struct {
	Message string
	DBErr   error
}

func (err *PairDatabaseError) Error() string {
	return fmt.Sprintf("%s: %v", err.Message, err.DBErr)
}

func (err *PairDatabaseError) Unwrap() error {
	return err.DBErr
}

var (
	// ErrProfilePictureUnauthorized is returned by GetProfilePictureInfo when trying to get the profile picture of a user
	// whose privacy settings prevent you from seeing their profile picture (status code 401).
	ErrProfilePictureUnauthorized = errors.New("the user has hidden their profile picture from you")
	// ErrProfilePictureNotSet is returned by GetProfilePictureInfo when the given user or group doesn't have a profile
	// picture (status code 404).
	ErrProfilePictureNotSet = errors.New("that user or group does not have a profile picture")
	// ErrGroupInviteLinkUnauthorized is returned by GetGroupInviteLink if you don't have the permission to get the link (status code 401).
	ErrGroupInviteLinkUnauthorized = errors.New("you don't have the permission to get the group's invite link")
	// ErrNotInGroup is returned by group info getting methods if you're not in the group (status code 403).
	ErrNotInGroup = errors.New("you're not participating in that group")
	// ErrGroupNotFound is returned by group info getting methods if the group doesn't exist (status code 404).
	ErrGroupNotFound = errors.New("that group does not exist")
	// ErrInviteLinkInvalid is returned by methods that use group invite links if the invite link is malformed.
	ErrInviteLinkInvalid = errors.New("that group invite link is not valid")
	// ErrInviteLinkRevoked is returned by methods that use group invite links if the invite link was valid, but has been revoked and can no longer be used.
	ErrInviteLinkRevoked = errors.New("that group invite link has been revoked")
	// ErrBusinessMessageLinkNotFound is returned by ResolveBusinessMessageLink if the link doesn't exist or has been revoked.
	ErrBusinessMessageLinkNotFound = errors.New("that business message link does not exist or has been revoked")
	// ErrContactQRLinkNotFound is returned by ResolveContactQRLink if the link doesn't exist or has been revoked.
	ErrContactQRLinkNotFound = errors.New("that contact QR link does not exist or has been revoked")
	// ErrInvalidImageFormat is returned by SetGroupPhoto if the given photo is not in the correct format.
	ErrInvalidImageFormat = errors.New("the given data is not a valid image")
	// ErrMediaNotAvailableOnPhone is returned by DecryptMediaRetryNotification if the given event contains error code 2.
	ErrMediaNotAvailableOnPhone = errors.New("media no longer available on phone")
	// ErrUnknownMediaRetryError is returned by DecryptMediaRetryNotification if the given event contains an unknown error code.
	ErrUnknownMediaRetryError = errors.New("unknown media retry error")
	// ErrInvalidDisappearingTimer is returned by SetDisappearingTimer if the given timer is not one of the allowed values.
	ErrInvalidDisappearingTimer = errors.New("invalid disappearing timer provided")
)

// Some errors that Client.SendMessage can return
var (
	ErrBroadcastListUnsupported = errors.New("sending to non-status broadcast lists is not yet supported")
	ErrUnknownServer            = errors.New("can't send message to unknown server")
	ErrRecipientADJID           = errors.New("message recipient must be a user JID with no device part")
	ErrServerReturnedError      = errors.New("server returned error")
)

// Some errors that Client.Download can return
var (
	ErrMediaDownloadFailedWith403 = errors.New("download failed with status code 403")
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

var (
	ErrOriginalMessageSecretNotFound = errors.New("original message secret key not found")
	ErrNotEncryptedReactionMessage   = errors.New("given message isn't an encrypted reaction message")
	ErrNotPollUpdateMessage          = errors.New("given message isn't a poll update message")
)

type wrappedIQError struct {
	HumanError error
	IQError    error
}

func (err *wrappedIQError) Error() string {
	return err.HumanError.Error()
}

func (err *wrappedIQError) Is(other error) bool {
	return errors.Is(other, err.HumanError)
}

func (err *wrappedIQError) Unwrap() error {
	return err.IQError
}

func wrapIQError(human, iq error) error {
	return &wrappedIQError{human, iq}
}

// IQError is a generic error container for info queries
type IQError struct {
	Code      int
	Text      string
	ErrorNode *waBinary.Node
	RawNode   *waBinary.Node
}

// Common errors returned by info queries for use with errors.Is
var (
	ErrIQBadRequest          error = &IQError{Code: 400, Text: "bad-request"}
	ErrIQNotAuthorized       error = &IQError{Code: 401, Text: "not-authorized"}
	ErrIQForbidden           error = &IQError{Code: 403, Text: "forbidden"}
	ErrIQNotFound            error = &IQError{Code: 404, Text: "item-not-found"}
	ErrIQNotAllowed          error = &IQError{Code: 405, Text: "not-allowed"}
	ErrIQNotAcceptable       error = &IQError{Code: 406, Text: "not-acceptable"}
	ErrIQGone                error = &IQError{Code: 410, Text: "gone"}
	ErrIQResourceLimit       error = &IQError{Code: 419, Text: "resource-limit"}
	ErrIQLocked              error = &IQError{Code: 423, Text: "locked"}
	ErrIQInternalServerError error = &IQError{Code: 500, Text: "internal-server-error"}
	ErrIQServiceUnavailable  error = &IQError{Code: 503, Text: "service-unavailable"}
	ErrIQPartialServerError  error = &IQError{Code: 530, Text: "partial-server-error"}
)

func parseIQError(node *waBinary.Node) error {
	var err IQError
	err.RawNode = node
	val, ok := node.GetOptionalChildByTag("error")
	if ok {
		err.ErrorNode = &val
		ag := val.AttrGetter()
		err.Code = ag.OptionalInt("code")
		err.Text = ag.OptionalString("text")
	}
	return &err
}

func (iqe *IQError) Error() string {
	if iqe.Code == 0 {
		if iqe.ErrorNode != nil {
			return fmt.Sprintf("info query returned unknown error: %s", iqe.ErrorNode.XMLString())
		} else if iqe.RawNode != nil {
			return fmt.Sprintf("info query returned unexpected response: %s", iqe.RawNode.XMLString())
		} else {
			return "unknown info query error"
		}
	}
	return fmt.Sprintf("info query returned status %d: %s", iqe.Code, iqe.Text)
}

func (iqe *IQError) Is(other error) bool {
	otherIQE, ok := other.(*IQError)
	if !ok {
		return false
	} else if iqe.Code != 0 && otherIQE.Code != 0 {
		return otherIQE.Code == iqe.Code && otherIQE.Text == iqe.Text
	} else if iqe.ErrorNode != nil && otherIQE.ErrorNode != nil {
		return iqe.ErrorNode.XMLString() == otherIQE.ErrorNode.XMLString()
	} else {
		return false
	}
}

// ElementMissingError is returned by various functions that parse XML elements when a required element is missing.
type ElementMissingError struct {
	Tag string
	In  string
}

func (eme *ElementMissingError) Error() string {
	return fmt.Sprintf("missing <%s> element in %s", eme.Tag, eme.In)
}

var ErrIQDisconnected = &DisconnectedError{Action: "info query"}

// DisconnectedError is returned if the websocket disconnects before an info query or other request gets a response.
type DisconnectedError struct {
	Action string
	Node   *waBinary.Node
}

func (err *DisconnectedError) Error() string {
	return fmt.Sprintf("websocket disconnected before %s returned response", err.Action)
}

func (err *DisconnectedError) Is(other error) bool {
	otherDisc, ok := other.(*DisconnectedError)
	if !ok {
		return false
	}
	return otherDisc.Action == err.Action
}
