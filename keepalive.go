// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsapp

import (
	"context"
	"math/rand"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
)

const (
	KeepAliveResponseDeadlineMS = 10_000
	KeepAliveIntervalMinMS      = 20_000
	KeepAliveIntervalMaxMS      = 30_000
)

func (cli *Client) keepAliveLoop(ctx context.Context) {
	for {
		interval := rand.Intn(KeepAliveIntervalMaxMS-KeepAliveIntervalMinMS) + KeepAliveIntervalMinMS
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
	respCh, err := cli.sendIQAsync(InfoQuery{
		Namespace: "w:p",
		Type:      "get",
		To:        waBinary.ServerJID,
		Content:   []waBinary.Node{{Tag: "ping"}},
	})
	if err != nil {
		cli.Log.Warnln("Failed to send keepalive:", err)
		return true
	}
	select {
	case <-respCh:
		// All good
	case <-time.After(KeepAliveResponseDeadlineMS * time.Millisecond):
		cli.Log.Warnln("Keepalive timed out")
	case <-ctx.Done():
		return false
	}
	return true
}
