package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/Rhymen/go-whatsapp/crypto/cbc"
	"github.com/Rhymen/go-whatsapp/whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/whatsapp/binary/composing"
	"github.com/Rhymen/go-whatsapp/whatsapp/binary/parsing"
	"github.com/gorilla/websocket"
	"net/http"
	"os"
	"strings"
	"time"
)

type conn struct {
	wsConn     *websocket.Conn
	session    *Session
	listener   map[string]chan string
	dispatcher *dispatcher
	msgCount   int
	msgTimeout time.Duration
}

func NewConn() (*conn, error) {
	dialer := &websocket.Dialer{
		ReadBufferSize:  25 * 1024 * 1024,
		WriteBufferSize: 10 * 1024 * 1024,
	}

	headers := http.Header{}
	headers.Add("Origin", "https://web.whatsapp.com")

	wsConn, _, err := dialer.Dial("wss://w3.web.whatsapp.com/ws", headers)
	if err != nil {
		return nil, fmt.Errorf("ws dial error: %v", err)
	}

	wac := &conn{wsConn, nil, make(map[string]chan string), newDispatcher(), 0, 5 * time.Second}

	go wac.dispatcher.dispatch()

	go wac.readPump()

	return wac, nil
}

func (wac *conn) write(data []interface{}) (<-chan string, error) {
	d, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	ts := time.Now().Unix()
	messageTag := fmt.Sprintf("%d.--%d", ts, wac.msgCount)
	msg := fmt.Sprintf("%s,%s", messageTag, d)

	wac.listener[messageTag] = make(chan string, 1)

	if err = wac.wsConn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		delete(wac.listener, messageTag)
		return nil, err
	}

	wac.msgCount++
	return wac.listener[messageTag], nil
}

func (wac *conn) writeBinary(node binary.Node, metric binary.Metric, flag binary.Flag, tag string) error {
	if len(tag) < 2 {
		return fmt.Errorf("no tag specified or to short")
	}
	b, err := composing.Marshal(node)
	if err != nil {
		return err
	}

	cipher, err := cbc.Encrypt(wac.session.EncKey, b)
	if err != nil {
		return err
	}

	h := hmac.New(sha256.New, wac.session.MacKey)
	h.Write(cipher)
	hash := h.Sum(nil)

	bin := []byte(tag + ",")
	bin = append(bin, byte(metric), byte(flag))
	bin = append(bin, hash[:32]...)
	bin = append(bin, cipher...)

	//ch := make(chan string, 1)
	//wac.listener[tag] = ch
	if err = wac.wsConn.WriteMessage(websocket.BinaryMessage, bin); err != nil {
		delete(wac.listener, tag)
		return err
	}

	//check 200
	/*
		select {
		case r := <-ch:
			var resp map[string]interface{}
			if err = json.Unmarshal([]byte(r), &resp); err != nil {
				return fmt.Errorf("error decoding login connResp: %v\n", err)
			}
			if int(resp["status"].(float64)) != 200 {
				return fmt.Errorf("message sending responded with %d", resp["status"])
			}
		case <-time.After(wac.msgTimeout):
			return fmt.Errorf("sending timed out")
		}
	*/
	return nil
}

func (wac *conn) readPump() {
	defer wac.wsConn.Close()

	for {
		msgType, msg, err := wac.wsConn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				fmt.Printf("unexpected websocket close: %v", err)
			}
			break
		}

		data := strings.SplitN(string(msg), ",", 2)

		if wac.listener[data[0]] != nil {
			wac.listener[data[0]] <- data[1]
			delete(wac.listener, data[0])
			// fmt.Printf("[] received msg: %v\n\n", data[1])
		} else if msgType == 2 && wac.session.EncKey != nil {
			//message validation
			h2 := hmac.New(sha256.New, wac.session.MacKey)
			h2.Write([]byte(data[1][32:]))
			if !hmac.Equal(h2.Sum(nil), []byte(data[1][:32])) {
				fmt.Fprint(os.Stderr, "invalid hmac\n\n")
				continue
			}

			// message decrypt
			d, err := cbc.Decrypt(wac.session.EncKey, nil, []byte(data[1])[32:])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error decryptAes data: %v\n", err)
				continue
			}

			// message unmarshal
			message, err := parsing.Unmarshal(d)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error parsing binary: %v\n", message)
				continue
			}

			// fmt.Printf("decoded %d binary message\n", message)
			wac.dispatcher.toDispatch <- message
		} else {
			fmt.Printf("[] %v discarded msg: %v\n\n", msgType, string(msg))
		}

	}
}
