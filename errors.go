package whatsapp

import "errors"

var (
	ErrAlreadyConnected = errors.New("already connected")
	ErrAlreadyLoggedIn  = errors.New("already logged in")
)
