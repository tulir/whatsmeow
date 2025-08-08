package net

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"go.mau.fi/whatsmeow/iface"
)

type CoderConn struct {
	conn *websocket.Conn

	readDeadline  time.Time
	writeDeadline time.Time
}

func (c *CoderConn) ReadMessage() (messageType int, p []byte, err error) {
	ctx := context.Background()
	if !c.readDeadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, c.readDeadline)
		defer cancel()
	}
	typ, p, err := c.conn.Read(ctx)
	return int(typ), p, err
}

func (c *CoderConn) SetCloseHandler(handler func(code int, text string) error) {
}

func (c *CoderConn) WriteMessage(messageType int, data []byte) error {
	ctx := context.Background()
	if !c.writeDeadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, c.writeDeadline)
		defer cancel()
	}
	return c.conn.Write(ctx, websocket.MessageType(messageType), data)
}

func (c *CoderConn) SetReadDeadline(t time.Time) error {
	c.readDeadline = t
	return nil
}

func (c *CoderConn) SetWriteDeadline(t time.Time) error {
	c.writeDeadline = t
	return nil
}

func (c *CoderConn) Close() error {
	err := c.conn.Close(websocket.StatusNormalClosure, "")

	if err != nil {
		if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "EOF") {
			return nil
		}
	}

	return err
}

type CoderDialer struct {
	DialOptions websocket.DialOptions
}

func NewDefaultCoderDialer() *CoderDialer {
	return &CoderDialer{}
}

type headerRoundTripper struct {
	headers http.Header
	rt      http.RoundTripper
}

func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	newHeader := req.Header.Clone()
	for k, v := range h.headers {
		newHeader[k] = v
	}
	req.Header = newHeader
	return h.rt.RoundTrip(req)
}

func (d *CoderDialer) DialContext(ctx context.Context, urlStr string, requestHeader http.Header) (iface.WebSocketConnection, *http.Response, error) {
	opts := d.DialOptions

	if len(requestHeader) > 0 {
		baseClient := http.DefaultClient
		if opts.HTTPClient != nil {
			baseClient = opts.HTTPClient
		}

		baseTransport := baseClient.Transport
		if baseTransport == nil {
			baseTransport = http.DefaultTransport
		}

		headerTransport := &headerRoundTripper{
			headers: requestHeader,
			rt:      baseTransport,
		}

		clientClone := *baseClient
		clientClone.Transport = headerTransport
		opts.HTTPClient = &clientClone
	}

	conn, resp, err := websocket.Dial(ctx, urlStr, &opts)
	if err != nil {
		return nil, resp, err
	}

	conn.SetReadLimit(1 << 20) // 1 MiB

	return &CoderConn{conn: conn}, resp, nil
}
