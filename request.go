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

func (cli *Client) receiveResponse(ctx context.Context, data *waBinary.Node) bool {
	id, ok := data.Attrs["id"].(string)
	if !ok || (data.Tag != "iq" && data.Tag != "ack") {
		return false
	}
	cli.responseWaitersLock.Lock()
	waiter, ok := cli.responseWaiters[id]
	if !ok {
		cli.responseWaitersLock.Unlock()
		// 🔒 FIX: Log quando não encontra waiter (pode ser resposta não esperada ou ID incorreto)
		cli.Log.Infof("receiveResponse: no waiter found for id=%s, tag=%s", id, data.Tag)
		return false
	}
	delete(cli.responseWaiters, id)
	cli.responseWaitersLock.Unlock()

	// 🔒 FIX: Log quando encontra waiter e envia resposta (INFO para garantir que apareça)
	cli.Log.Infof("receiveResponse: found waiter for id=%s, tag=%s, sending response", id, data.Tag)

	select {
	case waiter <- data:
		cli.Log.Infof("receiveResponse: response sent successfully for id=%s", id)
	case <-ctx.Done():
		cli.Log.Warnf("receiveResponse: context cancelled while sending response for id=%s", id)
	}
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
	SMaxID    string
	Content   interface{}

	Timeout time.Duration
	NoRetry bool
}

func (cli *Client) sendIQAsyncAndGetData(ctx context.Context, query *infoQuery) (<-chan *waBinary.Node, []byte, error) {
	if cli == nil {
		return nil, nil, ErrClientIsNil
	}
	if len(query.ID) == 0 {
		query.ID = cli.generateRequestID()
	}
	waiter := cli.waitResponse(query.ID)
	attrs := waBinary.Attrs{
		"id":    query.ID,
		"xmlns": query.Namespace,
		"type":  string(query.Type),
	}
	if query.SMaxID != "" {
		attrs["smax_id"] = query.SMaxID
	}
	if !query.To.IsEmpty() {
		attrs["to"] = query.To
	}
	if !query.Target.IsEmpty() {
		attrs["target"] = query.Target
	}

	// 🔒 FIX: Log antes de enviar (INFO para garantir que apareça)
	cli.Log.Infof("sendIQAsyncAndGetData: sending IQ, id=%s, namespace=%s, to=%s, target=%s", query.ID, query.Namespace, attrs["to"], attrs["target"])

	data, err := cli.sendNodeAndGetData(ctx, waBinary.Node{
		Tag:     "iq",
		Attrs:   attrs,
		Content: query.Content,
	})
	if err != nil {
		cli.Log.Warnf("sendIQAsyncAndGetData: sendNodeAndGetData failed, id=%s, err=%v", query.ID, err)
		cli.cancelResponse(query.ID, waiter)
		return nil, data, err
	}

	cli.Log.Infof("sendIQAsyncAndGetData: IQ sent successfully, id=%s", query.ID)
	return waiter, data, nil
}

func (cli *Client) sendIQAsync(ctx context.Context, query infoQuery) (<-chan *waBinary.Node, error) {
	ch, _, err := cli.sendIQAsyncAndGetData(ctx, &query)
	return ch, err
}

const defaultRequestTimeout = 75 * time.Second

func (cli *Client) sendIQ(ctx context.Context, query infoQuery) (*waBinary.Node, error) {
	if query.Timeout == 0 {
		query.Timeout = defaultRequestTimeout
	}

	// 🔒 FIX: Log da requisição IQ (INFO para garantir que apareça)
	cli.Log.Infof("sendIQ: namespace=%s, id=%s, timeout=%v", query.Namespace, query.ID, query.Timeout)

	resChan, data, err := cli.sendIQAsyncAndGetData(ctx, &query)
	if err != nil {
		cli.Log.Warnf("sendIQ: sendIQAsyncAndGetData failed: %v", err)
		return nil, err
	}

	cli.Log.Infof("sendIQ: waiting for response, id=%s", query.ID)

	select {
	case res := <-resChan:
		cli.Log.Infof("sendIQ: received response, id=%s, tag=%s", query.ID, res.Tag)
		if isDisconnectNode(res) {
			cli.Log.Warnf("sendIQ: received disconnect node, id=%s", query.ID)
			if query.NoRetry {
				return nil, &DisconnectedError{Action: "info query", Node: res}
			}
			res, err = cli.retryFrame(ctx, "info query", query.ID, data, res, query.Timeout)
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
	case <-ctx.Done():
		cli.Log.Warnf("sendIQ: context cancelled, id=%s, err=%v", query.ID, ctx.Err())
		return nil, ctx.Err()
	case <-time.After(query.Timeout):
		cli.Log.Warnf("sendIQ: timeout after %v, id=%s", query.Timeout, query.ID)
		return nil, ErrIQTimedOut
	}
}

func (cli *Client) retryFrame(
	ctx context.Context,
	reqType,
	id string,
	data []byte,
	origResp *waBinary.Node,
	timeout time.Duration,
) (*waBinary.Node, error) {
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
	err := sock.SendFrame(ctx, data)
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
