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
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
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
	wsConn         *websocket.Conn
	session        *Session
	listener       map[string]chan string
	listenerMutex  sync.RWMutex
	writeChan      chan wsMsg
	handler        []Handler
	msgCount       int
	msgTimeout     time.Duration
	Info           *Info
	Store          *Store
	ServerLastSeen time.Time
}

type wsMsg struct {
	messageType int
	data        []byte
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

	wac := &Conn{
		wsConn:        wsConn,
		listener:      make(map[string]chan string),
		listenerMutex: sync.RWMutex{},
		writeChan:     make(chan wsMsg),
		handler:       make([]Handler, 0),
		msgCount:      0,
		msgTimeout:    timeout,
		Store:         newStore(),
	}

	go wac.readPump()
	go wac.writePump()
	go wac.keepAlive(20000, 90000)

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

	ch := make(chan string, 1)

	wac.listenerMutex.Lock()
	wac.listener[messageTag] = ch
	wac.listenerMutex.Unlock()

	wac.writeChan <- wsMsg{websocket.TextMessage, []byte(msg)}

	wac.msgCount++
	return ch, nil
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

	data := []byte(tag + ",")
	data = append(data, byte(metric), byte(flag))
	data = append(data, hash[:32]...)
	data = append(data, cipher...)

	ch := make(chan string, 1)

	wac.listenerMutex.Lock()
	wac.listener[tag] = ch
	wac.listenerMutex.Unlock()

	msg := wsMsg{websocket.BinaryMessage, data}
	wac.writeChan <- msg

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

		//Kepp-Alive Timestmap
		if data[0][0] == '!' {
			msecs, err := strconv.ParseInt(data[0][1:], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error converting time string to uint: %v\n", err)
				continue
			}
			wac.ServerLastSeen = time.Unix(msecs/1000, (msecs%1000)*int64(time.Millisecond))
			continue
		}

		wac.listenerMutex.RLock()
		listener, hasListener := wac.listener[data[0]]
		wac.listenerMutex.RUnlock()

		if hasListener && len(data[1]) > 0 {
			listener <- data[1]

			wac.listenerMutex.Lock()
			delete(wac.listener, data[0])
			wac.listenerMutex.Unlock()
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

func (wac *Conn) writePump() {
	for msg := range wac.writeChan {
		if err := wac.wsConn.WriteMessage(msg.messageType, msg.data); err != nil {
			fmt.Fprintf(os.Stderr, "error writing to socket: %v", err)
		}
	}
}

func (wac *Conn) keepAlive(minIntervalMs int, maxIntervalMs int) {
	for {
		wac.writeChan <- wsMsg{
			messageType: websocket.TextMessage,
			data:        []byte("?,,"),
		}
		interval := rand.Intn(maxIntervalMs-minIntervalMs) + minIntervalMs
		<-time.After(time.Duration(interval) * time.Millisecond)
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
