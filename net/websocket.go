//go:build !wasm

package net

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"go.mau.fi/whatsmeow/iface"
)

type GorillaConn struct {
	*websocket.Conn
}

func (c *GorillaConn) ReadMessage() (messageType int, p []byte, err error) {
	return c.Conn.ReadMessage()
}

func (c *GorillaConn) SetCloseHandler(handler func(code int, text string) error) {
	c.Conn.SetCloseHandler(handler)
}

func (c *GorillaConn) WriteMessage(messageType int, data []byte) error {
	return c.Conn.WriteMessage(messageType, data)
}

func (c *GorillaConn) SetReadDeadline(t time.Time) error {
	return c.Conn.SetReadDeadline(t)
}

func (c *GorillaConn) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(t)
}

func (c *GorillaConn) Close() error {
	return c.Conn.Close()
}

type GorillaDialer struct {
	*websocket.Dialer
}

func NewDefaultGorillaDialer() *GorillaDialer {
	return &GorillaDialer{
		Dialer: &websocket.Dialer{},
	}
}

func (d *GorillaDialer) DialContext(ctx context.Context, urlStr string, requestHeader http.Header) (iface.WebSocketConnection, *http.Response, error) {
	conn, resp, err := d.Dialer.DialContext(ctx, urlStr, requestHeader)
	if err != nil {
		return nil, resp, err
	}
	return &GorillaConn{Conn: conn}, resp, nil
}
