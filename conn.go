//Package whatsapp provides a developer API to interact with the WhatsAppWeb-Servers.
package whatsapp

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

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
func NewConn(timeout time.Duration) (*Conn, error) {
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
	return wac, wac.connect()
}

// connect should be guarded with wsWriteMutex
func (wac *Conn) connect() (err error) {
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
	wac.session = nil
	return err
}

func (wac *Conn) keepAlive(minIntervalMs int, maxIntervalMs int) {
	defer wac.wg.Done()

	for {
		err := wac.sendKeepAlive()
		if err != nil {
			wac.handle(fmt.Errorf("keepAlive failed: %v", err))
			//TODO: Consequences?
		}
		interval := rand.Intn(maxIntervalMs-minIntervalMs) + minIntervalMs
		select {
		case <-time.After(time.Duration(interval) * time.Millisecond):
		case <-wac.wsClose:
			return
		}
	}
}
