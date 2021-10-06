// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	"strconv"
	"sync/atomic"

	waBinary "go.mau.fi/whatsmeow/binary"
)

func (cli *Client) generateRequestID() string {
	return cli.uniqueID + strconv.FormatUint(atomic.AddUint64(&cli.idCounter, 1), 10)
}

func (cli *Client) waitResponse(reqID string) chan *waBinary.Node {
	ch := make(chan *waBinary.Node, 1)
	cli.responseWaitersLock.Lock()
	cli.responseWaiters[reqID] = ch
	cli.responseWaitersLock.Unlock()
	return ch
}

func (cli *Client) cancelResponse(reqID string, ch chan *waBinary.Node) {
	cli.responseWaitersLock.Lock()
	close(ch)
	delete(cli.responseWaiters, reqID)
	cli.responseWaitersLock.Unlock()
}

func (cli *Client) receiveResponse(data *waBinary.Node) bool {
	id, ok := data.Attrs["id"].(string)
	if !ok || data.Tag != "iq" {
		return false
	}
	cli.responseWaitersLock.Lock()
	waiter, ok := cli.responseWaiters[id]
	if !ok {
		cli.responseWaitersLock.Unlock()
		return false
	}
	delete(cli.responseWaiters, id)
	cli.responseWaitersLock.Unlock()
	waiter <- data
	return true
}

func (cli *Client) sendRequest(req waBinary.Node) (<-chan *waBinary.Node, error) {
	reqID, ok := req.Attrs["id"].(string)
	if !ok {
		reqID = cli.generateRequestID()
		req.Attrs["id"] = reqID
	}
	waiter := cli.waitResponse(reqID)
	err := cli.sendNode(req)
	if err != nil {
		cli.cancelResponse(reqID, waiter)
		return nil, err
	}
	return waiter, nil
}
