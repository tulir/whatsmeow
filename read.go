package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/crypto/cbc"
)

type ResendFunc func() error

type inputWaiter struct {
	ch     chan<- string
	resend ResendFunc
}

type listenerWrapper struct {
	sync.RWMutex
	waiters map[string]inputWaiter
}

func newListenerWrapper() *listenerWrapper {
	return &listenerWrapper{waiters: make(map[string]inputWaiter)}
}

func (lw *listenerWrapper) add(ch chan<- string, resend func() error, isResendable bool, messageTag string) {
	lw.Lock()
	if !isResendable {
		resend = nil
	}
	lw.waiters[messageTag] = inputWaiter{ch, resend}
	lw.Unlock()
}

func (lw *listenerWrapper) remove(messageTag string) {
	lw.Lock()
	delete(lw.waiters, messageTag)
	lw.Unlock()
}

func (lw *listenerWrapper) get(messageTag string) (chan<- string, bool) {
	lw.RLock()
	listener, hasListener := lw.waiters[messageTag]
	lw.RUnlock()
	return listener.ch, hasListener
}

type resendableMessages struct {
	ids   []string
	funcs []ResendFunc
}

func (rsm *resendableMessages) Len() int {
	return len(rsm.ids)
}

func (rsm *resendableMessages) Swap(i, j int) {
	rsm.funcs[i], rsm.funcs[j] = rsm.funcs[j], rsm.funcs[i]
	rsm.ids[i], rsm.ids[j] = rsm.ids[j], rsm.ids[i]
}

func (rsm *resendableMessages) Less(i, j int) bool {
	return rsm.ids[i] < rsm.ids[j]
}

func (lw *listenerWrapper) onReconnect() (rsm resendableMessages) {
	lw.Lock()
	newWaiters := make(map[string]inputWaiter)
	for msgID, waiter := range lw.waiters {
		if waiter.resend != nil {
			rsm.ids = append(rsm.ids, msgID)
			rsm.funcs = append(rsm.funcs, waiter.resend)
		}
	}
	lw.waiters = newWaiters
	lw.Unlock()
	sort.Sort(&rsm)
	return rsm
}

func (wac *Conn) readPump(ws *websocketWrapper) {
	wac.log.Debugfln("Websocket read pump starting %p", ws)
	defer func() {
		wac.log.Debugfln("Websocket read pump exiting %p", ws)
		ws.Done()
		select {
		case <-ws.ctx.Done():
			wac.log.Debugln("Not disconnecting websocket as read pump was closed by disconnect")
		default:
			wac.log.Debugln("Disconnecting websocket due to read pump close")
			_ = wac.Disconnect()
		}
	}()

	var readErr error
	var msgType int
	var reader io.Reader

	for {
		readerFound := make(chan struct{})
		go func() {
			msgType, reader, readErr = ws.conn.NextReader()
			close(readerFound)
		}()
		select {
		case <-readerFound:
			if readErr != nil {
				wac.log.Errorln("Error getting next websocket reader:", readErr)
				wac.handle(&ErrConnectionFailed{Err: readErr})
				return
			}
			msg, err := ioutil.ReadAll(reader)
			if err != nil {
				wac.log.Errorln("Error reading message from websocket reader:", err)
				continue
			}
			err = wac.processReadData(msgType, msg)
			if err != nil {
				wac.log.Errorln("Error processing data from websocket:", err)
			}
		case <-ws.ctx.Done():
			return
		}
	}
}

func (wac *Conn) processReadData(msgType int, msg []byte) error {
	data := strings.SplitN(string(msg), ",", 2)

	if data[0][0] == '!' { //Keep-Alive Timestamp
		data = append(data, data[0][1:]) //data[1]
		data[0] = "!"
	}

	if len(data) == 2 && len(data[1]) == 0 {
		// TODO use these request acknowledgements?
		return nil
	}

	if len(data) != 2 || len(data[1]) == 0 {
		return ErrInvalidWsData
	}

	listener, hasListener := wac.listener.get(data[0])

	if hasListener {
		// listener only exists for TextMessages query messages out of contact.go
		// If these binary query messages can be handled another way,
		// then the TextMessages, which are all JSON encoded, can directly
		// be unmarshalled. The listener chan could then be changed from type
		// chan string to something like chan map[string]interface{}. The unmarshalling
		// in several places, especially in session.go, would then be gone.
		select {
		case listener <- data[1]:
			close(listener)
		default:
			wac.log.Debugln("Channel for response to", data[0], "is no longer receiving")
		}
		wac.listener.remove(data[0])
	} else if msgType == websocket.BinaryMessage {
		wac.loginSessionLock.RLock()
		sess := wac.session
		wac.loginSessionLock.RUnlock()
		if sess == nil || sess.MacKey == nil || sess.EncKey == nil {
			return ErrInvalidWsState
		}
		message, err := wac.decryptBinaryMessage([]byte(data[1]))
		if err != nil {
			return fmt.Errorf("error decoding binary: %w", err)
		}
		wac.dispatch(message)
	} else { //RAW json status updates
		wac.handleJSONMessage(data[1])
		wac.handle(json.RawMessage(data[1]))
	}
	return nil
}

func (wac *Conn) decryptBinaryMessage(msg []byte) (*binary.Node, error) {
	//message validation
	h2 := hmac.New(sha256.New, wac.session.MacKey)
	if len(msg) < 33 {
		var response struct {
			Status int `json:"status"`
		}

		if err := json.Unmarshal(msg, &response); err == nil {
			if response.Status == http.StatusNotFound {
				return nil, ErrServerRespondedWith404
			}
			return nil, fmt.Errorf("server responded with %d", response.Status)
		} else {
			return nil, fmt.Errorf("%w: %s", ErrInvalidServerResponse, msg)
		}
	}
	h2.Write(msg[32:])
	if !hmac.Equal(h2.Sum(nil), msg[:32]) {
		return nil, ErrInvalidHmac
	}

	// message decrypt
	d, err := cbc.Decrypt(wac.session.EncKey, nil, msg[32:])
	if err != nil {
		return nil, fmt.Errorf("decrypting message with AES-CBC failed: %w", err)
	}

	// message unmarshal
	message, err := binary.Unmarshal(d)
	if err != nil {
		return nil, fmt.Errorf("could not decode binary: %w", err)
	}

	return message, nil
}
