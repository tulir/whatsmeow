package socket_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"go.mau.fi/whatsmeow/socket"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type exitLogger struct {
	waLog.Logger
	exited chan struct{}
	done   atomic.Bool
}

func (l *exitLogger) Debugf(msg string, _ ...any) {
	if strings.Contains(msg, "read pump exiting") && l.done.CompareAndSwap(false, true) {
		close(l.exited)
	}
}

func (l *exitLogger) Sub(string) waLog.Logger { return l }

// A frame that arrives when nothing reads from Frames any more must not keep the read pump alive after the socket is
// closed.
func TestCloseReleasesReadPumpBlockedOnUnreadFrame(t *testing.T) {
	serverDone := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(serverDone)
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.CloseNow()
		// One whole frame: three length bytes (2) and two payload bytes.
		_ = c.Write(r.Context(), websocket.MessageBinary, []byte{0, 0, 2, 'h', 'i'})
		_, _, _ = c.Read(r.Context())
	}))
	defer srv.Close()

	log := &exitLogger{Logger: waLog.Noop, exited: make(chan struct{})}
	fs := socket.NewFrameSocket(log, srv.Client())
	fs.URL = "ws" + strings.TrimPrefix(srv.URL, "http")
	if err := fs.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Nobody reads fs.Frames, as after the frame consumer has stopped. Give the read pump time to block on the frame.
	time.Sleep(200 * time.Millisecond)
	fs.Close(websocket.StatusNormalClosure)
	select {
	case <-log.exited:
	case <-time.After(10 * time.Second):
		t.Fatal("the read pump is still blocked on an unread frame after Close")
	}
	<-serverDone
}
