package whatsapp

import (
	"errors"
)

// Various errors that Client methods can return.
var (
	ErrMediaDownloadFailedWith404 = errors.New("download failed with status code 404")
	ErrMediaDownloadFailedWith410 = errors.New("download failed with status code 410")

	ErrNoSession = errors.New("can't encrypt message for device: no signal session established")

	ErrBroadcastListUnsupported = errors.New("sending to broadcast lists is not yet supported")
	ErrUnknownServer = errors.New("can't send message to unknown server")

	ErrNoURLPresent       = errors.New("no url present")
	ErrFileLengthMismatch = errors.New("file length does not match")
	ErrInvalidHashLength  = errors.New("hash too short")
	ErrTooShortFile       = errors.New("file too short")
	ErrInvalidMediaHMAC   = errors.New("invalid media hmac")
)
