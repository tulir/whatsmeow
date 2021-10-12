// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

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

type InfoQuery struct {
	Namespace string
	Type      string
	To        waBinary.FullJID
	ID        string
	Content   interface{}

	Timeout time.Duration
}

func (cli *Client) sendIQAsync(query InfoQuery) (<-chan *waBinary.Node, error) {
	if len(query.ID) == 0 {
		query.ID = cli.generateRequestID()
	}
	waiter := cli.waitResponse(query.ID)
	err := cli.sendNode(waBinary.Node{
		Tag: "iq",
		Attrs: map[string]interface{}{
			"id":    query.ID,
			"xmlns": query.Namespace,
			"type":  query.Type,
			"to":    query.To,
		},
		Content: query.Content,
	})
	if err != nil {
		cli.cancelResponse(query.ID, waiter)
		return nil, err
	}
	return waiter, nil
}

func (cli *Client) sendIQ(query InfoQuery) (*waBinary.Node, error) {
	resChan, err := cli.sendIQAsync(query)
	if err != nil {
		return nil, err
	}
	if query.Timeout == 0 {
		query.Timeout = 1 * time.Minute
	}
	select {
	case res := <-resChan:
		resType, _ := res.Attrs["type"].(string)
		if res.Tag != "iq" || resType != "result" {
			return nil, fmt.Errorf("unexpected response %s %s", res.Tag, resType)
		}
		return res, nil
	case <-time.After(query.Timeout):
		return nil, fmt.Errorf("query timed out")
	}
}
