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
	"os"
	"strconv"
	"strings"
	"time"
)

func (wac *Conn) readPump() {
	defer wac.wg.Done()

	var readErr error
	var msgType int
	var reader io.Reader

	for {
		readerFound := make(chan struct{})
		go func() {
			msgType, reader, readErr = wac.wsConn.NextReader()
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
			wac.process(msgType, reader)
		case <-wac.wsClose:
			return
		}
	}
}

func (wac *Conn) process(msgType int, r io.Reader) {
	msg, err := ioutil.ReadAll(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read: %v", err)
		return
	}

	data := strings.SplitN(string(msg), ",", 2)

	//Kepp-Alive Timestmap
	if data[0][0] == '!' {
		msecs, err := strconv.ParseInt(data[0][1:], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error converting time string to uint: %v\n", err)
			return
		}
		wac.ServerLastSeen = time.Unix(msecs/1000, (msecs%1000)*int64(time.Millisecond))
		return
	}

	wac.listenerMutex.RLock()
	listener, hasListener := wac.listener[data[0]]
	wac.listenerMutex.RUnlock()

	if len(data[1]) == 0 {
		return
	} else if hasListener {
		listener <- data[1]

		wac.listenerMutex.Lock()
		delete(wac.listener, data[0])
		wac.listenerMutex.Unlock()
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
