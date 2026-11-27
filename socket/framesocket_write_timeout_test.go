// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !js

package socket

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// stallConn forwards writes until stall is enabled, then blocks until Close.
// Close unblocks a stalled Write, matching how a kernel TCP write returns
// after the socket is closed (as coder/websocket does when the write context
// expires).
type stallConn struct {
	net.Conn

	mu        sync.Mutex
	stall     bool
	done      chan struct{}
	closeOnce sync.Once
}

func newStallConn(c net.Conn) *stallConn {
	return &stallConn{Conn: c, done: make(chan struct{})}
}

func (c *stallConn) setStall(v bool) {
	c.mu.Lock()
	c.stall = v
	c.mu.Unlock()
}

func (c *stallConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	stall := c.stall
	c.mu.Unlock()
	if !stall {
		return c.Conn.Write(p)
	}
	<-c.done
	return 0, net.ErrClosed
}

func (c *stallConn) Close() error {
	c.closeOnce.Do(func() { close(c.done) })
	return c.Conn.Close()
}

func startFrameSocketTestServer(t *testing.T) (url string, closeServer func()) {
	t.Helper()
	done := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			close(done)
		}
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
			CompressionMode:    websocket.CompressionContextTakeover,
		})
		if err != nil {
			return
		}
		defer conn.CloseNow()
		<-done
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http"), srv.Close
}

func connectTestFrameSocket(t *testing.T, url string, stall **stallConn) *FrameSocket {
	t.Helper()
	d := &net.Dialer{}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			c, err := d.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			sc := newStallConn(c)
			if stall != nil {
				*stall = sc
			}
			return sc, nil
		},
	}
	fs := NewFrameSocket(waLog.Noop, &http.Client{Transport: transport})
	fs.URL = url
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	if err := fs.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { fs.Close(websocket.StatusNormalClosure) })
	return fs
}

func TestSendFrameWriteTimeout(t *testing.T) {
	prev := WriteTimeout
	WriteTimeout = 200 * time.Millisecond
	t.Cleanup(func() { WriteTimeout = prev })

	url, _ := startFrameSocketTestServer(t)
	var stall *stallConn
	fs := connectTestFrameSocket(t, url, &stall)
	if stall == nil {
		t.Fatal("expected dial to wrap the connection")
	}
	stall.setStall(true)

	start := time.Now()
	err := fs.SendFrame([]byte("ping"))
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected write timeout, got nil")
	}
	if !errors.Is(err, ErrWriteTimeout) {
		t.Fatalf("expected ErrWriteTimeout, got %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	if elapsed < 200*time.Millisecond {
		t.Fatalf("write returned too quickly: %v", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("write did not fail fast: %v", elapsed)
	}
}

func TestSendFrameSucceedsWithinTimeout(t *testing.T) {
	prev := WriteTimeout
	WriteTimeout = time.Second
	t.Cleanup(func() { WriteTimeout = prev })

	url, _ := startFrameSocketTestServer(t)
	fs := connectTestFrameSocket(t, url, nil)
	if err := fs.SendFrame([]byte("hello")); err != nil {
		t.Fatalf("SendFrame: %v", err)
	}
}

func TestSendFrameWriteTimeoutDisabledBlocksUntilClose(t *testing.T) {
	prev := WriteTimeout
	WriteTimeout = 0
	t.Cleanup(func() { WriteTimeout = prev })

	url, _ := startFrameSocketTestServer(t)
	var stall *stallConn
	fs := connectTestFrameSocket(t, url, &stall)
	if stall == nil {
		t.Fatal("expected dial to wrap the connection")
	}
	stall.setStall(true)

	done := make(chan error, 1)
	go func() { done <- fs.SendFrame([]byte("ping")) }()

	select {
	case err := <-done:
		t.Fatalf("write returned before close with timeout disabled: %v", err)
	case <-time.After(300 * time.Millisecond):
	}

	fs.Close(0)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error after close, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("write did not return after the socket was closed")
	}
}
