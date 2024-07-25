// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"fmt"
	"strconv"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func (cli *Client) generateRequestID() string {
	return cli.uniqueID + strconv.FormatUint(cli.idCounter.Add(1), 10)
}

var xmlStreamEndNode = &waBinary.Node{Tag: "xmlstreamend"}

func isDisconnectNode(node *waBinary.Node) bool {
	return node == xmlStreamEndNode || node.Tag == "stream:error"
}

// isAuthErrorDisconnect checks if the given disconnect node is an error that shouldn't cause retrying.
func isAuthErrorDisconnect(node *waBinary.Node) bool {
	if node.Tag != "stream:error" {
		return false
	}
	code, _ := node.Attrs["code"].(string)
	conflict, _ := node.GetOptionalChildByTag("conflict")
	conflictType := conflict.AttrGetter().OptionalString("type")
	if code == "401" || conflictType == "replaced" || conflictType == "device_removed" {
		return true
	}
	return false
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
	NoRetry bool
	Context context.Context
}

func (cli *Client) sendIQAsyncAndGetData(query *infoQuery) (<-chan *waBinary.Node, []byte, error) {
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
	data, err := cli.sendNodeAndGetData(waBinary.Node{
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
	ch, _, err := cli.sendIQAsyncAndGetData(&query)
	return ch, err
}

const defaultRequestTimeout = 75 * time.Second

func (cli *Client) sendIQ(query infoQuery) (*waBinary.Node, error) {
	resChan, data, err := cli.sendIQAsyncAndGetData(&query)
	if err != nil {
		return nil, err
	}
	if query.Timeout == 0 {
		query.Timeout = defaultRequestTimeout
	}
	if query.Context == nil {
		query.Context = context.Background()
	}
	select {
	case res := <-resChan:
		if isDisconnectNode(res) {
			if query.NoRetry {
				return nil, &DisconnectedError{Action: "info query", Node: res}
			}
			res, err = cli.retryFrame("info query", query.ID, data, res, query.Context, query.Timeout)
			if err != nil {
				return nil, err
			}
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

func (cli *Client) retryFrame(reqType, id string, data []byte, origResp *waBinary.Node, ctx context.Context, timeout time.Duration) (*waBinary.Node, error) {
	if isAuthErrorDisconnect(origResp) {
		cli.Log.Debugf("%s (%s) was interrupted by websocket disconnection (%s), not retrying as it looks like an auth error", id, reqType, origResp.XMLString())
		return nil, &DisconnectedError{Action: reqType, Node: origResp}
	}

	cli.Log.Debugf("%s (%s) was interrupted by websocket disconnection (%s), waiting for reconnect to retry...", id, reqType, origResp.XMLString())
	if !cli.WaitForConnection(5 * time.Second) {
		cli.Log.Debugf("Websocket didn't reconnect within 5 seconds of failed %s (%s)", reqType, id)
		return nil, &DisconnectedError{Action: reqType, Node: origResp}
	}

	cli.socketLock.RLock()
	sock := cli.socket
	cli.socketLock.RUnlock()
	if sock == nil {
		return nil, ErrNotConnected
	}

	respChan := cli.waitResponse(id)
	err := sock.SendFrame(data)
	if err != nil {
		cli.cancelResponse(id, respChan)
		return nil, err
	}
	var resp *waBinary.Node
	timeoutChan := make(<-chan time.Time, 1)
	if timeout > 0 {
		timeoutChan = time.After(timeout)
	}
	select {
	case resp = <-respChan:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timeoutChan:
		// FIXME this error isn't technically correct (but works for now - the timeout param is only used from sendIQ)
		return nil, ErrIQTimedOut
	}
	if isDisconnectNode(resp) {
		cli.Log.Debugf("Retrying %s %s was interrupted by websocket disconnection (%v), not retrying anymore", reqType, id, resp.XMLString())
		return nil, &DisconnectedError{Action: fmt.Sprintf("%s (retry)", reqType), Node: resp}
	}
	return resp, nil
}
