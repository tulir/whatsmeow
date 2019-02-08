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

	wsConnOK       bool
	wsConnMutex    sync.RWMutex
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
		wsConnMutex:   sync.RWMutex{},
		listener:      make(map[string]chan string),
		listenerMutex: sync.RWMutex{},
		writeChan:     make(chan wsMsg),
		handler:       make([]Handler, 0),
		msgCount:      0,
		msgTimeout:    timeout,
		Store:         newStore(),

		longClientName:  "github.com/rhymen/go-whatsapp",
		shortClientName: "go-whatsapp",
	}
	return wac
}

func (wac *Conn) isConnected() bool {
	wac.wsConnMutex.RLock()
	defer wac.wsConnMutex.RUnlock()
	if wac.wsConn == nil {
		return false
	}
	if wac.wsConnOK {
		return true
	}

	// just send a keepalive to test the connection
	wac.sendKeepAlive()

	// this method is expected to be called by loops. So we can just return false
	return false
}

// connect should be guarded with wsConnMutex
func (wac *Conn) Connect() error {
	if wac.connected {
		return errors.New("already connected")
	}
	wac.connected = true

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

	//TODO
	wsConn.SetCloseHandler(func(code int, text string) error {
		fmt.Fprintf(os.Stderr, "websocket connection closed(%d, %s)\n", code, text)

		// from default CloseHandler
		message := websocket.FormatCloseMessage(code, "")
		wsConn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))

		// our close handling
		if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			fmt.Println("Trigger reconnect")
			go wac.reconnect()
		}
		return nil
	})

	wac.wsClose = make(chan struct{})
	wac.wg.Add(3)
	go wac.readPump()
	go wac.writePump()
	go wac.keepAlive(20000, 90000)

	wac.wsConn = wsConn
	wac.wsConnOK = true
	return nil
}

func (wac *Conn) Disconnect() error {
	if !wac.connected {
		return errors.New("not connected")
	}
	wac.wsConnOK = false

	close(wac.wsClose) //signal close
	wac.wg.Wait()      //wait for close

	return wac.wsConn.Close()
}

// reconnect should be run as go routine
func (wac *Conn) reconnect() {
	return
	wac.wsConnMutex.Lock()
	wac.wsConn.Close()
	wac.wsConn = nil
	wac.wsConnOK = false
	wac.wsConnMutex.Unlock()

	// wait up to 60 seconds and then reconnect. As writePump should send immediately, it might
	// reconnect as well. So we check its existance before reconnecting
	for !wac.isConnected() {
		time.Sleep(time.Duration(rand.Intn(60)) * time.Second)

		wac.wsConnMutex.Lock()
		if wac.wsConn == nil {
			if err := wac.Connect(); err != nil {
				fmt.Fprintf(os.Stderr, "could not reconnect to websocket: %v\n", err)
			}
		}
		wac.wsConnMutex.Unlock()
	}
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
				wac.wsConnOK = false
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
	wac.wsConnOK = true

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

func (wac *Conn) writePump() {
	defer wac.wg.Done()

	for {
		select {
		case <-wac.wsClose:
			return
		case msg := <-wac.writeChan:
			/*
				for !wac.isConnected() {
					// reconnect to send the message ASAP
					wac.wsConnMutex.Lock()
					if wac.wsConn == nil {
						if err := wac.Connect(); err != nil {
							fmt.Fprintf(os.Stderr, "could not reconnect to websocket: %v\n", err)
						}
					}
					wac.wsConnMutex.Unlock()
					if !wac.isConnected() {
						// reconnecting failed. Sleep for a while and try again afterwards
						time.Sleep(time.Duration(rand.Intn(5)) * time.Second)
					}
				}
			*/
			if err := wac.wsConn.WriteMessage(msg.messageType, msg.data); err != nil {
				fmt.Fprintf(os.Stderr, "error writing to socket: %v\n", err)
				wac.wsConnOK = false
				// add message to channel again to no loose it
				go func() {
					wac.writeChan <- msg
				}()
			}
		}
	}
}

func (wac *Conn) sendKeepAlive() {
	// whatever issues might be there allow sending this message
	wac.wsConnOK = true
	wac.writeChan <- wsMsg{
		messageType: websocket.TextMessage,
		data:        []byte("?,,"),
	}
}

func (wac *Conn) keepAlive(minIntervalMs int, maxIntervalMs int) {
	defer wac.wg.Done()

	for {
		wac.sendKeepAlive()
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
