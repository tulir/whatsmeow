package iface

import (
	"context"
	"net/http"
	"time"
)

const (
	BinaryMessage = 2
	CloseMessage  = 8
)

func FormatClosePayload(code int) []byte {
	payload := make([]byte, 2)
	payload[0] = byte(code >> 8)
	payload[1] = byte(code)
	return payload
}

type WebSocketConnection interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	Close() error
	SetCloseHandler(handler func(code int, text string) error)
}

type WebSocketDialer interface {
	DialContext(ctx context.Context, urlStr string, requestHeader http.Header) (WebSocketConnection, *http.Response, error)
}
