package whatsapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/crypto/cbc"
)

type websocketWrapper struct {
	sync.Mutex
	sync.WaitGroup
	conn   *websocket.Conn
	ctx    context.Context
	cancel func()

	pingInKeepalive int
}

func (wsw *websocketWrapper) countTimeout() {
	if wsw.pingInKeepalive < 10 {
		wsw.pingInKeepalive++
	}
}

func newWebsocketWrapper(conn *websocket.Conn) *websocketWrapper {
	ctx, cancel := context.WithCancel(context.Background())
	return &websocketWrapper{
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (wsw *websocketWrapper) write(messageType int, data []byte) error {
	if wsw.conn == nil {
		return ErrInvalidWebsocket
	}
	wsw.Lock()
	err := wsw.conn.WriteMessage(messageType, data)
	wsw.Unlock()

	if err != nil {
		return fmt.Errorf("error writing to websocket: %w", err)
	}

	return nil
}

func (wac *Conn) writeJSON(data []interface{}) (<-chan string, error) {
	ch, _, err := wac.writeJSONRetry(data, false)
	return ch, err
}

//writeJSON enqueues a json message into the writeChan
func (wac *Conn) writeJSONRetry(data []interface{}, isResendable bool) (<-chan string, ResendFunc, error) {
	ch := make(chan string, 1)

	d, err := json.Marshal(data)
	if err != nil {
		close(ch)
		return ch, nil, err
	}

	ts := time.Now().Unix()
	messageTag := fmt.Sprintf("%d.--%d", ts, wac.msgCount)
	bytes := []byte(fmt.Sprintf("%s,%s", messageTag, d))

	if wac.timeTag == "" {
		tss := fmt.Sprintf("%d", ts)
		wac.timeTag = tss[len(tss)-3:]
	}

	resend := func() error {
		return wac.ws.write(websocket.TextMessage, bytes)
	}

	wac.listener.add(ch, resend, isResendable, messageTag)

	err = resend()
	if err != nil {
		wac.listener.pop(messageTag)
		close(ch)
		return ch, nil, err
	}

	wac.msgCount++
	return ch, resend, nil
}

func (wac *Conn) writeBinary(node binary.Node, metric metric, flag flag, messageTag string) (<-chan string, error) {
	ch, _, err := wac.writeBinaryRetry(node, metric, flag, messageTag, false)
	return ch, err
}

func (wac *Conn) writeBinaryRetry(node binary.Node, metric metric, flag flag, messageTag string, isResendable bool) (<-chan string, ResendFunc, error) {
	ch := make(chan string, 1)

	if len(messageTag) < 2 {
		close(ch)
		return ch, nil, ErrMissingMessageTag
	}

	data, err := wac.encryptBinaryMessage(node)
	if err != nil {
		close(ch)
		return ch, nil, fmt.Errorf("encryptBinaryMessage(node) failed: %w", err)
	}

	bytes := []byte(messageTag + ",")
	bytes = append(bytes, byte(metric), byte(flag))
	bytes = append(bytes, data...)

	resend := func() error {
		return wac.ws.write(websocket.BinaryMessage, bytes)
	}

	wac.listener.add(ch, resend, isResendable, messageTag)

	err = resend()
	if err != nil {
		wac.listener.pop(messageTag)
		close(ch)
		return ch, nil, fmt.Errorf("failed to write message: %w", err)
	}

	wac.msgCount++
	return ch, resend, nil
}

func (wac *Conn) encryptBinaryMessage(node binary.Node) (data []byte, err error) {
	b, err := binary.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("binary node marshal failed: %w", err)
	}

	cipher, err := cbc.Encrypt(wac.session.EncKey, nil, b)
	if err != nil {
		return nil, fmt.Errorf("encrypt failed: %w", err)
	}

	h := hmac.New(sha256.New, wac.session.MacKey)
	h.Write(cipher)
	hash := h.Sum(nil)

	data = append(data, hash[:32]...)
	data = append(data, cipher...)

	return data, nil
}
