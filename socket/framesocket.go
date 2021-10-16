// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package socket

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	waLog "go.mau.fi/whatsmeow/log"
)

type FrameSocket struct {
	conn   *websocket.Conn
	ctx    context.Context
	cancel func()
	log    waLog.Logger

	OnFrame      func([]byte)
	WriteTimeout time.Duration

	Header []byte

	incomingLength int
	receivedLength int
	incoming       []byte
	partialHeader  []byte
}

func NewFrameSocket(log waLog.Logger, header []byte) *FrameSocket {
	return &FrameSocket{
		conn:   nil,
		log:    log,
		Header: header,
	}
}

func (fs *FrameSocket) Context() context.Context {
	return fs.ctx
}

func (fs *FrameSocket) Close() {
	if fs.conn == nil {
		return
	}

	err := fs.conn.Close()
	if err != nil {
		fs.log.Errorf("Error closing websocket: %v", err)
	}
	if fs.cancel != nil {
		fs.cancel()
	}
	fs.conn = nil
	fs.ctx = nil
	fs.cancel = nil
}

func (fs *FrameSocket) Connect() error {
	if fs.conn != nil {
		return ErrSocketAlreadyOpen
	}
	ctx, cancel := context.WithCancel(context.Background())
	fs.ctx, fs.cancel = ctx, cancel
	dialer := websocket.Dialer{}

	headers := http.Header{"Origin": []string{Origin}}
	fs.log.Debugf("Dialing %s", URL)
	var err error
	fs.conn, _, err = dialer.Dial(URL, headers)
	if err != nil {
		fs.cancel()
		return fmt.Errorf("couldn't dial whatsapp web websocket: %w", err)
	}

	fs.conn.SetCloseHandler(func(code int, text string) error {
		fs.log.Debugf("Close handler called with %d/%s", code, text)
		cancel()
		// from default CloseHandler
		message := websocket.FormatCloseMessage(code, "")
		_ = fs.conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))
		return nil
	})

	go fs.readPump(ctx)
	return nil
}

func (fs *FrameSocket) SendFrame(data []byte) error {
	if fs.conn == nil {
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

	if fs.WriteTimeout > 0 {
		err := fs.conn.SetWriteDeadline(time.Now().Add(fs.WriteTimeout))
		if err != nil {
			fs.log.Warnf("Failed to set write deadline: %v", err)
		}
	}
	return fs.conn.WriteMessage(websocket.BinaryMessage, wholeFrame)
}

func (fs *FrameSocket) SetOnFrame(onFrame func([]byte)) {
	fs.OnFrame = onFrame
}

func (fs *FrameSocket) GetOnFrame() func([]byte) {
	return fs.OnFrame
}

func (fs *FrameSocket) ConsumeNextFrame() (output <-chan []byte, cancel func()) {
	return ConsumeNextFrame(fs)
}

func (fs *FrameSocket) SendAndReceiveFrame(ctx context.Context, data []byte) ([]byte, error) {
	output, cancel := fs.ConsumeNextFrame()
	defer cancel()
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
		fs.log.Warnf("No handler defined, dropping frame")
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
				fs.log.Warnf("Received partial header (report if this happens often)")
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

	fs.log.Debugf("Frame websocket read pump starting %p", fs)
	defer fs.log.Debugf("Frame websocket read pump exiting %p", fs)
	for {
		readerFound := make(chan struct{})
		go func() {
			msgType, reader, readErr = fs.conn.NextReader()
			close(readerFound)
		}()
		select {
		case <-readerFound:
			if readErr != nil {
				fs.log.Errorf("Error getting next websocket reader: %v", readErr)
				fs.Close()
				return
			} else if msgType != websocket.BinaryMessage {
				fs.log.Warnf("Got unexpected websocket message type %d", msgType)
				continue
			}
			msg, err := io.ReadAll(reader)
			if err != nil {
				fs.log.Errorf("Error reading message from websocket reader: %v", err)
				continue
			}
			fs.processData(msg)
		case <-ctx.Done():
			return
		}
	}
}
