//go:build !js

package socket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	waLog "go.mau.fi/whatsmeow/util/log"
)

func TestFrameSocketNegotiatesCompression(t *testing.T) {
	const payload = "compressed websocket frame "
	requestExtensions := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestExtensions <- r.Header.Get("Sec-WebSocket-Extensions")

		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
			CompressionMode:    websocket.CompressionContextTakeover,
		})
		if err != nil {
			t.Errorf("failed to accept websocket: %v", err)
			return
		}
		defer conn.CloseNow()

		data := []byte(strings.Repeat(payload, 16))
		frame := make([]byte, FrameLengthSize+len(data))
		frame[0] = byte(len(data) >> 16)
		frame[1] = byte(len(data) >> 8)
		frame[2] = byte(len(data))
		copy(frame[FrameLengthSize:], data)
		if err := conn.Write(r.Context(), websocket.MessageBinary, frame); err != nil {
			t.Errorf("failed to write compressed websocket frame: %v", err)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	fs := NewFrameSocket(waLog.Noop, http.DefaultClient)
	fs.URL = "ws" + strings.TrimPrefix(server.URL, "http")
	if err := fs.Connect(ctx); err != nil {
		t.Fatalf("failed to connect frame socket: %v", err)
	}
	defer fs.Close(0)

	select {
	case got := <-fs.Frames:
		expected := []byte(strings.Repeat(payload, 16))
		if string(got) != string(expected) {
			t.Fatalf("unexpected frame payload: got %d bytes, want %d", len(got), len(expected))
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for compressed frame: %v", ctx.Err())
	}

	select {
	case extensions := <-requestExtensions:
		if !strings.Contains(extensions, "permessage-deflate") {
			t.Fatalf("request did not negotiate permessage-deflate: %q", extensions)
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for websocket handshake: %v", ctx.Err())
	}
}
