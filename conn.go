//Package whatsapp provides a developer API to interact with the WhatsAppWeb-Servers.
package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/crypto/cbc"
	"github.com/gorilla/websocket"
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
	wsConn *websocket.Conn

	wsClose   chan struct{}
	connected bool
	wg        sync.WaitGroup

	wsWriteMutex   sync.RWMutex
	session        *Session
	listener       map[string]chan string
	listenerMutex  sync.RWMutex
	handler        []Handler
	msgCount       int
	msgTimeout     time.Duration
	Info           *Info
	Store          *Store
	ServerLastSeen time.Time

	longClientName  string
	shortClientName string
}

type wsMsg struct {
	messageType int
	data        []byte
}

/*
Creates a new connection with a given timeout. The websocket connection to the WhatsAppWeb servers get´s established.
The goroutine for handling incoming messages is started
*/
func NewConn(timeout time.Duration) *Conn {
	wac := &Conn{
		wsConn:        nil, // will be set in connect()
		wsClose:       nil, //will be set in connect()
		wsWriteMutex:  sync.RWMutex{},
		listener:      make(map[string]chan string),
		listenerMutex: sync.RWMutex{},
		handler:       make([]Handler, 0),
		msgCount:      0,
		msgTimeout:    timeout,
		Store:         newStore(),

		longClientName:  "github.com/rhymen/go-whatsapp",
		shortClientName: "go-whatsapp",
	}
	return wac
}

// connect should be guarded with wsWriteMutex
func (wac *Conn) Connect() (err error) {
	if wac.connected {
		return errors.New("already connected")
	}
	wac.connected = true
	defer func() { // set connected to false on error
		if err != nil {
			wac.connected = false
		}
	}()

	dialer := &websocket.Dialer{
		ReadBufferSize:   25 * 1024 * 1024,
		WriteBufferSize:  10 * 1024 * 1024,
		HandshakeTimeout: wac.msgTimeout,
	}

	headers := http.Header{"Origin": []string{"https://web.whatsapp.com"}}
	wsConn, _, err := dialer.Dial("wss://w3.web.whatsapp.com/ws", headers)
	if err != nil {
		return fmt.Errorf("couldn't dial whatsapp web websocket: %v", err)
	}

	wsConn.SetCloseHandler(func(code int, text string) error {
		// from default CloseHandler
		message := websocket.FormatCloseMessage(code, "")
		err := wsConn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))

		// our close handling
		switch code {
		case websocket.CloseNormalClosure:
			wac.handle(errors.New("server closed connection, normal"))
		case websocket.CloseGoingAway:
			wac.handle(errors.New("server closed connection, going away"))
		default:
			wac.handle(fmt.Errorf("connection closed: %v, %v", code, text))
		}
		return err
	})

	wac.wsClose = make(chan struct{})
	wac.wg.Add(2)
	go wac.readPump()
	go wac.keepAlive(20000, 90000)

	wac.wsConn = wsConn
	return nil
}

func (wac *Conn) Disconnect() error {
	if !wac.connected {
		return errors.New("not connected")
	}
	wac.connected = false

	close(wac.wsClose) //signal close
	wac.wg.Wait()      //wait for close

	err := wac.wsConn.Close()
	wac.wsConn = nil
	return err
}

//writeJson enqueues a json message into the writeChan
func (wac *Conn) writeJson(data []interface{}) (<-chan string, error) {
	d, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	ts := time.Now().Unix()
	messageTag := fmt.Sprintf("%d.--%d", ts, wac.msgCount)
	bytes := fmt.Sprintf("%s,%s", messageTag, d)

	ch := make(chan string, 1)

	wac.listenerMutex.Lock()
	wac.listener[messageTag] = ch
	wac.listenerMutex.Unlock()

	msg := wsMsg{
		messageType: websocket.TextMessage,
		data:        []byte(bytes),
	}
	if err = wac.write(msg); err != nil {
		wac.listenerMutex.Lock()
		delete(wac.listener, messageTag)
		wac.listenerMutex.Unlock()
		return nil, err
	}

	wac.msgCount++
	return ch, nil
}

func (wac *Conn) writeBinary(node binary.Node, metric metric, flag flag, messageTag string) (<-chan string, error) {
	if len(messageTag) < 2 {
		return nil, fmt.Errorf("no messageTag specified or to short")
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

	data := []byte(messageTag + ",")
	data = append(data, byte(metric), byte(flag))
	data = append(data, hash[:32]...)
	data = append(data, cipher...)

	ch := make(chan string, 1)

	wac.listenerMutex.Lock()
	wac.listener[messageTag] = ch
	wac.listenerMutex.Unlock()

	msg := wsMsg{
		messageType: websocket.BinaryMessage,
		data:        data,
	}
	if err = wac.write(msg); err != nil {
		wac.listenerMutex.Lock()
		delete(wac.listener, messageTag)
		wac.listenerMutex.Unlock()
		return nil, err
	}

	wac.msgCount++
	return ch, nil
}

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

func (wac *Conn) write(msg wsMsg) error {
	wac.wsWriteMutex.Lock()
	defer wac.wsWriteMutex.Unlock()

	if err := wac.wsConn.WriteMessage(msg.messageType, msg.data); err != nil {
		return fmt.Errorf("error writing to socket: %v\n", err)
	}
	return nil
}

func (wac *Conn) sendKeepAlive() error {
	msg := wsMsg{
		messageType: websocket.TextMessage,
		data:        []byte("?,,"),
	}
	return wac.write(msg)
}

func (wac *Conn) keepAlive(minIntervalMs int, maxIntervalMs int) {
	defer wac.wg.Done()

	for {
		err := wac.sendKeepAlive()
		if err != nil {
			wac.handle(fmt.Errorf("keepAlive failed: %v", err))
		}
		interval := rand.Intn(maxIntervalMs-minIntervalMs) + minIntervalMs
		select {
		case <-time.After(time.Duration(interval) * time.Millisecond):
		case <-wac.wsClose:
			return
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
