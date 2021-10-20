// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"math/rand"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

var (
	// KeepAliveResponseDeadline specifies the duration to wait for a response to websocket keepalive pings.
	KeepAliveResponseDeadline = 10 * time.Second
	// KeepAliveIntervalMin specifies the minimum interval for websocket keepalive pings.
	KeepAliveIntervalMin = 20 * time.Second
	// KeepAliveIntervalMax specifies the maximum interval for websocket keepalive pings.
	KeepAliveIntervalMax = 30 * time.Second
)

func (cli *Client) keepAliveLoop(ctx context.Context) {
	for {
		interval := rand.Int63n(KeepAliveIntervalMax.Milliseconds()-KeepAliveIntervalMin.Milliseconds()) + KeepAliveIntervalMin.Milliseconds()
		select {
		case <-time.After(time.Duration(interval) * time.Millisecond):
			if !cli.sendKeepAlive(ctx) {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (cli *Client) sendKeepAlive(ctx context.Context) bool {
	respCh, err := cli.sendIQAsync(infoQuery{
		Namespace: "w:p",
		Type:      "get",
		To:        types.ServerJID,
		Content:   []waBinary.Node{{Tag: "ping"}},
	})
	if err != nil {
		cli.Log.Warnf("Failed to send keepalive: %v", err)
		return true
	}
	select {
	case <-respCh:
		// All good
	case <-time.After(KeepAliveResponseDeadline):
		// TODO disconnect websocket?
		cli.Log.Warnf("Keepalive timed out")
	case <-ctx.Done():
		return false
	}
	return true
}
