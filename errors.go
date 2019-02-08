package whatsapp

import "errors"

var (
	ErrAlreadyConnected = errors.New("already connected")
	ErrAlreadyLoggedIn  = errors.New("already logged in")
	ErrInvalidSession   = errors.New("invalid session")
	ErrNotConnected     = errors.New("not connected")
)
