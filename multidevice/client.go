// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	log "maunium.net/go/maulogger/v2"

	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/multidevice/keys"
	"go.mau.fi/whatsmeow/multidevice/session"
	"go.mau.fi/whatsmeow/multidevice/socket"
)

type Client struct {
	Session *session.Session
	Log     log.Logger
	socket  *socket.NoiseSocket

	mediaConn     *whatsapp.MediaConn
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

func NewClient(sess *session.Session, log log.Logger) *Client {
	randomBytes := make([]byte, 2)
	_, _ = rand.Read(randomBytes)
	cli := &Client{
		Session:         sess,
		Log:             log,
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
		cli.handleDevicesNotification,
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

const streamEnd = "\xf8\x01\x02"

func (cli *Client) handleFrame(data []byte) {
	decompressed, err := waBinary.Unpack(data)
	if err != nil {
		cli.Log.Warnln("Failed to decompress frame:", err)
		cli.Log.Debugln("Errored frame hex:", hex.EncodeToString(data))
		return
	}
	if len(decompressed) == len(streamEnd) && string(decompressed) == streamEnd {
		cli.Log.Warnln("Received stream end frame")
		return
	}
	node, err := waBinary.Unmarshal(decompressed, true)
	if err != nil {
		cli.Log.Warnln("Failed to decode node in frame:", err)
		cli.Log.Debugln("Errored frame hex:", hex.EncodeToString(decompressed))
		return
	}
	cli.Log.Debugln("RECEIVED:", node.XMLString())
	switch {
	case cli.receiveResponse(node):
	case cli.dispatchNode(node):
	default:
		cli.Log.Debugln("Didn't handle WhatsApp node")
	}
}

func (cli *Client) sendNode(node waBinary.Node) error {
	payload, err := waBinary.Marshal(node, true)
	if err != nil {
		return fmt.Errorf("failed to marshal ping IQ: %w", err)
	}

	cli.Log.Debugln("SENDING:", node.XMLString())
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
