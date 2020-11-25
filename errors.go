package whatsapp

import (
	"fmt"

	"github.com/pkg/errors"
)

var (
	ErrAlreadyConnected          = errors.New("already connected")
	ErrAlreadyLoggedIn           = errors.New("already logged in")
	ErrInvalidSession            = errors.New("invalid session")
	ErrLoginInProgress           = errors.New("login or restore already running")
	ErrNotConnected              = errors.New("not connected")
	ErrInvalidWsData             = errors.New("received invalid data")
	ErrInvalidWsState            = errors.New("can't handle binary data when not logged in")
	ErrConnectionTimeout         = errors.New("connection timed out")
	ErrMissingMessageTag         = errors.New("no messageTag specified or to short")
	ErrInvalidHmac               = errors.New("invalid hmac")
	ErrInvalidServerResponse     = errors.New("invalid response received from server")
	ErrServerRespondedWith404    = errors.New("server responded with status 404")
	ErrInvalidWebsocket          = errors.New("invalid websocket")
	ErrMessageTypeNotImplemented = errors.New("message type not implemented")
	ErrOptionsNotProvided        = errors.New("new conn options not provided")
)

type ErrConnectionFailed struct {
	Err error
}

func (e *ErrConnectionFailed) Error() string {
	return fmt.Sprintf("connection to WhatsApp servers failed: %v", e.Err)
}

type ErrConnectionClosed struct {
	Code int
	Text string
}

func (e *ErrConnectionClosed) Error() string {
	return fmt.Sprintf("server closed connection,code: %d,text: %s", e.Code, e.Text)
}
