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
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L27-L33
	ag := node.AttrGetter()
	if ag.String("class") != "call" {
		return
	}

	callID := ackCallID(node)
	groupUpdate, isGroup, groupErr := voip.ParseInitialGroupCallAck(node)
	if isGroup && groupUpdate != nil {
		callID = groupUpdate.CallID
	}
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
	if groupErr != nil {
		cli.Log.Warnf("Failed to parse initial group call ACK, call_id: %s: %v", callID, groupErr)
		return
	}
	cs := cli.getCall(callID)
	if cs == nil {
		return
	}
	if isGroup {
		if !cli.applyGroupUpdate(*groupUpdate) {
			cli.Log.Debugf(
				"Ignoring unapplied initial group call ACK, call_id: %s, transaction_id: %d",
				groupUpdate.CallID,
				groupUpdate.TransactionID,
			)
			return
		}
		meta.GroupJID = groupUpdate.GroupJID
		cli.dispatchEvent(&events.CallGroupUpdate{
			BasicCallMeta: meta,
			Update:        *groupUpdate,
			Data:          node,
		})
		cli.applyVoipSettingsCodec(cs, node, callID)
		cli.maybeEmitMediaReady(cs)
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
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L27-L33
	if en := voip.FindChild(node, "error"); en != nil {
		if id := en.AttrGetter().String("call-id"); id != "" {
			return id
		}
	}
	if r := voip.FindRelay(node); r != nil {
		if id := r.AttrGetter().String("call-id"); id != "" {
			return id
		}
	}
	if groupInfo := voip.FindChild(node, "group_info"); groupInfo != nil {
		return groupInfo.AttrGetter().String("call-id")
	}
	return ""
}
