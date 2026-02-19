// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/coder/websocket"

	waLog "go.mau.fi/whatsmeow/util/log"
)

type FrameSocket struct {
	parentCtx context.Context
	cancelCtx context.Context
	cancel    context.CancelFunc
	conn      *websocket.Conn
	log       waLog.Logger
	lock      sync.Mutex

	URL         string
	HTTPHeaders http.Header
	HTTPClient  *http.Client

	Frames       chan []byte
	OnDisconnect func(ctx context.Context, remote bool)

	Header []byte

	closed bool

	incomingLength int
	receivedLength int
	incoming       []byte
	partialHeader  []byte
}

func NewFrameSocket(log waLog.Logger, client *http.Client) *FrameSocket {
	return &FrameSocket{
		log:    log,
		Header: WAConnHeader,
		Frames: make(chan []byte),

		URL:         URL,
		HTTPHeaders: http.Header{"Origin": {Origin}},
		HTTPClient:  client,
	}
}

func (fs *FrameSocket) IsConnected() bool {
	return fs.conn != nil
}

func (fs *FrameSocket) Close(code websocket.StatusCode) {
	fs.lock.Lock()
	defer fs.lock.Unlock()

	if fs.conn == nil {
		return
	}

	fs.closed = true
	if code > 0 {
		err := fs.conn.Close(code, "")
		if err != nil {
			fs.log.Warnf("Error sending close to websocket: %v", err)
		}
	} else {
		err := fs.conn.CloseNow()
		if err != nil {
			fs.log.Debugf("Error force closing websocket: %v", err)
		}
	}
	fs.conn = nil
	fs.cancel()
	fs.cancel = nil
	if fs.OnDisconnect != nil {
		go fs.OnDisconnect(fs.parentCtx, code == 0)
	}
}

func (fs *FrameSocket) Connect(ctx context.Context) error {
	fs.lock.Lock()
	defer fs.lock.Unlock()
	if fs.conn != nil {
		return ErrSocketAlreadyOpen
	}
	fs.parentCtx = ctx
	fs.cancelCtx, fs.cancel = context.WithCancel(ctx)

	fs.log.Debugf("Dialing %s", fs.URL)
	conn, resp, err := websocket.Dial(ctx, fs.URL, fs.makeDialOptions())
	if err != nil {
		if resp != nil {
			err = ErrWithStatusCode{err, resp.StatusCode}
		}
		fs.cancel()
		return fmt.Errorf("failed to dial whatsapp web websocket: %w", err)
	}
	conn.SetReadLimit(FrameMaxSize)

	fs.conn = conn

	go fs.readPump(conn, ctx)
	return nil
}

func (fs *FrameSocket) Context() context.Context {
	return fs.cancelCtx
}

func (fs *FrameSocket) SendFrame(data []byte) error {
	conn := fs.conn
	if conn == nil {
		return ErrSocketClosed
	}
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

	return conn.Write(fs.cancelCtx, websocket.MessageBinary, wholeFrame)
}

func (fs *FrameSocket) frameComplete() {
	data := fs.incoming
	fs.incoming = nil
	fs.partialHeader = nil
	fs.incomingLength = 0
	fs.receivedLength = 0
	fs.Frames <- data
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
				fs.log.Warnf("Received partial header (report if this happens often)")
				fs.partialHeader = msg
				msg = nil
			}
		} else {
			if fs.receivedLength+len(msg) >= fs.incomingLength {
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

func (fs *FrameSocket) readPump(conn *websocket.Conn, ctx context.Context) {
	fs.log.Debugf("Frame websocket read pump starting %p", fs)
	defer func() {
		fs.log.Debugf("Frame websocket read pump exiting %p", fs)
		go fs.Close(0)
	}()
	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			// Ignore the error if the context has been closed
			if !fs.closed && !errors.Is(ctx.Err(), context.Canceled) {
				fs.log.Errorf("Error reading from websocket: %v", err)
			}
			return
		} else if msgType != websocket.MessageBinary {
			fs.log.Warnf("Got unexpected websocket message type %d", msgType)
			continue
		}
		fs.processData(data)
	}
}
