//Package whatsapp provides a developer API to interact with the WhatsAppWeb-Servers.
package whatsapp

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

type metric byte

const (
	debugLog metric = iota + 1
	queryResume
	queryReceipt
	queryMedia
	queryChat
	queryContacts
	queryMessages
	presence
	presenceSubscribe
	group
	read
	chat
	received
	pic
	status
	message
	queryActions
	block
	queryGroup
	queryPreview
	queryEmoji
	queryMessageInfo
	spam
	querySearch
	queryIdentity
	queryUrl
	profile
	contact
	queryVcard
	queryStatus
	queryStatusUpdate
	privacyStatus
	queryLiveLocations
	liveLocation
	queryVname
	queryLabels
	call
	queryCall
	queryQuickReplies
)

type flag byte

const (
	ignore flag = 1 << (7 - iota)
	ackRequest
	available
	notAvailable
	expires
	skipOffline
)

/*
Conn is created by NewConn. Interacting with the initialized Conn is the main way of interacting with our package.
It holds all necessary information to make the package work internally.
*/
type Conn struct {
	wsConn     *websocket.Conn
	session    *Session
	listener   map[string]chan string
	handler    []Handler
	msgCount   int
	msgTimeout time.Duration
}

/*
Creates a new connection with a given timeout. The websocket connection to the WhatsAppWeb servers get´s established.
The goroutine for handling incoming messages is started
*/
func NewConn(timeout time.Duration) (*Conn, error) {
	dialer := &websocket.Dialer{
		ReadBufferSize:   25 * 1024 * 1024,
		WriteBufferSize:  10 * 1024 * 1024,
		HandshakeTimeout: timeout,
	}

	headers := http.Header{"Origin": []string{"https://web.whatsapp.com"}}
	wsConn, _, err := dialer.Dial("wss://w3.web.whatsapp.com/ws", headers)
	if err != nil {
		return nil, fmt.Errorf("couldn't dial whatsapp web websocket: %v", err)
	}

	wac := &Conn{wsConn, nil, make(map[string]chan string), make([]Handler, 0), 0, timeout}

	go wac.readPump()

	return wac, nil
}

func (wac *Conn) write(data []interface{}) (<-chan string, error) {
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

func (wac *Conn) writeBinary(node binary.Node, metric metric, flag flag, tag string) (<-chan string, error) {
	if len(tag) < 2 {
		return nil, fmt.Errorf("no tag specified or to short")
	}
	b, err := binary.Marshal(node)
	if err != nil {
		return nil, err
	}

	cipher, err := cbc.Encrypt(wac.session.EncKey, nil, b)
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

func (wac *Conn) readPump() {
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
			message, err := wac.decryptBinaryMessage([]byte(data[1]))
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
