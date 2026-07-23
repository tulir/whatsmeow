// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import "go.mau.fi/whatsmeow/types"

type groupCallState struct {
	snapshot types.GroupCallUpdate
}

// applyGroupUpdate applies a server group-call snapshot to an existing call.
func (cli *Client) applyGroupUpdate(update types.GroupCallUpdate) bool {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/699185f41519da3177c17ea6a10f9d4aa48b6941/datasheets/voip-group-call-state.md#L60
	cli.callsLock.Lock()
	defer cli.callsLock.Unlock()

	cs := cli.calls[update.CallID]
	if cs == nil {
		return false
	}
	if cs.group != nil && update.TransactionID <= cs.group.snapshot.TransactionID {
		return false
	}
	cs.group = &groupCallState{snapshot: update}
	return true
}

// signalingTarget returns the destination for call-wide signaling.
func (cs *callState) signalingTarget() types.JID {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/699185f41519da3177c17ea6a10f9d4aa48b6941/datasheets/voip-group-call-state.md#L62-L68
	if cs.group == nil {
		return cs.to
	}
	return types.NewJID(cs.meta.CallID, "call")
}
