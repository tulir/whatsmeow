// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	"context"
	"math/rand"
	"time"

	waBinary "github.com/Rhymen/go-whatsapp/binary"
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
			respCh, err := cli.sendRequest(waBinary.Node{
				Tag: "iq",
				Attrs: map[string]interface{}{
					"to":    waBinary.ServerJID,
					"type":  "get",
					"xmlns": "w:p",
					"id":    cli.generateRequestID(),
				},
				Content: []waBinary.Node{{Tag: "ping"}},
			})
			if err != nil {
				cli.Log.Warnln("Failed to send keepalive:", err)
				continue
			}
			select {
			case <-respCh:
				// All good
			case <-time.After(KeepAliveResponseDeadlineMS * time.Millisecond):
				cli.Log.Warnln("Keepalive timed out")
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
