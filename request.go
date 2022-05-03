// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"encoding/base64"
	"strconv"
	"sync/atomic"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func (cli *Client) generateRequestID() string {
	return cli.uniqueID + strconv.FormatUint(uint64(atomic.AddUint32(&cli.idCounter, 1)), 10)
}

var xmlStreamEndNode = &waBinary.Node{Tag: "xmlstreamend"}

func isDisconnectNode(node *waBinary.Node) bool {
	return node == xmlStreamEndNode || node.Tag == "stream:error"
}

func (cli *Client) clearResponseWaiters(node *waBinary.Node) {
	cli.responseWaitersLock.Lock()
	for _, waiter := range cli.responseWaiters {
		select {
		case waiter <- node:
		default:
			close(waiter)
		}
	}
	cli.responseWaiters = make(map[string]chan<- *waBinary.Node)
	cli.responseWaitersLock.Unlock()
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
	if !ok || (data.Tag != "iq" && data.Tag != "ack") {
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

type infoQueryType string

const (
	iqSet infoQueryType = "set"
	iqGet infoQueryType = "get"
)

type infoQuery struct {
	Namespace string
	Type      infoQueryType
	To        types.JID
	Target    types.JID
	ID        string
	Content   interface{}

	Timeout time.Duration
	Context context.Context
}

func (cli *Client) sendIQAsyncDebug(query infoQuery) (<-chan *waBinary.Node, []byte, error) {
	if len(query.ID) == 0 {
		query.ID = cli.generateRequestID()
	}
	waiter := cli.waitResponse(query.ID)
	attrs := waBinary.Attrs{
		"id":    query.ID,
		"xmlns": query.Namespace,
		"type":  string(query.Type),
	}
	if !query.To.IsEmpty() {
		attrs["to"] = query.To
	}
	if !query.Target.IsEmpty() {
		attrs["target"] = query.Target
	}
	data, err := cli.sendNodeDebug(waBinary.Node{
		Tag:     "iq",
		Attrs:   attrs,
		Content: query.Content,
	})
	if err != nil {
		cli.cancelResponse(query.ID, waiter)
		return nil, data, err
	}
	return waiter, data, nil
}

func (cli *Client) sendIQAsync(query infoQuery) (<-chan *waBinary.Node, error) {
	ch, _, err := cli.sendIQAsyncDebug(query)
	return ch, err
}

func (cli *Client) sendIQ(query infoQuery) (*waBinary.Node, error) {
	resChan, data, err := cli.sendIQAsyncDebug(query)
	if err != nil {
		return nil, err
	}
	if query.Timeout == 0 {
		query.Timeout = 75 * time.Second
	}
	if query.Context == nil {
		query.Context = context.Background()
	}
	select {
	case res := <-resChan:
		if isDisconnectNode(res) {
			if cli.DebugDecodeBeforeSend && res.Tag == "stream:error" && res.GetChildByTag("xml-not-well-formed").Tag != "" {
				cli.Log.Debugf("Info query that was interrupted by xml-not-well-formed: %s", base64.URLEncoding.EncodeToString(data))
			}
			return nil, &DisconnectedError{Action: "info query", Node: res}
		}
		resType, _ := res.Attrs["type"].(string)
		if res.Tag != "iq" || (resType != "result" && resType != "error") {
			return res, &IQError{RawNode: res}
		} else if resType == "error" {
			return res, parseIQError(res)
		}
		return res, nil
	case <-query.Context.Done():
		return nil, query.Context.Err()
	case <-time.After(query.Timeout):
		return nil, ErrIQTimedOut
	}
}
