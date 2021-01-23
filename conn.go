//Package whatsapp provides a developer API to interact with the WhatsAppWeb-Servers.
package whatsapp

import (
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
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
	ws       *websocketWrapper
	listener *listenerWrapper

	connected bool
	loggedIn  bool
	wg        *sync.WaitGroup

	session        *Session
	sessionLock    uint32
	handler        []Handler
	msgCount       int
	msgTimeout     time.Duration
	Info           *Info
	Store          *Store
	ServerLastSeen time.Time

	timeTag string // last 3 digits obtained after a successful login takeover

	longClientName  string
	shortClientName string
	clientVersion   string

	loginSessionLock sync.RWMutex
	Proxy            func(*http.Request) (*url.URL, error)

	writerLock sync.RWMutex
}

type websocketWrapper struct {
	sync.Mutex
	conn  *websocket.Conn
	close chan struct{}
}

type listenerWrapper struct {
	sync.RWMutex
	m map[string]chan string
}

/*
Creates a new connection with a given timeout. The websocket connection to the WhatsAppWeb servers get´s established.
The goroutine for handling incoming messages is started
*/
func NewConn(timeout time.Duration) (*Conn, error) {
	return NewConnWithOptions(&Options{
		Timeout: timeout,
	})
}

// NewConnWithProxy Create a new connect with a given timeout and a http proxy.
func NewConnWithProxy(timeout time.Duration, proxy func(*http.Request) (*url.URL, error)) (*Conn, error) {
	return NewConnWithOptions(&Options{
		Timeout: timeout,
		Proxy: proxy,
	})
}

// NewConnWithOptions Create a new connect with a given options.
type Options struct {
	Proxy            func(*http.Request) (*url.URL, error)
	Timeout          time.Duration
	Handler          []Handler
	ShortClientName  string
	LongClientName   string
	ClientVersion    string
	Store            *Store
}
func NewConnWithOptions(opt *Options) (*Conn, error) {
	if opt == nil {
		return nil, ErrOptionsNotProvided
	}
	wac := &Conn{
		handler:    make([]Handler, 0),
		msgCount:   0,
		msgTimeout: opt.Timeout,
		Store:      newStore(),
		longClientName:  "github.com/Rhymen/go-whatsapp",
		shortClientName: "go-whatsapp",
		clientVersion:   "0.1.0",
	}
	if opt.Handler != nil {
		wac.handler = opt.Handler
	}
	if opt.Store != nil {
		wac.Store = opt.Store
	}
	if opt.Proxy != nil {
		wac.Proxy = opt.Proxy
	}
	if len(opt.ShortClientName) != 0 {
		wac.shortClientName = opt.ShortClientName
	}
	if len(opt.LongClientName) != 0 {
		wac.longClientName = opt.LongClientName
	}
	if len(opt.ClientVersion) != 0 {
		wac.clientVersion = opt.ClientVersion
	}
	return wac, wac.connect()
}

// connect should be guarded with wsWriteMutex
func (wac *Conn) connect() (err error) {
	if wac.connected {
		return ErrAlreadyConnected
	}
	wac.connected = true
	defer func() { // set connected to false on error
		if err != nil {
			wac.connected = false
		}
	}()

	dialer := &websocket.Dialer{
		ReadBufferSize:   0,
		WriteBufferSize:  0,
		HandshakeTimeout: wac.msgTimeout,
		Proxy:            wac.Proxy,
	}

	headers := http.Header{"Origin": []string{"https://web.whatsapp.com"}}
	wsConn, _, err := dialer.Dial("wss://web.whatsapp.com/ws", headers)
	if err != nil {
		return errors.Wrap(err, "couldn't dial whatsapp web websocket")
	}

	wsConn.SetCloseHandler(func(code int, text string) error {
		// from default CloseHandler
		message := websocket.FormatCloseMessage(code, "")
		err := wsConn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))

		// our close handling
		_, _ = wac.Disconnect()
		wac.handle(&ErrConnectionClosed{Code: code, Text: text})
		return err
	})

	wac.ws = &websocketWrapper{
		conn:  wsConn,
		close: make(chan struct{}),
	}

	wac.listener = &listenerWrapper{
		m: make(map[string]chan string),
	}

	wac.wg = &sync.WaitGroup{}
	wac.wg.Add(2)
	go wac.readPump()
	go wac.keepAlive(20000, 60000)

	wac.loggedIn = false
	return nil
}

func (wac *Conn) Disconnect() (Session, error) {
	if !wac.connected {
		return Session{}, ErrNotConnected
	}
	wac.connected = false
	wac.loggedIn = false

	close(wac.ws.close) //signal close
	wac.wg.Wait()       //wait for close

	err := wac.ws.conn.Close()
	wac.ws = nil

	if wac.session == nil {
		return Session{}, err
	}
	return *wac.session, err
}

func (wac *Conn) AdminTest() (bool, error) {
	if !wac.connected {
		return false, ErrNotConnected
	}

	if !wac.loggedIn {
		return false, ErrInvalidSession
	}

	result, err := wac.sendAdminTest()
	return result, err
}

func (wac *Conn) keepAlive(minIntervalMs int, maxIntervalMs int) {
	defer wac.wg.Done()

	for {
		err := wac.sendKeepAlive()
		if err != nil {
			wac.handle(errors.Wrap(err, "keepAlive failed"))
			//TODO: Consequences?
		}
		interval := rand.Intn(maxIntervalMs-minIntervalMs) + minIntervalMs
		select {
		case <-time.After(time.Duration(interval) * time.Millisecond):
		case <-wac.ws.close:
			return
		}
	}
}

// IsConnected returns whether the server connection is established or not
func (wac *Conn) IsConnected() bool {
	return wac.connected
}

// GetConnected returns whether the server connection is established or not
//
// Deprecated: function name is not go idiomatic, use IsConnected instead
func (wac *Conn) GetConnected() bool {
	return wac.connected
}

//IsLoggedIn returns whether the you are logged in or not
func (wac *Conn) IsLoggedIn() bool {
	return wac.loggedIn
}

// GetLoggedIn returns whether the you are logged in or not
//
// Deprecated: function name is not go idiomatic, use IsLoggedIn instead.
func (wac *Conn) GetLoggedIn() bool {
	return wac.loggedIn
}
