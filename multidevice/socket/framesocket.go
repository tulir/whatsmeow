// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	log "maunium.net/go/maulogger/v2"
)

const (
	// Origin is the Origin header for all WhatsApp websocket connections
	Origin = "https://web.whatsapp.com"
	// URL is the websocket URL for the new multidevice protocol
	URL = "wss://web.whatsapp.com/ws/chat"
	// LegacyURL is the websocket URL for the legacy phone link protocol
	LegacyURL = "wss://web.whatsapp.com/ws"
)

const WADictVersion = 2
const WAMagicValue = 5

var WAConnHeader = []byte{'W', 'A', WAMagicValue, WADictVersion}

type FrameSocket struct {
	conn   *websocket.Conn
	cancel func()
	log    log.Logger

	OnFrame      func([]byte)
	WriteTimeout time.Duration

	Header []byte

	incomingLength int
	receivedLength int
	incoming       []byte
	partialHeader  []byte
}

func NewFrameSocket(log log.Logger, header []byte) *FrameSocket {
	return &FrameSocket{
		conn:   nil,
		log:    log,
		Header: header,
	}
}

func (fs *FrameSocket) Close() {
	err := fs.conn.Close()
	if err != nil {
		fs.log.Errorln("Error closing websocket:", err)
	}
}

func (fs *FrameSocket) Connect() error {
	var ctx context.Context
	ctx, fs.cancel = context.WithCancel(context.Background())
	dialer := websocket.Dialer{}

	headers := http.Header{"Origin": []string{Origin}}
	fs.log.Debugln("Dialing " + URL)
	var err error
	fs.conn, _, err = dialer.Dial(URL, headers)
	if err != nil {
		fs.cancel()
		return fmt.Errorf("couldn't dial whatsapp web websocket: %w", err)
	}

	fs.conn.SetCloseHandler(func(code int, text string) error {
		fs.log.Debugfln("Close handler called with %d/%s", code, text)
		fs.cancel()
		// from default CloseHandler
		message := websocket.FormatCloseMessage(code, "")
		_ = fs.conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))
		return nil
	})

	go fs.readPump(ctx)
	return nil
}

const FrameMaxSize = 2 << 23
const FrameLengthSize = 3

var ErrFrameTooLarge = errors.New("frame too large")

func (fs *FrameSocket) SendFrame(data []byte) error {
	dataLength := len(data)
	if dataLength >= FrameMaxSize {
		return fmt.Errorf("%w (got %d bytes, max %d bytes)", ErrFrameTooLarge, len(data), FrameMaxSize)
	}

	headerLength := len(fs.Header)
	// Whole frame is header + 3 bytes for length + data
	wholeFrame := make([]byte, headerLength+FrameLengthSize+dataLength)

	// Copy the header if it's there
	if fs.Header != nil {
		copy(wholeFrame[:headerLength], fs.Header)
		// We only want to send the header once
		fs.Header = nil
	}

	// Encode length of frame
	wholeFrame[headerLength] = byte(dataLength >> 16)
	wholeFrame[headerLength+1] = byte(dataLength >> 8)
	wholeFrame[headerLength+2] = byte(dataLength)

	// Copy actual frame data
	copy(wholeFrame[headerLength+FrameLengthSize:], data)

	if fs.WriteTimeout > 0 {
		err := fs.conn.SetWriteDeadline(time.Now().Add(fs.WriteTimeout))
		if err != nil {
			fs.log.Warnln("Failed to set write deadline:", err)
		}
	}
	return fs.conn.WriteMessage(websocket.BinaryMessage, wholeFrame)
}

func (fs *FrameSocket) SendAndReceiveFrame(ctx context.Context, data []byte) ([]byte, error) {
	output := make(chan []byte, 1)
	prevOnFrame := fs.OnFrame
	defer func() {
		fs.OnFrame = prevOnFrame
	}()
	fs.OnFrame = func(bytes []byte) {
		output <- bytes
	}
	err := fs.SendFrame(data)
	if err != nil {
		return nil, err
	}
	select {
	case data = <-output:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (fs *FrameSocket) frameComplete() {
	data := fs.incoming
	fs.incoming = nil
	fs.partialHeader = nil
	fs.incomingLength = 0
	fs.receivedLength = 0
	if fs.OnFrame == nil {
		fs.log.Warnln("No handler defined, dropping frame")
	} else {
		fs.OnFrame(data)
	}
}

func (fs *FrameSocket) processData(msg []byte) {
	for len(msg) > 0 {
		// This probably doesn't happen a lot (if at all), so the code is unoptimized
		if fs.partialHeader != nil {
			msg = append(fs.partialHeader, msg...)
			fs.partialHeader = nil
		}
		if fs.incoming == nil {
			if len(msg) >= FrameLengthSize {
				length := (int(msg[0]) << 16) + (int(msg[1]) << 8) + int(msg[2])
				fs.incomingLength = length
				fs.receivedLength = len(msg)
				msg = msg[FrameLengthSize:]
				if len(msg) >= length {
					fs.incoming = msg[:length]
					msg = msg[length:]
					fs.frameComplete()
				} else {
					fs.incoming = make([]byte, length)
					copy(fs.incoming, msg)
					msg = nil
				}
			} else {
				fs.log.Warnln("Received partial header (report if this happens often)")
				fs.partialHeader = msg
				msg = nil
			}
		} else {
			if len(fs.incoming)+len(msg) >= fs.incomingLength {
				copy(fs.incoming[fs.receivedLength:], msg[:fs.incomingLength-fs.receivedLength])
				msg = msg[fs.incomingLength-fs.receivedLength:]
				fs.frameComplete()
			} else {
				copy(fs.incoming[fs.receivedLength:], msg)
				fs.receivedLength += len(msg)
				msg = nil
			}
		}
	}
}

func (fs *FrameSocket) readPump(ctx context.Context) {
	var readErr error
	var msgType int
	var reader io.Reader

	fs.log.Debugfln("Frame websocket read pump starting %p", fs)
	defer fs.log.Debugfln("Frame websocket read pump exiting %p", fs)
	for {
		readerFound := make(chan struct{})
		go func() {
			msgType, reader, readErr = fs.conn.NextReader()
			close(readerFound)
		}()
		select {
		case <-readerFound:
			if readErr != nil {
				fs.log.Errorln("Error getting next websocket reader:", readErr)
				return
			} else if msgType != websocket.BinaryMessage {
				fs.log.Warnfln("Got unexpected websocket message type %d", msgType)
				continue
			}
			msg, err := io.ReadAll(reader)
			if err != nil {
				fs.log.Errorln("Error reading message from websocket reader:", err)
				continue
			}
			fs.processData(msg)
		case <-ctx.Done():
			return
		}
	}
}
