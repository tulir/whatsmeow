package whatsapp_connection

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/crypto/cbc"
	"github.com/gorilla/websocket"
	"net/http"
	"strings"
	"time"
)

type Metric byte

const (
	DEBUG_LOG Metric = iota + 1
	QUERY_RESUME
	QUERY_RECEIPT
	QUERY_MEDIA
	QUERY_CHAT
	QUERY_CONTACTS
	QUERY_MESSAGES
	PRESENCE
	PRESENCE_SUBSCRIBE
	GROUP
	READ
	CHAT
	RECEIVED
	PIC
	STATUS
	MESSAGE
	QUERY_ACTIONS
	BLOCK
	QUERY_GROUP
	QUERY_PREVIEW
	QUERY_EMOJI
	QUERY_MESSAGE_INFO
	SPAM
	QUERY_SEARCH
	QUERY_IDENTITY
	QUERY_URL
	PROFILE
	CONTACT
	QUERY_VCARD
	QUERY_STATUS
	QUERY_STATUS_UPDATE
	PRIVACY_STATUS
	QUERY_LIVE_LOCATIONS
	LIVE_LOCATION
	QUERY_VNAME
	QUERY_LABELS
	CALL
	QUERY_CALL
	QUERY_QUICK_REPLIES
)

type Flag byte

const (
	IGNORE Flag = 1 << (7 - iota)
	ACKREQUEST
	AVAILABLE
	NOTAVAILABLE
	EXPIRES
	SKIPOFFLINE
)

type conn struct {
	wsConn     *websocket.Conn
	session    *Session
	listener   map[string]chan string
	handler    []Handler
	msgCount   int
	msgTimeout time.Duration
}

func NewConn(timeout time.Duration) (*conn, error) {
	dialer := &websocket.Dialer{
		ReadBufferSize:  25 * 1024 * 1024,
		WriteBufferSize: 10 * 1024 * 1024,
	}

	headers := http.Header{"Origin": []string{"https://web.whatsapp.com"}}
	wsConn, _, err := dialer.Dial("wss://w3.web.whatsapp.com/ws", headers)
	if err != nil {
		return nil, fmt.Errorf("couldn't dial whatsapp web websocket: %v", err)
	}

	wac := &conn{wsConn, nil, make(map[string]chan string), make([]Handler, 0), 0, timeout}

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

func (wac *conn) writeBinary(node binary.Node, metric Metric, flag Flag, tag string) (<-chan string, error) {
	if len(tag) < 2 {
		return nil, fmt.Errorf("no tag specified or to short")
	}
	b, err := binary.Marshal(node)
	if err != nil {
		return nil, err
	}

	cipher, err := cbc.Encrypt(wac.session.EncKey, b)
	if err != nil {
		return nil, err
	}

	h := hmac.New(sha256.New, wac.session.MacKey)
	h.Write(cipher)
	hash := h.Sum(nil)

	bin := []byte(tag + ",")
	bin = append(bin, byte(metric), byte(flag))
	bin = append(bin, hash[:32]...)
	bin = append(bin, cipher...)

	ch := make(chan string, 1)
	wac.listener[tag] = ch

	if err = wac.wsConn.WriteMessage(websocket.BinaryMessage, bin); err != nil {
		return nil, err
	}

	wac.msgCount++
	return ch, nil
}

func (wac *conn) readPump() {
	defer wac.wsConn.Close()

	for {
		msgType, msg, err := wac.wsConn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				wac.handle(fmt.Errorf("unexpected websocket close: %v", err))
			}
			break
		}

		data := strings.SplitN(string(msg), ",", 2)

		if wac.listener[data[0]] != nil && len(data[1]) > 0 {
			wac.listener[data[0]] <- data[1]
			delete(wac.listener, data[0])
		} else if msgType == 2 && wac.session != nil && wac.session.EncKey != nil {
			message, err := wac.decryptBinaryMessage([]byte(data[1][:32]))
			if err != nil {
				wac.handle(fmt.Errorf("error decoding binary: %v", err))
				continue
			}

			wac.dispatch(message)
		} else {
			if len(data[1]) > 0 {
				wac.handle(string(data[1]))
			}
		}

	}
}

func (wac *conn) decryptBinaryMessage(msg []byte) (*binary.Node, error) {
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
