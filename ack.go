// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/voip"
)

func (cli *Client) handleCallAck(ctx context.Context, node *waBinary.Node) {
	ag := node.AttrGetter()
	if ag.String("class") != "call" {
		return
	}

	callID := ackCallID(node)
	meta := types.BasicCallMeta{CallID: callID}
	if cs := cli.getCall(callID); cs != nil {
		meta = cs.meta
	}
	cli.dispatchEvent(&events.CallAck{BasicCallMeta: meta, Data: node})

	if errCode := ag.String("error"); errCode != "" {
		cli.Log.Warnf("Call rejected by server, call_id: %s, error_code: %s", callID, errCode)
		cli.dropCall(callID)
		cli.dispatchEvent(&events.CallMediaStop{BasicCallMeta: meta, Reason: "server:" + errCode})
		return
	}

	if callID == "" {
		return
	}
	cs := cli.getCall(callID)
	if cs == nil {
		return
	}
	ep := voip.ParseRelay(node, types.CallDirectionOutgoing)
	if ep == nil {
		return
	}
	cli.applyVoipSettingsCodec(cs, node, callID)
	cli.captureCallRelay(cs, node)
}

func ackCallID(node *waBinary.Node) string {
	if en := voip.FindChild(node, "error"); en != nil {
		if id := en.AttrGetter().String("call-id"); id != "" {
			return id
		}
	}
	if r := voip.FindRelay(node); r != nil {
		return r.AttrGetter().String("call-id")
	}
	return ""
}
