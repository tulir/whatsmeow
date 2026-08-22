// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waConsumerApplication"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type outboundTestSocket struct {
	connected      atomic.Bool
	frames         chan []byte
	stopCalls      atomic.Int32
	sendError      error
	writeRelease   <-chan struct{}
	writeCompleted chan<- struct{}
}

func newOutboundTestSocket(buffer int) *outboundTestSocket {
	sock := &outboundTestSocket{frames: make(chan []byte, buffer)}
	sock.connected.Store(true)
	return sock
}

func (sock *outboundTestSocket) SendFrame(ctx context.Context, frame []byte) error {
	frameCopy := append([]byte(nil), frame...)
	select {
	case sock.frames <- frameCopy:
	case <-ctx.Done():
		return ctx.Err()
	}
	if sock.sendError != nil {
		return sock.sendError
	}
	if sock.writeRelease == nil {
		return nil
	}
	writeDone := make(chan struct{})
	go func() {
		<-sock.writeRelease
		if sock.writeCompleted != nil {
			sock.writeCompleted <- struct{}{}
		}
		close(writeDone)
	}()
	select {
	case <-writeDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (sock *outboundTestSocket) IsConnected() bool {
	return sock.connected.Load()
}

func (sock *outboundTestSocket) Stop(_, _ bool) {
	sock.stopCalls.Add(1)
	sock.connected.Store(false)
}

type outboundMethod string

const (
	outboundWhatsApp outboundMethod = "SendMessage"
	outboundFacebook outboundMethod = "SendFBMessage"
)

type outboundResult struct {
	resp SendResponse
	err  error
}

func newOutboundTestClient(t *testing.T, sock clientSocket) *Client {
	t.Helper()
	ownID := types.JID{User: "10000000000", Device: 1, Server: types.DefaultUserServer}
	noop := &store.NoopStore{}
	device := &store.Device{ID: &ownID, LID: ownID}
	device.SetAllStores(noop)
	device.LIDs = noop
	cli := NewClient(device, nil)
	cli.socket = sock
	cli.isLoggedIn.Store(true)

	// SendFBMessage asks for device lists before reaching the common message-frame
	// response path. A list containing only this client is valid and results in no
	// peer ciphertext, while still exercising the public method and frame writer.
	fbTarget := types.NewJID("20000000000", types.MessengerServer)
	cli.userDevicesCache[fbTarget] = deviceCache{devices: []types.JID{ownID}}
	cli.userDevicesCache[ownID.ToNonAD()] = deviceCache{devices: []types.JID{ownID}}
	return cli
}

func startOutboundSend(t *testing.T, cli *Client, method outboundMethod, ctx context.Context, id types.MessageID, extra SendRequestExtra) <-chan outboundResult {
	t.Helper()
	extra.ID = id
	result := make(chan outboundResult, 1)
	go func() {
		var resp SendResponse
		var err error
		switch method {
		case outboundWhatsApp:
			resp, err = cli.SendMessage(ctx, types.NewJID("test", types.NewsletterServer), &waE2E.Message{
				Conversation: proto.String("networkless test"),
			}, extra)
		case outboundFacebook:
			resp, err = cli.SendFBMessage(ctx, types.NewJID("20000000000", types.MessengerServer),
				&waConsumerApplication.ConsumerApplication{}, nil, extra)
		default:
			panic(fmt.Sprintf("unknown outbound method %q", method))
		}
		result <- outboundResult{resp: resp, err: err}
	}()
	return result
}

func receiveFrame(t *testing.T, sock *outboundTestSocket) []byte {
	t.Helper()
	select {
	case frame := <-sock.frames:
		return frame
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message frame")
		return nil
	}
}

func receiveResult(t *testing.T, result <-chan outboundResult) outboundResult {
	t.Helper()
	select {
	case res := <-result:
		return res
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for public send result")
		return outboundResult{}
	}
}

func routeAck(t *testing.T, cli *Client, id types.MessageID) bool {
	t.Helper()
	return cli.receiveResponse(context.Background(), &waBinary.Node{
		Tag: "ack",
		Attrs: waBinary.Attrs{
			"id": string(id),
			"t":  "1",
		},
	})
}

func disconnectForTest(cli *Client, sock *outboundTestSocket, node *waBinary.Node) {
	sock.connected.Store(false)
	cli.socketLock.Lock()
	if cli.socket == sock {
		cli.socket = nil
	}
	cli.socketLock.Unlock()
	cli.clearResponseWaiters(node)
	cli.closeSocketWaitChan()
}

func reconnectForTest(cli *Client, sock *outboundTestSocket) {
	cli.socketLock.Lock()
	cli.socket = sock
	cli.socketLock.Unlock()
	cli.closeSocketWaitChan()
}

func assertNoWaiters(t *testing.T, cli *Client) {
	t.Helper()
	cli.responseWaitersLock.Lock()
	remaining := len(cli.responseWaiters)
	cli.responseWaitersLock.Unlock()
	if remaining != 0 {
		t.Fatalf("response waiter leak: %d remain", remaining)
	}
}

func TestPublicMessageMethodsNoRetryOutcomes(t *testing.T) {
	for _, method := range []outboundMethod{outboundWhatsApp, outboundFacebook} {
		method := method
		t.Run(string(method), func(t *testing.T) {
			t.Run("normal acknowledgement", func(t *testing.T) {
				sock := newOutboundTestSocket(1)
				cli := newOutboundTestClient(t, sock)
				id := types.MessageID("normal-" + string(method))
				result := startOutboundSend(t, cli, method, context.Background(), id, SendRequestExtra{NoRetry: true})
				receiveFrame(t, sock)
				if !routeAck(t, cli, id) {
					t.Fatal("production response router did not match acknowledgement")
				}
				if res := receiveResult(t, result); res.err != nil {
					t.Fatalf("normal acknowledgement failed: %v", res.err)
				}
				assertNoWaiters(t, cli)
			})

			for _, disconnect := range []struct {
				name string
				node *waBinary.Node
			}{
				{name: "ordinary disconnect", node: xmlStreamEndNode},
				{name: "auth disconnect", node: &waBinary.Node{Tag: "stream:error", Attrs: waBinary.Attrs{"code": "401"}}},
			} {
				disconnect := disconnect
				t.Run(disconnect.name, func(t *testing.T) {
					first := newOutboundTestSocket(1)
					cli := newOutboundTestClient(t, first)
					id := types.MessageID(disconnect.name + "-" + string(method))
					result := startOutboundSend(t, cli, method, context.Background(), id, SendRequestExtra{NoRetry: true})
					receiveFrame(t, first)
					disconnectForTest(cli, first, disconnect.node)
					res := receiveResult(t, result)
					var disconnected *DisconnectedError
					if !errors.As(res.err, &disconnected) || disconnected.Action != "message send" || disconnected.Node != disconnect.node {
						t.Fatalf("expected original message-send disconnect, got %v", res.err)
					}

					second := newOutboundTestSocket(1)
					reconnectForTest(cli, second)
					if routeAck(t, cli, id) {
						t.Fatal("late acknowledgement matched a completed request")
					}
					if len(second.frames) != 0 {
						t.Fatalf("NoRetry resent %d frames after reconnect", len(second.frames))
					}
					assertNoWaiters(t, cli)
				})
			}
		})
	}
}

func TestPublicMessageMethodsDefaultRetryPreserved(t *testing.T) {
	for _, method := range []outboundMethod{outboundWhatsApp, outboundFacebook} {
		method := method
		t.Run(string(method), func(t *testing.T) {
			first := newOutboundTestSocket(1)
			cli := newOutboundTestClient(t, first)
			id := types.MessageID("retry-" + string(method))
			result := startOutboundSend(t, cli, method, context.Background(), id, SendRequestExtra{})
			receiveFrame(t, first)
			disconnectForTest(cli, first, xmlStreamEndNode)

			second := newOutboundTestSocket(1)
			reconnectForTest(cli, second)
			receiveFrame(t, second)
			if !routeAck(t, cli, id) {
				t.Fatal("production response router did not match retry acknowledgement")
			}
			if res := receiveResult(t, result); res.err != nil {
				t.Fatalf("default retry failed: %v", res.err)
			}
			if len(first.frames) != 0 || len(second.frames) != 0 {
				t.Fatal("unexpected extra message frames")
			}
			assertNoWaiters(t, cli)
		})
	}
}

func TestPublicNoRetryShutdownAndTimeoutCleanup(t *testing.T) {
	for _, method := range []outboundMethod{outboundWhatsApp, outboundFacebook} {
		method := method
		t.Run(string(method)+" initial send error", func(t *testing.T) {
			sendErr := errors.New("synthetic send error")
			sock := newOutboundTestSocket(1)
			sock.sendError = sendErr
			cli := newOutboundTestClient(t, sock)
			result := startOutboundSend(t, cli, method, context.Background(), types.MessageID("send-error-"+string(method)), SendRequestExtra{NoRetry: true})
			receiveFrame(t, sock)
			if res := receiveResult(t, result); !errors.Is(res.err, sendErr) {
				t.Fatalf("expected initial send error, got %v", res.err)
			}
			if len(sock.frames) != 0 {
				t.Fatalf("send error produced extra frames: %d", len(sock.frames))
			}
			assertNoWaiters(t, cli)
		})

		t.Run(string(method)+" shutdown", func(t *testing.T) {
			sock := newOutboundTestSocket(1)
			cli := newOutboundTestClient(t, sock)
			result := startOutboundSend(t, cli, method, context.Background(), types.MessageID("shutdown-"+string(method)), SendRequestExtra{NoRetry: true})
			receiveFrame(t, sock)
			cli.Disconnect()
			res := receiveResult(t, result)
			var disconnected *DisconnectedError
			if !errors.As(res.err, &disconnected) || sock.stopCalls.Load() != 1 {
				t.Fatalf("shutdown result=%v stop calls=%d", res.err, sock.stopCalls.Load())
			}
			assertNoWaiters(t, cli)
		})

		t.Run(string(method)+" timeout", func(t *testing.T) {
			sock := newOutboundTestSocket(1)
			cli := newOutboundTestClient(t, sock)
			result := startOutboundSend(t, cli, method, context.Background(), types.MessageID("timeout-"+string(method)), SendRequestExtra{NoRetry: true, Timeout: 10 * time.Millisecond})
			receiveFrame(t, sock)
			if res := receiveResult(t, result); !errors.Is(res.err, ErrMessageTimedOut) {
				t.Fatalf("expected message timeout, got %v", res.err)
			}
			assertNoWaiters(t, cli)
		})
	}
}

func TestRetryReceiptFalseCallbackStopsBeforeSendBoundary(t *testing.T) {
	chat := types.NewJID("10000000000", types.DefaultUserServer)
	id := types.MessageID("receipt-retry")
	sock := newOutboundTestSocket(1)
	cli := &Client{
		Log:                         waLog.Noop,
		socket:                      sock,
		recentMessagesMap:           map[recentMessageKey]RecentMessage{{To: chat, ID: id}: {wa: &waE2E.Message{}}},
		incomingRetryRequestCounter: make(map[incomingRetryKey]int),
		UseRetryMessageStore:        false,
		GetMessageForRetry:          nil,
		PreRetryCallback: func(*events.Receipt, types.MessageID, int, *waE2E.Message) bool {
			return false
		},
	}
	node := &waBinary.Node{Tag: "receipt", Content: []waBinary.Node{{
		Tag:   "retry",
		Attrs: waBinary.Attrs{"id": string(id), "t": "1", "count": "1"},
	}}}
	for device := uint8(1); device <= 8; device++ {
		receipt := &events.Receipt{MessageSource: types.MessageSource{
			Chat:   chat,
			Sender: types.NewADJID("10000000001", 0, device),
		}}
		if err := cli.handleRetryReceipt(context.Background(), receipt, node); err != nil {
			t.Fatalf("device %d: false callback should stop cleanly: %v", device, err)
		}
	}
	if len(cli.incomingRetryRequestCounter) != 8 {
		t.Fatalf("callback did not observe all device keys: %d", len(cli.incomingRetryRequestCounter))
	}
	if len(sock.frames) != 0 {
		t.Fatalf("receipt retry reached send boundary %d times", len(sock.frames))
	}
}

func TestPublicNoRetryCancellationWithLateWriteAndAck(t *testing.T) {
	release := make(chan struct{})
	completed := make(chan struct{}, 1)
	sock := newOutboundTestSocket(1)
	sock.writeRelease = release
	sock.writeCompleted = completed
	cli := newOutboundTestClient(t, sock)
	ctx, cancel := context.WithCancel(context.Background())
	id := types.MessageID("cancel-late-write")
	result := startOutboundSend(t, cli, outboundWhatsApp, ctx, id, SendRequestExtra{NoRetry: true})
	receiveFrame(t, sock)
	cancel()
	if res := receiveResult(t, result); !errors.Is(res.err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", res.err)
	}
	close(release)
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("late underlying write did not complete")
	}
	if routeAck(t, cli, id) {
		t.Fatal("late acknowledgement matched canceled request")
	}
	if len(sock.frames) != 0 {
		t.Fatalf("cancellation produced extra frames: %d", len(sock.frames))
	}
	assertNoWaiters(t, cli)
}

func TestPublicNoRetryParallel32SharedClient(t *testing.T) {
	const sends = 32
	sock := newOutboundTestSocket(sends)
	cli := newOutboundTestClient(t, sock)
	results := make([]<-chan outboundResult, sends)
	for i := range sends {
		method := outboundWhatsApp
		if i%2 == 1 {
			method = outboundFacebook
		}
		id := types.MessageID(fmt.Sprintf("parallel-%02d", i))
		results[i] = startOutboundSend(t, cli, method, context.Background(), id, SendRequestExtra{NoRetry: true})
	}

	seen := make(map[string]struct{}, sends)
	for range sends {
		receiveFrame(t, sock)
		cli.responseWaitersLock.Lock()
		if len(cli.responseWaiters) != 1 {
			cli.responseWaitersLock.Unlock()
			t.Fatalf("active response waiters=%d, want 1", len(cli.responseWaiters))
		}
		var id string
		for id = range cli.responseWaiters {
		}
		cli.responseWaitersLock.Unlock()
		if _, duplicate := seen[id]; duplicate {
			t.Fatalf("duplicate message id %q", id)
		}
		seen[id] = struct{}{}
		if !routeAck(t, cli, types.MessageID(id)) {
			t.Fatalf("production response router did not match %q", id)
		}
	}

	var wg sync.WaitGroup
	errs := make(chan error, sends)
	for _, result := range results {
		wg.Add(1)
		go func(result <-chan outboundResult) {
			defer wg.Done()
			if res := <-result; res.err != nil {
				errs <- res.err
			}
		}(result)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("parallel public send failed: %v", err)
	}
	if len(seen) != sends {
		t.Fatalf("observed %d frames, want %d", len(seen), sends)
	}
	assertNoWaiters(t, cli)
}
