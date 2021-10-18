package whatsmeow

import (
	"errors"
)

// Various errors that Client methods can return.
var (
	ErrMediaDownloadFailedWith404 = errors.New("download failed with status code 404")
	ErrMediaDownloadFailedWith410 = errors.New("download failed with status code 410")

	ErrNoSession = errors.New("can't encrypt message for device: no signal session established")

	ErrBroadcastListUnsupported = errors.New("sending to broadcast lists is not yet supported")
	ErrUnknownServer            = errors.New("can't send message to unknown server")
	ErrRecipientADJID           = errors.New("message recipient must be normal (non-AD) JID")

	ErrNoURLPresent       = errors.New("no url present")
	ErrFileLengthMismatch = errors.New("file length does not match")
	ErrInvalidHashLength  = errors.New("hash too short")
	ErrTooShortFile       = errors.New("file too short")
	ErrInvalidMediaHMAC   = errors.New("invalid media hmac")

	ErrInvalidMediaEncSHA256 = errors.New("hash of media ciphertext doesn't match")
	ErrInvalidMediaSHA256    = errors.New("hash of media plaintext doesn't match")

	ErrUnknownMediaType         = errors.New("unknown media type")
	ErrNothingDownloadableFound = errors.New("didn't find any attachments in message")
)
