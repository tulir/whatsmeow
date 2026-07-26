// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"

	"go.mau.fi/whatsmeow/types"
)

type groupCallState struct {
	snapshot types.GroupCallUpdate
}

// applyGroupUpdate applies a server group-call snapshot to an existing call.
func (cli *Client) applyGroupUpdate(update types.GroupCallUpdate) bool {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/699185f41519da3177c17ea6a10f9d4aa48b6941/datasheets/voip-group-call-state.md#L60
	cli.callsLock.Lock()

	cs := cli.calls[update.CallID]
	if cs == nil {
		cli.callsLock.Unlock()
		return false
	}
	if cs.group != nil && update.TransactionID <= cs.group.snapshot.TransactionID {
		cli.callsLock.Unlock()
		return false
	}
	cs.group = &groupCallState{snapshot: cloneGroupCallUpdate(update)}
	cs.connected = true
	cs.inWaitingRoom = false
	cancel := cs.waitingRoomCancel
	cs.waitingRoomCancel = nil
	cli.callsLock.Unlock()
	if cancel != nil {
		cancel()
	}
	return true
}

// signalingTarget returns the destination for call-wide signaling.
func (cs *callState) signalingTarget() types.JID {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/699185f41519da3177c17ea6a10f9d4aa48b6941/datasheets/voip-group-call-state.md#L62-L68
	if cs.group == nil && cs.linkToken == "" {
		return cs.to
	}
	return types.NewJID(cs.meta.CallID, "call")
}

func cloneGroupCallUpdate(update types.GroupCallUpdate) types.GroupCallUpdate {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L183-L191
	cloned := update
	if update.Participants != nil {
		cloned.Participants = make([]types.GroupCallParticipant, len(update.Participants))
		for i, participant := range update.Participants {
			cloned.Participants[i] = participant
			if participant.Devices != nil {
				cloned.Participants[i].Devices = make([]types.GroupCallDevice, len(participant.Devices))
				for j, device := range participant.Devices {
					cloned.Participants[i].Devices[j] = device
					cloned.Participants[i].Devices[j].Capability = bytes.Clone(device.Capability)
				}
			}
		}
	}
	if update.Relay != nil {
		relay := *update.Relay
		relay.Key = bytes.Clone(update.Relay.Key)
		relay.HBHKey = bytes.Clone(update.Relay.HBHKey)
		if update.Relay.Tokens != nil {
			relay.Tokens = make([][]byte, len(update.Relay.Tokens))
			for i, token := range update.Relay.Tokens {
				relay.Tokens[i] = bytes.Clone(token)
			}
		}
		if update.Relay.AuthTokens != nil {
			relay.AuthTokens = make([][]byte, len(update.Relay.AuthTokens))
			for i, token := range update.Relay.AuthTokens {
				relay.AuthTokens[i] = bytes.Clone(token)
			}
		}
		if update.Relay.Endpoints != nil {
			relay.Endpoints = make([]types.GroupCallRelayEndpoint, len(update.Relay.Endpoints))
			for i, endpoint := range update.Relay.Endpoints {
				relay.Endpoints[i] = endpoint
				relay.Endpoints[i].Address = bytes.Clone(endpoint.Address)
			}
		}
		cloned.Relay = &relay
	}
	return cloned
}
