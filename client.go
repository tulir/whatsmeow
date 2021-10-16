// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsapp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/keys"
	waLog "go.mau.fi/whatsmeow/log"
	"go.mau.fi/whatsmeow/socket"
	"go.mau.fi/whatsmeow/store"
)

type Client struct {
	Store   *store.Device
	Log     waLog.Logger
	recvLog waLog.Logger
	sendLog waLog.Logger
	socket  *socket.NoiseSocket

	mediaConn     *MediaConn
	mediaConnLock sync.Mutex

	responseWaiters     map[string]chan<- *waBinary.Node
	responseWaitersLock sync.Mutex

	messageRetries     map[string]int
	messageRetriesLock sync.Mutex

	nodeHandlers  []nodeHandler
	eventHandlers []func(interface{})

	uniqueID  string
	idCounter uint64
}

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
	}
	cli.nodeHandlers = []nodeHandler{
		cli.handlePairDevice,
		cli.handlePairSuccess,
		cli.handleConnectSuccess,
		cli.handleStreamError,
		cli.handleEncryptedMessage,
		cli.handleReceipt,
		cli.handleNotification,
	}
	return cli
}

func (cli *Client) Connect() error {
	fs := socket.NewFrameSocket(cli.Log.Sub("Socket"), socket.WAConnHeader)
	if err := fs.Connect(); err != nil {
		fs.Close()
		return err
	} else if err = cli.doHandshake(fs, *keys.NewKeyPair()); err != nil {
		fs.Close()
		return fmt.Errorf("noise handshake failed: %w", err)
	}
	cli.socket.OnFrame = cli.handleFrame
	go cli.keepAliveLoop(cli.socket.Context())
	return nil
}

func (cli *Client) Disconnect() {
	if cli.socket != nil {
		cli.socket.Close()
		cli.socket = nil
	}
}

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
		return
	}
	switch {
	case cli.receiveResponse(node):
	case cli.dispatchNode(node):
	default:
		cli.Log.Debugf("Didn't handle WhatsApp node")
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

func (cli *Client) dispatchNode(node *waBinary.Node) bool {
	for _, handler := range cli.nodeHandlers {
		if handler(node) {
			return true
		}
	}
	return false
}

func (cli *Client) dispatchEvent(evt interface{}) {
	for _, handler := range cli.eventHandlers {
		handler(evt)
	}
}
