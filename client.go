// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package whatsmeow implements a client for interacting with the WhatsApp web multidevice API.
package whatsmeow

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.mau.fi/util/random"
	"golang.org/x/net/proxy"

	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/socket"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/util/keys"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// EventHandler is a function that can handle events from WhatsApp.
type EventHandler func(evt interface{})
type nodeHandler func(node *waBinary.Node)

var nextHandlerID uint32

type wrappedEventHandler struct {
	fn EventHandler
	id uint32
}

type deviceCache struct {
	devices []types.JID
	dhash   string
}

// Client contains everything necessary to connect to and interact with the WhatsApp web API.
type Client struct {
	Store   *store.Device
	Log     waLog.Logger
	recvLog waLog.Logger
	sendLog waLog.Logger

	socket     *socket.NoiseSocket
	socketLock sync.RWMutex
	socketWait chan struct{}

	isLoggedIn            atomic.Bool
	expectedDisconnect    atomic.Bool
	EnableAutoReconnect   bool
	LastSuccessfulConnect time.Time
	AutoReconnectErrors   int
	// AutoReconnectHook is called when auto-reconnection fails. If the function returns false,
	// the client will not attempt to reconnect. The number of retries can be read from AutoReconnectErrors.
	AutoReconnectHook func(error) bool

	sendActiveReceipts atomic.Uint32

	// EmitAppStateEventsOnFullSync can be set to true if you want to get app state events emitted
	// even when re-syncing the whole state.
	EmitAppStateEventsOnFullSync bool

	AutomaticMessageRerequestFromPhone bool
	pendingPhoneRerequests             map[types.MessageID]context.CancelFunc
	pendingPhoneRerequestsLock         sync.RWMutex

	appStateProc     *appstate.Processor
	appStateSyncLock sync.Mutex

	historySyncNotifications  chan *waProto.HistorySyncNotification
	historySyncHandlerStarted atomic.Bool

	uploadPreKeysLock sync.Mutex
	lastPreKeyUpload  time.Time

	mediaConnCache *MediaConn
	mediaConnLock  sync.Mutex

	responseWaiters     map[string]chan<- *waBinary.Node
	responseWaitersLock sync.Mutex

	nodeHandlers      map[string]nodeHandler
	handlerQueue      chan *waBinary.Node
	eventHandlers     []wrappedEventHandler
	eventHandlersLock sync.RWMutex

	messageRetries     map[string]int
	messageRetriesLock sync.Mutex

	incomingRetryRequestCounter     map[incomingRetryKey]int
	incomingRetryRequestCounterLock sync.Mutex

	appStateKeyRequests     map[string]time.Time
	appStateKeyRequestsLock sync.RWMutex

	messageSendLock sync.Mutex

	privacySettingsCache atomic.Value

	groupParticipantsCache     map[types.JID][]types.JID
	groupParticipantsCacheLock sync.Mutex
	userDevicesCache           map[types.JID]deviceCache
	userDevicesCacheLock       sync.Mutex

	recentMessagesMap  map[recentMessageKey]RecentMessage
	recentMessagesList [recentMessagesSize]recentMessageKey
	recentMessagesPtr  int
	recentMessagesLock sync.RWMutex

	sessionRecreateHistory     map[types.JID]time.Time
	sessionRecreateHistoryLock sync.Mutex
	// GetMessageForRetry is used to find the source message for handling retry receipts
	// when the message is not found in the recently sent message cache.
	GetMessageForRetry func(requester, to types.JID, id types.MessageID) *waProto.Message
	// PreRetryCallback is called before a retry receipt is accepted.
	// If it returns false, the accepting will be cancelled and the retry receipt will be ignored.
	PreRetryCallback func(receipt *events.Receipt, id types.MessageID, retryCount int, msg *waProto.Message) bool

	// PrePairCallback is called before pairing is completed. If it returns false, the pairing will be cancelled and
	// the client will disconnect.
	PrePairCallback func(jid types.JID, platform, businessName string) bool

	// GetClientPayload is called to get the client payload for connecting to the server.
	// This should NOT be used for WhatsApp (to change the OS name, update fields in store.BaseClientPayload directly).
	GetClientPayload func() *waProto.ClientPayload

	// Should untrusted identity errors be handled automatically? If true, the stored identity and existing signal
	// sessions will be removed on untrusted identity errors, and an events.IdentityChange will be dispatched.
	// If false, decrypting a message from untrusted devices will fail.
	AutoTrustIdentity bool

	// Should sending to own devices be skipped when sending broadcasts?
	// This works around a bug in the WhatsApp android app where it crashes if you send a status message from a linked device.
	DontSendSelfBroadcast bool

	// Should SubscribePresence return an error if no privacy token is stored for the user?
	ErrorOnSubscribePresenceWithoutToken bool

	phoneLinkingCache *phoneLinkingCache

	uniqueID  string
	idCounter atomic.Uint64

	proxy          Proxy
	socksProxy     proxy.Dialer
	proxyOnlyLogin bool
	http           *http.Client

	// This field changes the client to act like a Messenger client instead of a WhatsApp one.
	//
	// Note that you cannot use a Messenger account just by setting this field, you must use a
	// separate library for all the non-e2ee-related stuff like logging in.
	// The library is currently embedded in mautrix-meta (https://github.com/mautrix/meta), but may be separated later.
	MessengerConfig *MessengerConfig
	RefreshCAT      func() error
}

type MessengerConfig struct {
	UserAgent string
	BaseURL   string
}

// Size of buffer for the channel that all incoming XML nodes go through.
// In general it shouldn't go past a few buffered messages, but the channel is big to be safe.
const handlerQueueSize = 2048

// NewClient initializes a new WhatsApp web client.
//
// The logger can be nil, it will default to a no-op logger.
//
// The device store must be set. A default SQL-backed implementation is available in the store/sqlstore package.
//
//	container, err := sqlstore.New("sqlite3", "file:yoursqlitefile.db?_foreign_keys=on", nil)
//	if err != nil {
//		panic(err)
//	}
//	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
//	deviceStore, err := container.GetFirstDevice()
//	if err != nil {
//		panic(err)
//	}
//	client := whatsmeow.NewClient(deviceStore, nil)
func NewClient(deviceStore *store.Device, log waLog.Logger) *Client {
	if log == nil {
		log = waLog.Noop
	}
	uniqueIDPrefix := random.Bytes(2)
	cli := &Client{
		http: &http.Client{
			Transport: (http.DefaultTransport.(*http.Transport)).Clone(),
		},
		proxy:           http.ProxyFromEnvironment,
		Store:           deviceStore,
		Log:             log,
		recvLog:         log.Sub("Recv"),
		sendLog:         log.Sub("Send"),
		uniqueID:        fmt.Sprintf("%d.%d-", uniqueIDPrefix[0], uniqueIDPrefix[1]),
		responseWaiters: make(map[string]chan<- *waBinary.Node),
		eventHandlers:   make([]wrappedEventHandler, 0, 1),
		messageRetries:  make(map[string]int),
		handlerQueue:    make(chan *waBinary.Node, handlerQueueSize),
		appStateProc:    appstate.NewProcessor(deviceStore, log.Sub("AppState")),
		socketWait:      make(chan struct{}),

		incomingRetryRequestCounter: make(map[incomingRetryKey]int),

		historySyncNotifications: make(chan *waProto.HistorySyncNotification, 32),

		groupParticipantsCache: make(map[types.JID][]types.JID),
		userDevicesCache:       make(map[types.JID]deviceCache),

		recentMessagesMap:      make(map[recentMessageKey]RecentMessage, recentMessagesSize),
		sessionRecreateHistory: make(map[types.JID]time.Time),
		GetMessageForRetry:     func(requester, to types.JID, id types.MessageID) *waProto.Message { return nil },
		appStateKeyRequests:    make(map[string]time.Time),

		pendingPhoneRerequests: make(map[types.MessageID]context.CancelFunc),

		EnableAutoReconnect:   true,
		AutoTrustIdentity:     true,
		DontSendSelfBroadcast: true,
	}
	cli.nodeHandlers = map[string]nodeHandler{
		"message":      cli.handleEncryptedMessage,
		"receipt":      cli.handleReceipt,
		"call":         cli.handleCallEvent,
		"chatstate":    cli.handleChatState,
		"presence":     cli.handlePresence,
		"notification": cli.handleNotification,
		"success":      cli.handleConnectSuccess,
		"failure":      cli.handleConnectFailure,
		"stream:error": cli.handleStreamError,
		"iq":           cli.handleIQ,
		"ib":           cli.handleIB,
		// Apparently there's also an <error> node which can have a code=479 and means "Invalid stanza sent (smax-invalid)"
	}
	return cli
}

// SetProxyAddress is a helper method that parses a URL string and calls SetProxy or SetSOCKSProxy based on the URL scheme.
//
// Returns an error if url.Parse fails to parse the given address.
func (cli *Client) SetProxyAddress(addr string, opts ...SetProxyOptions) error {
	if addr == "" {
		cli.SetProxy(nil, opts...)
		return nil
	}
	parsed, err := url.Parse(addr)
	if err != nil {
		return err
	}
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		cli.SetProxy(http.ProxyURL(parsed), opts...)
	} else if parsed.Scheme == "socks5" {
		px, err := proxy.FromURL(parsed, proxy.Direct)
		if err != nil {
			return err
		}
		cli.SetSOCKSProxy(px, opts...)
	} else {
		return fmt.Errorf("unsupported proxy scheme %q", parsed.Scheme)
	}
	return nil
}

type Proxy = func(*http.Request) (*url.URL, error)

// SetProxy sets a HTTP proxy to use for WhatsApp web websocket connections and media uploads/downloads.
//
// Must be called before Connect() to take effect in the websocket connection.
// If you want to change the proxy after connecting, you must call Disconnect() and then Connect() again manually.
//
// By default, the client will find the proxy from the https_proxy environment variable like Go's net/http does.
//
// To disable reading proxy info from environment variables, explicitly set the proxy to nil:
//
//	cli.SetProxy(nil)
//
// To use a different proxy for the websocket and media, pass a function that checks the request path or headers:
//
//	cli.SetProxy(func(r *http.Request) (*url.URL, error) {
//		if r.URL.Host == "web.whatsapp.com" && r.URL.Path == "/ws/chat" {
//			return websocketProxyURL, nil
//		} else {
//			return mediaProxyURL, nil
//		}
//	})
func (cli *Client) SetProxy(proxy Proxy, opts ...SetProxyOptions) {
	var opt SetProxyOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	if !opt.NoWebsocket {
		cli.proxy = proxy
		cli.socksProxy = nil
	}
	if !opt.NoMedia {
		transport := cli.http.Transport.(*http.Transport)
		transport.Proxy = proxy
		transport.Dial = nil
		transport.DialContext = nil
	}
}

type SetProxyOptions struct {
	// If NoWebsocket is true, the proxy won't be used for the websocket
	NoWebsocket bool
	// If NoMedia is true, the proxy won't be used for media uploads/downloads
	NoMedia bool
}

// SetSOCKSProxy sets a SOCKS5 proxy to use for WhatsApp web websocket connections and media uploads/downloads.
//
// Same details as SetProxy apply, but using a different proxy for the websocket and media is not currently supported.
func (cli *Client) SetSOCKSProxy(px proxy.Dialer, opts ...SetProxyOptions) {
	var opt SetProxyOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	if !opt.NoWebsocket {
		cli.socksProxy = px
		cli.proxy = nil
	}
	if !opt.NoMedia {
		transport := cli.http.Transport.(*http.Transport)
		transport.Proxy = nil
		transport.Dial = cli.socksProxy.Dial
		contextDialer, ok := cli.socksProxy.(proxy.ContextDialer)
		if ok {
			transport.DialContext = contextDialer.DialContext
		} else {
			transport.DialContext = nil
		}
	}
}

// ToggleProxyOnlyForLogin changes whether the proxy set with SetProxy or related methods
// is only used for the pre-login websocket and not authenticated websockets.
func (cli *Client) ToggleProxyOnlyForLogin(only bool) {
	cli.proxyOnlyLogin = only
}

func (cli *Client) getSocketWaitChan() <-chan struct{} {
	cli.socketLock.RLock()
	ch := cli.socketWait
	cli.socketLock.RUnlock()
	return ch
}

func (cli *Client) closeSocketWaitChan() {
	cli.socketLock.Lock()
	close(cli.socketWait)
	cli.socketWait = make(chan struct{})
	cli.socketLock.Unlock()
}

func (cli *Client) getOwnID() types.JID {
	id := cli.Store.ID
	if id == nil {
		return types.EmptyJID
	}
	return *id
}

func (cli *Client) WaitForConnection(timeout time.Duration) bool {
	timeoutChan := time.After(timeout)
	cli.socketLock.RLock()
	for cli.socket == nil || !cli.socket.IsConnected() || !cli.IsLoggedIn() {
		ch := cli.socketWait
		cli.socketLock.RUnlock()
		select {
		case <-ch:
		case <-timeoutChan:
			return false
		}
		cli.socketLock.RLock()
	}
	cli.socketLock.RUnlock()
	return true
}

// Connect connects the client to the WhatsApp web websocket. After connection, it will either
// authenticate if there's data in the device store, or emit a QREvent to set up a new link.
func (cli *Client) Connect() error {
	cli.socketLock.Lock()
	defer cli.socketLock.Unlock()
	if cli.socket != nil {
		if !cli.socket.IsConnected() {
			cli.unlockedDisconnect()
		} else {
			return ErrAlreadyConnected
		}
	}

	cli.resetExpectedDisconnect()
	wsDialer := websocket.Dialer{}
	if !cli.proxyOnlyLogin || cli.Store.ID == nil {
		if cli.proxy != nil {
			wsDialer.Proxy = cli.proxy
		} else if cli.socksProxy != nil {
			wsDialer.NetDial = cli.socksProxy.Dial
			contextDialer, ok := cli.socksProxy.(proxy.ContextDialer)
			if ok {
				wsDialer.NetDialContext = contextDialer.DialContext
			}
		}
	}
	fs := socket.NewFrameSocket(cli.Log.Sub("Socket"), wsDialer)
	if cli.MessengerConfig != nil {
		fs.URL = "wss://web-chat-e2ee.facebook.com/ws/chat"
		fs.HTTPHeaders.Set("Origin", cli.MessengerConfig.BaseURL)
		fs.HTTPHeaders.Set("User-Agent", cli.MessengerConfig.UserAgent)
		fs.HTTPHeaders.Set("Sec-Fetch-Dest", "empty")
		fs.HTTPHeaders.Set("Sec-Fetch-Mode", "websocket")
		fs.HTTPHeaders.Set("Sec-Fetch-Site", "cross-site")
	}
	if err := fs.Connect(); err != nil {
		fs.Close(0)
		return err
	} else if err = cli.doHandshake(fs, *keys.NewKeyPair()); err != nil {
		fs.Close(0)
		return fmt.Errorf("noise handshake failed: %w", err)
	}
	go cli.keepAliveLoop(cli.socket.Context())
	go cli.handlerQueueLoop(cli.socket.Context())
	return nil
}

// IsLoggedIn returns true after the client is successfully connected and authenticated on WhatsApp.
func (cli *Client) IsLoggedIn() bool {
	return cli.isLoggedIn.Load()
}

func (cli *Client) onDisconnect(ns *socket.NoiseSocket, remote bool) {
	ns.Stop(false)
	cli.socketLock.Lock()
	defer cli.socketLock.Unlock()
	if cli.socket == ns {
		cli.socket = nil
		cli.clearResponseWaiters(xmlStreamEndNode)
		if !cli.isExpectedDisconnect() && remote {
			cli.Log.Debugf("Emitting Disconnected event")
			go cli.dispatchEvent(&events.Disconnected{})
			go cli.autoReconnect()
		} else if remote {
			cli.Log.Debugf("OnDisconnect() called, but it was expected, so not emitting event")
		} else {
			cli.Log.Debugf("OnDisconnect() called after manual disconnection")
		}
	} else {
		cli.Log.Debugf("Ignoring OnDisconnect on different socket")
	}
}

func (cli *Client) expectDisconnect() {
	cli.expectedDisconnect.Store(true)
}

func (cli *Client) resetExpectedDisconnect() {
	cli.expectedDisconnect.Store(false)
}

func (cli *Client) isExpectedDisconnect() bool {
	return cli.expectedDisconnect.Load()
}

func (cli *Client) autoReconnect() {
	if !cli.EnableAutoReconnect || cli.Store.ID == nil {
		return
	}
	for {
		autoReconnectDelay := time.Duration(cli.AutoReconnectErrors) * 2 * time.Second
		cli.Log.Debugf("Automatically reconnecting after %v", autoReconnectDelay)
		cli.AutoReconnectErrors++
		time.Sleep(autoReconnectDelay)
		err := cli.Connect()
		if errors.Is(err, ErrAlreadyConnected) {
			cli.Log.Debugf("Connect() said we're already connected after autoreconnect sleep")
			return
		} else if err != nil {
			cli.Log.Errorf("Error reconnecting after autoreconnect sleep: %v", err)
			if cli.AutoReconnectHook != nil && !cli.AutoReconnectHook(err) {
				cli.Log.Debugf("AutoReconnectHook returned false, not reconnecting")
				return
			}
		} else {
			return
		}
	}
}

// IsConnected checks if the client is connected to the WhatsApp web websocket.
// Note that this doesn't check if the client is authenticated. See the IsLoggedIn field for that.
func (cli *Client) IsConnected() bool {
	cli.socketLock.RLock()
	connected := cli.socket != nil && cli.socket.IsConnected()
	cli.socketLock.RUnlock()
	return connected
}

// Disconnect disconnects from the WhatsApp web websocket.
//
// This will not emit any events, the Disconnected event is only used when the
// connection is closed by the server or a network error.
func (cli *Client) Disconnect() {
	if cli.socket == nil {
		return
	}
	cli.socketLock.Lock()
	cli.unlockedDisconnect()
	cli.socketLock.Unlock()
}

// Disconnect closes the websocket connection.
func (cli *Client) unlockedDisconnect() {
	if cli.socket != nil {
		cli.socket.Stop(true)
		cli.socket = nil
		cli.clearResponseWaiters(xmlStreamEndNode)
	}
}

// Logout sends a request to unlink the device, then disconnects from the websocket and deletes the local device store.
//
// If the logout request fails, the disconnection and local data deletion will not happen either.
// If an error is returned, but you want to force disconnect/clear data, call Client.Disconnect() and Client.Store.Delete() manually.
//
// Note that this will not emit any events. The LoggedOut event is only used for external logouts
// (triggered by the user from the main device or by WhatsApp servers).
func (cli *Client) Logout() error {
	if cli.MessengerConfig != nil {
		return errors.New("can't logout with Messenger credentials")
	}
	ownID := cli.getOwnID()
	if ownID.IsEmpty() {
		return ErrNotLoggedIn
	}
	_, err := cli.sendIQ(infoQuery{
		Namespace: "md",
		Type:      "set",
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "remove-companion-device",
			Attrs: waBinary.Attrs{
				"jid":    ownID,
				"reason": "user_initiated",
			},
		}},
	})
	if err != nil {
		return fmt.Errorf("error sending logout request: %w", err)
	}
	cli.Disconnect()
	err = cli.Store.Delete()
	if err != nil {
		return fmt.Errorf("error deleting data from store: %w", err)
	}
	return nil
}

// AddEventHandler registers a new function to receive all events emitted by this client.
//
// The returned integer is the event handler ID, which can be passed to RemoveEventHandler to remove it.
//
// All registered event handlers will receive all events. You should use a type switch statement to
// filter the events you want:
//
//	func myEventHandler(evt interface{}) {
//		switch v := evt.(type) {
//		case *events.Message:
//			fmt.Println("Received a message!")
//		case *events.Receipt:
//			fmt.Println("Received a receipt!")
//		}
//	}
//
// If you want to access the Client instance inside the event handler, the recommended way is to
// wrap the whole handler in another struct:
//
//	type MyClient struct {
//		WAClient *whatsmeow.Client
//		eventHandlerID uint32
//	}
//
//	func (mycli *MyClient) register() {
//		mycli.eventHandlerID = mycli.WAClient.AddEventHandler(mycli.myEventHandler)
//	}
//
//	func (mycli *MyClient) myEventHandler(evt interface{}) {
//		// Handle event and access mycli.WAClient
//	}
func (cli *Client) AddEventHandler(handler EventHandler) uint32 {
	nextID := atomic.AddUint32(&nextHandlerID, 1)
	cli.eventHandlersLock.Lock()
	cli.eventHandlers = append(cli.eventHandlers, wrappedEventHandler{handler, nextID})
	cli.eventHandlersLock.Unlock()
	return nextID
}

// RemoveEventHandler removes a previously registered event handler function.
// If the function with the given ID is found, this returns true.
//
// N.B. Do not run this directly from an event handler. That would cause a deadlock because the
// event dispatcher holds a read lock on the event handler list, and this method wants a write lock
// on the same list. Instead run it in a goroutine:
//
//	func (mycli *MyClient) myEventHandler(evt interface{}) {
//		if noLongerWantEvents {
//			go mycli.WAClient.RemoveEventHandler(mycli.eventHandlerID)
//		}
//	}
func (cli *Client) RemoveEventHandler(id uint32) bool {
	cli.eventHandlersLock.Lock()
	defer cli.eventHandlersLock.Unlock()
	for index := range cli.eventHandlers {
		if cli.eventHandlers[index].id == id {
			if index == 0 {
				cli.eventHandlers[0].fn = nil
				cli.eventHandlers = cli.eventHandlers[1:]
				return true
			} else if index < len(cli.eventHandlers)-1 {
				copy(cli.eventHandlers[index:], cli.eventHandlers[index+1:])
			}
			cli.eventHandlers[len(cli.eventHandlers)-1].fn = nil
			cli.eventHandlers = cli.eventHandlers[:len(cli.eventHandlers)-1]
			return true
		}
	}
	return false
}

// RemoveEventHandlers removes all event handlers that have been registered with AddEventHandler
func (cli *Client) RemoveEventHandlers() {
	cli.eventHandlersLock.Lock()
	cli.eventHandlers = make([]wrappedEventHandler, 0, 1)
	cli.eventHandlersLock.Unlock()
}

func (cli *Client) handleFrame(data []byte) {
	decompressed, err := waBinary.Unpack(data)
	if err != nil {
		cli.Log.Warnf("Failed to decompress frame: %v", err)
		cli.Log.Debugf("Errored frame hex: %s", hex.EncodeToString(data))
		return
	}
	node, err := waBinary.Unmarshal(decompressed)
	if err != nil {
		cli.Log.Warnf("Failed to decode node in frame: %v", err)
		cli.Log.Debugf("Errored frame hex: %s", hex.EncodeToString(decompressed))
		return
	}
	cli.recvLog.Debugf("%s", node.XMLString())
	if node.Tag == "xmlstreamend" {
		if !cli.isExpectedDisconnect() {
			cli.Log.Warnf("Received stream end frame")
		}
		// TODO should we do something else?
	} else if cli.receiveResponse(node) {
		// handled
	} else if _, ok := cli.nodeHandlers[node.Tag]; ok {
		select {
		case cli.handlerQueue <- node:
		default:
			cli.Log.Warnf("Handler queue is full, message ordering is no longer guaranteed")
			go func() {
				cli.handlerQueue <- node
			}()
		}
	} else if node.Tag != "ack" {
		cli.Log.Debugf("Didn't handle WhatsApp node %s", node.Tag)
	}
}

func stopAndDrainTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func (cli *Client) handlerQueueLoop(ctx context.Context) {
	timer := time.NewTimer(5 * time.Minute)
	stopAndDrainTimer(timer)
	cli.Log.Debugf("Starting handler queue loop")
	for {
		select {
		case node := <-cli.handlerQueue:
			doneChan := make(chan struct{}, 1)
			go func() {
				start := time.Now()
				cli.nodeHandlers[node.Tag](node)
				duration := time.Since(start)
				doneChan <- struct{}{}
				if duration > 5*time.Second {
					cli.Log.Warnf("Node handling took %s for %s", duration, node.XMLString())
				}
			}()
			timer.Reset(5 * time.Minute)
			select {
			case <-doneChan:
				stopAndDrainTimer(timer)
			case <-timer.C:
				cli.Log.Warnf("Node handling is taking long for %s - continuing in background", node.XMLString())
			}
		case <-ctx.Done():
			cli.Log.Debugf("Closing handler queue loop")
			return
		}
	}
}

func (cli *Client) sendNodeAndGetData(node waBinary.Node) ([]byte, error) {
	cli.socketLock.RLock()
	sock := cli.socket
	cli.socketLock.RUnlock()
	if sock == nil {
		return nil, ErrNotConnected
	}

	payload, err := waBinary.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal node: %w", err)
	}

	cli.sendLog.Debugf("%s", node.XMLString())
	return payload, sock.SendFrame(payload)
}

func (cli *Client) sendNode(node waBinary.Node) error {
	_, err := cli.sendNodeAndGetData(node)
	return err
}

func (cli *Client) dispatchEvent(evt interface{}) {
	cli.eventHandlersLock.RLock()
	defer func() {
		cli.eventHandlersLock.RUnlock()
		err := recover()
		if err != nil {
			cli.Log.Errorf("Event handler panicked while handling a %T: %v\n%s", evt, err, debug.Stack())
		}
	}()
	for _, handler := range cli.eventHandlers {
		handler.fn(evt)
	}
}

// ParseWebMessage parses a WebMessageInfo object into *events.Message to match what real-time messages have.
//
// The chat JID can be found in the Conversation data:
//
//	chatJID, err := types.ParseJID(conv.GetId())
//	for _, historyMsg := range conv.GetMessages() {
//		evt, err := cli.ParseWebMessage(chatJID, historyMsg.GetMessage())
//		yourNormalEventHandler(evt)
//	}
func (cli *Client) ParseWebMessage(chatJID types.JID, webMsg *waProto.WebMessageInfo) (*events.Message, error) {
	var err error
	if chatJID.IsEmpty() {
		chatJID, err = types.ParseJID(webMsg.GetKey().GetRemoteJid())
		if err != nil {
			return nil, fmt.Errorf("no chat JID provided and failed to parse remote JID: %w", err)
		}
	}
	info := types.MessageInfo{
		MessageSource: types.MessageSource{
			Chat:     chatJID,
			IsFromMe: webMsg.GetKey().GetFromMe(),
			IsGroup:  chatJID.Server == types.GroupServer,
		},
		ID:        webMsg.GetKey().GetId(),
		PushName:  webMsg.GetPushName(),
		Timestamp: time.Unix(int64(webMsg.GetMessageTimestamp()), 0),
	}
	if info.IsFromMe {
		info.Sender = cli.getOwnID().ToNonAD()
		if info.Sender.IsEmpty() {
			return nil, ErrNotLoggedIn
		}
	} else if chatJID.Server == types.DefaultUserServer || chatJID.Server == types.NewsletterServer {
		info.Sender = chatJID
	} else if webMsg.GetParticipant() != "" {
		info.Sender, err = types.ParseJID(webMsg.GetParticipant())
	} else if webMsg.GetKey().GetParticipant() != "" {
		info.Sender, err = types.ParseJID(webMsg.GetKey().GetParticipant())
	} else {
		return nil, fmt.Errorf("couldn't find sender of message %s", info.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse sender of message %s: %v", info.ID, err)
	}
	evt := &events.Message{
		RawMessage:   webMsg.GetMessage(),
		SourceWebMsg: webMsg,
		Info:         info,
	}
	evt.UnwrapRaw()
	return evt, nil
}
