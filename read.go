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
	"time"

	"github.com/gorilla/websocket"

	"go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/crypto/cbc"
)

type ResendFunc func() error

type inputWaiter struct {
	ch     chan<- string
	resend ResendFunc
	tags   []string

	addedAt     time.Time
	needsResend bool
}

type listenerWrapper struct {
	sync.Mutex
	waiters map[string]*inputWaiter
}

func newListenerWrapper() *listenerWrapper {
	return &listenerWrapper{waiters: make(map[string]*inputWaiter)}
}

func (lw *listenerWrapper) addWaiter(iw *inputWaiter, messageTag string, addTagToList bool) {
	lw.Lock()
	lw.waiters[messageTag] = iw
	if addTagToList {
		iw.tags = append(iw.tags, messageTag)
	}
	if iw.addedAt.IsZero() {
		iw.addedAt = time.Now()
	}
	lw.Unlock()
}

func (lw *listenerWrapper) add(ch chan<- string, resend func() error, isResendable bool, messageTag string) {
	if !isResendable {
		resend = nil
	}
	lw.addWaiter(&inputWaiter{ch: ch, resend: resend, tags: []string{messageTag}}, messageTag, false)
}

func (lw *listenerWrapper) pop(messageTag string) (*inputWaiter, bool) {
	lw.Lock()
	listener, hasListener := lw.waiters[messageTag]
	if hasListener {
		for _, tag := range listener.tags {
			if tagListener, ok := lw.waiters[tag]; ok && tagListener == listener {
				delete(lw.waiters, tag)
			}
		}
	}
	lw.Unlock()
	return listener, hasListener
}

type resendableMessages []*inputWaiter

func (rsm resendableMessages) Len() int {
	return len(rsm)
}

func (rsm resendableMessages) Swap(i, j int) {
	rsm[i], rsm[j] = rsm[j], rsm[i]
}

func (rsm resendableMessages) Less(i, j int) bool {
	return rsm[i].addedAt.Before(rsm[j].addedAt)
}

func (lw *listenerWrapper) getResendables() (rsm resendableMessages) {
	lw.Lock()
	newWaiters := make(map[string]*inputWaiter)
	for msgID, waiter := range lw.waiters {
		if waiter.resend != nil {
			rsm = append(rsm, waiter)
			newWaiters[msgID] = waiter
		} else if !waiter.needsResend {
			newWaiters[msgID] = waiter
		}
	}
	lw.waiters = newWaiters
	lw.Unlock()
	sort.Sort(&rsm)
	return rsm
}

func (lw *listenerWrapper) onReconnect() {
	lw.Lock()
	for _, waiter := range lw.waiters {
		waiter.needsResend = true
	}
	lw.Unlock()
}

func (wac *Conn) readPump(ws *websocketWrapper) {
	wac.log.Debugfln("Websocket read pump starting %p", ws)
	defer func() {
		wac.log.Debugfln("Websocket read pump exiting %p", ws)
		ws.Done()
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
				go wac.handle(&ErrConnectionFailed{Err: readErr})
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

	listener, hasListener := wac.listener.pop(data[0])
	if hasListener {
		select {
		case listener.ch <- data[1]:
			close(listener.ch)
		default:
			wac.log.Debugln("Channel for response to", data[0], "is no longer receiving")
		}
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
		wac.dispatch(data[0], message)
	} else { //RAW json status updates
		wac.handleJSONMessage(data[1])
		wac.handle(RawJSONMessage{json.RawMessage(data[1]), data[0]})
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
	message, err := binary.Unmarshal(d, false)
	if err != nil {
		return nil, fmt.Errorf("could not decode binary: %w", err)
	}

	return message, nil
}
