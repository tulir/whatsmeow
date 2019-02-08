package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/crypto/cbc"
	"github.com/gorilla/websocket"
	"io"
	"io/ioutil"
	"strings"
)

func (wac *Conn) readPump() {
	defer wac.wg.Done()

	var readErr error
	var msgType int
	var reader io.Reader

	for {
		readerFound := make(chan struct{})
		go func() {
			msgType, reader, readErr = wac.ws.conn.NextReader()
			close(readerFound)
		}()
		select {
		case <-readerFound:
			if readErr != nil {
				if websocket.IsUnexpectedCloseError(readErr, websocket.CloseGoingAway) {
					wac.handle(fmt.Errorf("unexpected websocket close: %v", readErr))
					return
				}
				wac.handle(fmt.Errorf("error reading message: %v", readErr))
				continue
			}
			msg, err := ioutil.ReadAll(reader)
			if err != nil {
				wac.handle(fmt.Errorf("error reading message: %v", err))
				continue
			}
			wac.processReadData(msgType, msg)
		case <-wac.ws.close:
			return
		}
	}
}

func (wac *Conn) processReadData(msgType int, msg []byte) {
	data := strings.SplitN(string(msg), ",", 2)

	if data[0][0] == '!' { //Keep-Alive Timestamp
		data = append(data, data[0][1:]) //data[1]
		data[0] = "!"
	}

	wac.listener.RLock()
	listener, hasListener := wac.listener.m[data[0]]
	wac.listener.RUnlock()

	if len(data[1]) == 0 {
		return
	} else if hasListener {
		// listener only exists for TextMessages
		// And query messages out of contact.go
		listener <- data[1]

		wac.listener.Lock()
		delete(wac.listener.m, data[0])
		wac.listener.Unlock()
	} else if msgType == 2 && wac.session != nil && wac.session.EncKey != nil {
		message, err := wac.decryptBinaryMessage([]byte(data[1]))
		if err != nil {
			wac.handle(fmt.Errorf("error decoding binary: %v", err))
			return
		}

		wac.dispatch(message)
	} else {
		wac.handle(string(data[1]))
	}
}

func (wac *Conn) decryptBinaryMessage(msg []byte) (*binary.Node, error) {
	//message validation
	h2 := hmac.New(sha256.New, wac.session.MacKey)
	h2.Write([]byte(msg[32:]))
	if !hmac.Equal(h2.Sum(nil), msg[:32]) {
		return nil, fmt.Errorf("message received with invalid hmac")
	}

	// message decrypt
	d, err := cbc.Decrypt(wac.session.EncKey, nil, msg[32:])
	if err != nil {
		return nil, fmt.Errorf("error decrypting message with AES: %v", err)
	}

	// message unmarshal
	message, err := binary.Unmarshal(d)
	if err != nil {
		return nil, fmt.Errorf("error decoding binary: %v", err)
	}

	return message, nil
}
