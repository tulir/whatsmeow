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

	waBinary "github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/multidevice/socket"
)

type Client struct {
	Session Session
	Log     log.Logger
	socket  *socket.NoiseSocket

	responseWaiters     map[string]chan<- *waBinary.Node
	responseWaitersLock sync.Mutex

	uniqueID  string
	idCounter uint64
}

func NewClient(log log.Logger) *Client {
	randomBytes := make([]byte, 2)
	_, _ = rand.Read(randomBytes)
	return &Client{
		Log:             log,
		uniqueID:        fmt.Sprintf("%d.%d-", randomBytes[0], randomBytes[1]),
		responseWaiters: make(map[string]chan<- *waBinary.Node),
	}
}

func (cli *Client) Connect() error {
	fs := socket.NewFrameSocket(cli.Log.Sub("Socket"), socket.WAConnHeader)
	if ephemeralKP, err := NewKeyPair(); err != nil {
		return fmt.Errorf("failed to generate ephemeral keypair: %w", err)
	} else if err = fs.Connect(); err != nil {
		fs.Close()
		return err
	} else if err = cli.doHandshake(fs, *ephemeralKP); err != nil {
		fs.Close()
		return fmt.Errorf("noise handshake failed: %w", err)
	}
	cli.socket.OnFrame = cli.handleFrame
	go cli.keepAliveLoop(cli.socket.Context())
	return nil
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
	cli.Log.Debugln("<--", node.XMLString())
	cli.receiveResponse(node)
}

func (cli *Client) sendNode(node waBinary.Node) error {
	payload, err := waBinary.Marshal(node, true)
	if err != nil {
		return fmt.Errorf("failed to marshal ping IQ: %w", err)
	}

	cli.Log.Debugln("-->", node.XMLString())
	return cli.socket.SendFrame(payload)
}
