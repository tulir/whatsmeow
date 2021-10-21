package binary

import "errors"

// Errors returned by the binary XML decoder.
var (
	ErrInvalidType    = errors.New("unsupported payload type")
	ErrInvalidJIDType = errors.New("invalid JID type")
	ErrInvalidNode    = errors.New("invalid node")
	ErrInvalidToken   = errors.New("invalid token with tag")
	ErrNonStringKey   = errors.New("non-string key")
)
