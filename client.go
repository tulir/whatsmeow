// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/socket"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/util/keys"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Client contains everything necessary to connect to and interact with the WhatsApp web API.
type Client struct {
	Store   *store.Device
	Log     waLog.Logger
	recvLog waLog.Logger
	sendLog waLog.Logger

	socket     *socket.NoiseSocket
	socketLock sync.Mutex

	isExpectedDisconnect  bool
	EnableAutoReconnect   bool
	LastSuccessfulConnect time.Time
	AutoReconnectErrors   int

	IsLoggedIn bool

	appStateProc *appstate.Processor

	mediaConn     *MediaConn
	mediaConnLock sync.Mutex

	responseWaiters     map[string]chan<- *waBinary.Node
	responseWaitersLock sync.Mutex

	messageRetries     map[string]int
	messageRetriesLock sync.Mutex

	nodeHandlers  map[string]nodeHandler
	handlerQueue  chan *waBinary.Node
	eventHandlers []func(interface{})

	uniqueID  string
	idCounter uint64
}

const handlerQueueSize = 2048

// NewClient initializes a new WhatsApp web client.
//
// The device store must be set. A default SQL-backed implementation is available in the store package.
//
// The logger can be nil, it will default to a no-op logger.
func NewClient(deviceStore *store.Device, log waLog.Logger) *Client {
	if log == nil {
		log = waLog.Noop
	}
	randomBytes := make([]byte, 2)
	_, _ = rand.Read(randomBytes)
	cli := &Client{
		Store:           deviceStore,
		Log:             log,
		recvLog:         log.Sub("Recv"),
		sendLog:         log.Sub("Send"),
		uniqueID:        fmt.Sprintf("%d.%d-", randomBytes[0], randomBytes[1]),
		responseWaiters: make(map[string]chan<- *waBinary.Node),
		eventHandlers:   make([]func(interface{}), 0),
		messageRetries:  make(map[string]int),
		handlerQueue:    make(chan *waBinary.Node, handlerQueueSize),
		appStateProc:    appstate.NewProcessor(deviceStore, log.Sub("AppState")),
	}
	cli.nodeHandlers = map[string]nodeHandler{
		"message":      cli.handleEncryptedMessage,
		"receipt":      cli.handleReceipt,
		"notification": cli.handleNotification,
		"success":      cli.handleConnectSuccess,
		"failure":      cli.handleConnectFailure,
		"stream:error": cli.handleStreamError,
		"iq":           cli.handleIQ,
	}
	return cli
}

// Connect connects the client to the WhatsApp web websocket. After connection, it will either
// authenticate if there's data in the device store, or emit a QREvent to set up a new link.
func (cli *Client) Connect() error {
	cli.socketLock.Lock()
	defer cli.socketLock.Unlock()
	if cli.socket != nil {
		if !cli.socket.IsConnected() {
			cli.disconnect()
		} else {
			return ErrAlreadyConnected
		}
	}

	fs := socket.NewFrameSocket(cli.Log.Sub("Socket"), socket.WAConnHeader)
	if err := fs.Connect(); err != nil {
		fs.Close(0)
		return err
	} else if err = cli.doHandshake(fs, *keys.NewKeyPair()); err != nil {
		fs.Close(0)
		return fmt.Errorf("noise handshake failed: %w", err)
	}
	cli.socket.OnFrame = cli.handleFrame
	cli.socket.SetOnDisconnect(func(ns *socket.NoiseSocket) {
		ns.OnFrame = nil
		ns.SetOnDisconnect(nil)
		cli.socketLock.Lock()
		defer cli.socketLock.Unlock()
		if cli.socket == ns {
			cli.socket = nil
			if !cli.isExpectedDisconnect {
				cli.Log.Debugf("Emitting Disconnected event")
				go cli.dispatchEvent(&events.Disconnected{})
				go cli.autoReconnect()
			} else {
				cli.Log.Debugf("OnDisconnect() called, but it was expected, so not emitting event")
			}
		} else {
			cli.Log.Debugf("Ignoring OnDisconnect on different socket")
		}
	})
	go cli.keepAliveLoop(cli.socket.Context())
	go cli.handlerQueueLoop(cli.socket.Context())
	return nil
}

func (cli *Client) autoReconnect() {
	if !cli.EnableAutoReconnect {
		return
	}
	for {
		cli.AutoReconnectErrors++
		autoReconnectDelay := time.Duration(cli.AutoReconnectErrors) * 2 * time.Second
		cli.Log.Debugf("Automatically reconnecting after %v", autoReconnectDelay)
		time.Sleep(autoReconnectDelay)
		err := cli.Connect()
		if errors.Is(err, ErrAlreadyConnected) {
			cli.Log.Debugf("Connect() said we're already connected after autoreconnect sleep")
			return
		} else if err != nil {
			cli.Log.Errorf("Error reconnecting after autoreconnect sleep: %v", err)
		} else {
			return
		}
	}
}

func (cli *Client) IsConnected() bool {
	return cli.socket != nil && cli.socket.IsConnected()
}

func (cli *Client) Disconnect() {
	if cli.socket == nil {
		return
	}
	cli.socketLock.Lock()
	cli.disconnect()
	cli.socketLock.Unlock()
}

// Disconnect closes the websocket connection.
func (cli *Client) disconnect() {
	if cli.socket != nil {
		cli.socket.SetOnDisconnect(nil)
		cli.socket.OnFrame = nil
		cli.socket.Close(websocket.CloseNormalClosure)
		cli.socket = nil
	}
}

// AddEventHandler registers a new function to receive all events emitted by this client.
func (cli *Client) AddEventHandler(handler func(interface{})) {
	cli.eventHandlers = append(cli.eventHandlers, handler)
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
		cli.Log.Warnf("Received stream end frame")
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
	} else {
		cli.Log.Debugf("Didn't handle WhatsApp node")
	}
}

func (cli *Client) handlerQueueLoop(ctx context.Context) {
	for {
		select {
		case node := <-cli.handlerQueue:
			cli.nodeHandlers[node.Tag](node)
		case <-ctx.Done():
			return
		}
	}
}
func (cli *Client) sendNode(node waBinary.Node) error {
	payload, err := waBinary.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal ping IQ: %w", err)
	}

	cli.sendLog.Debugf("%s", node.XMLString())
	return cli.socket.SendFrame(payload)
}

func (cli *Client) dispatchEvent(evt interface{}) {
	for _, handler := range cli.eventHandlers {
		handler(evt)
	}
}
