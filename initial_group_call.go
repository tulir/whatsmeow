// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/voip"
)

// GroupCallOfferOptions configures a new outgoing group call.
type GroupCallOfferOptions struct {
	GroupJID types.JID
}

type initialGroupCallStartDependencies struct {
	resolve   func(context.Context, types.JID) (types.JID, error)
	discover  func(context.Context, []types.JID) ([]types.JID, error)
	callID    func() string
	requestID func() string
	send      func(context.Context, waBinary.Node) error
	install   func(string, *callState)
	remove    func(string, *callState)
}

func runInitialGroupCallStart(
	ctx context.Context,
	self types.JID,
	targets []types.JID,
	options GroupCallOfferOptions,
	deps initialGroupCallStartDependencies,
) (string, error) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L164-L181
	if self.IsEmpty() {
		return "", fmt.Errorf("whatsmeow: not logged in: %w", ErrNotLoggedIn)
	}
	if len(targets) < 2 {
		return "", fmt.Errorf("whatsmeow: start group call: at least two remote targets are required")
	}
	if deps.resolve == nil || deps.discover == nil || deps.callID == nil ||
		deps.requestID == nil || deps.send == nil || deps.install == nil || deps.remove == nil {
		return "", fmt.Errorf("whatsmeow: start group call: incomplete dependencies")
	}

	selfDevice := self
	selfBare := self.ToNonAD()
	resolved := make([]types.JID, len(targets))
	seen := make(map[types.JID]struct{}, len(targets))
	for i, target := range targets {
		if target.IsEmpty() {
			return "", fmt.Errorf("whatsmeow: start group call: target %d is empty", i)
		}
		peer, err := deps.resolve(ctx, target)
		if err != nil {
			return "", fmt.Errorf("whatsmeow: resolve group call target %d: %w", i, err)
		}
		peer = peer.ToNonAD()
		if peer.IsEmpty() {
			return "", fmt.Errorf("whatsmeow: start group call: resolved target %d is empty", i)
		}
		if peer == selfBare {
			return "", fmt.Errorf("whatsmeow: start group call: target %d is self", i)
		}
		if _, exists := seen[peer]; exists {
			return "", fmt.Errorf("whatsmeow: start group call: duplicate target %s", peer)
		}
		seen[peer] = struct{}{}
		resolved[i] = peer
	}

	devices, err := deps.discover(ctx, resolved)
	if err != nil {
		return "", fmt.Errorf("whatsmeow: group call device discovery: %w", err)
	}
	devicesByTarget := make(map[types.JID][]types.JID, len(resolved))
	for _, device := range devices {
		owner := device.ToNonAD()
		if _, selected := seen[owner]; selected {
			devicesByTarget[owner] = append(devicesByTarget[owner], device)
		}
	}

	participants := make([]types.GroupCallParticipant, 1, len(resolved)+1)
	participants[0] = types.GroupCallParticipant{
		JID: selfBare,
		Devices: []types.GroupCallDevice{{
			JID:               selfDevice,
			CapabilityVersion: 1,
			Capability:        bytes.Clone(voip.CapabilityOffer),
		}},
	}
	for _, target := range resolved {
		targetDevices := devicesByTarget[target]
		if len(targetDevices) == 0 {
			return "", fmt.Errorf("whatsmeow: group call target %s has no devices", target)
		}
		participant := types.GroupCallParticipant{
			JID:     target,
			Devices: make([]types.GroupCallDevice, len(targetDevices)),
		}
		for i, device := range targetDevices {
			participant.Devices[i].JID = device
		}
		participants = append(participants, participant)
	}

	callID := deps.callID()
	offer, err := voip.BuildInitialGroupOffer(voip.InitialGroupOfferParams{
		CallID:       callID,
		CallCreator:  selfDevice,
		GroupJID:     options.GroupJID,
		Participants: participants,
	})
	if err != nil {
		return "", err
	}
	offer.Attrs["id"] = deps.requestID()
	state := newOutgoingGroupCallState(callID, selfDevice, resolved, options.GroupJID)
	deps.install(callID, state)
	if err = deps.send(ctx, offer); err != nil {
		deps.remove(callID, state)
		return callID, fmt.Errorf("whatsmeow: send initial group call offer: %w", err)
	}
	return callID, nil
}

// OfferGroupCall places an audio group call to at least two remote users.
func (cli *Client) OfferGroupCall(
	ctx context.Context,
	targets []types.JID,
	options ...GroupCallOfferOptions,
) (callID string, err error) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L164-L181
	// NOT VALIDATED: clear when a live group offer completes the ACK, rekey, and media-ready transition.
	if cli == nil {
		return "", ErrClientIsNil
	}
	var opts GroupCallOfferOptions
	if len(options) > 0 {
		opts = options[0]
	}
	callID, err = runInitialGroupCallStart(
		ctx,
		cli.getOwnLID(),
		targets,
		opts,
		initialGroupCallStartDependencies{
			resolve:  cli.resolvePeerCallLID,
			discover: cli.GetUserDevices,
			callID:   newCallID,
			requestID: func() string {
				return string(cli.GenerateMessageID())
			},
			send:    cli.sendNode,
			install: cli.putCall,
			remove:  cli.removeCallIfSame,
		},
	)
	if err != nil {
		return callID, err
	}
	cli.Log.Debugf(
		"Sent initial group call offer, call_id: %s, target_count: %d, group_bound: %t",
		callID,
		len(targets),
		!opts.GroupJID.IsEmpty(),
	)
	return callID, nil
}

func (cli *Client) removeCallIfSame(callID string, state *callState) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L178-L180
	cli.callsLock.Lock()
	defer cli.callsLock.Unlock()
	if cli.calls[callID] == state {
		delete(cli.calls, callID)
	}
}

func newOutgoingGroupCallState(
	callID string,
	self types.JID,
	targets []types.JID,
	groupJID types.JID,
) *callState {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L177-L181
	participants := make([]types.GroupCallParticipant, 1, len(targets)+1)
	participants[0] = types.GroupCallParticipant{JID: self.ToNonAD()}
	for _, target := range targets {
		participants = append(participants, types.GroupCallParticipant{JID: target.ToNonAD()})
	}
	firstTarget := targets[0].ToNonAD()
	return &callState{
		meta: types.BasicCallMeta{
			From:        self,
			Timestamp:   time.Now(),
			CallCreator: self,
			CallID:      callID,
			GroupJID:    groupJID,
		},
		selfLID:  self,
		peerLID:  firstTarget,
		to:       firstTarget,
		creator:  self,
		outgoing: true,
		group: &groupCallState{snapshot: types.GroupCallUpdate{
			CallID:       callID,
			CallCreator:  self,
			GroupJID:     groupJID,
			Participants: participants,
		}},
		inviteSelfDevice: types.GroupCallDevice{
			JID:               self,
			CapabilityVersion: 1,
			Capability:        bytes.Clone(voip.CapabilityOffer),
		},
	}
}

func groupMediaReadyFields(
	self types.JID,
	update types.GroupCallUpdate,
) (peer types.JID, relay types.RelayEndpoint, ok bool) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L103-L111
	selfBare := self.ToNonAD()
	for _, participant := range update.Participants {
		if participant.State != "connected" || participant.JID.ToNonAD() == selfBare {
			continue
		}
		for _, device := range participant.Devices {
			if device.HasPID && !device.JID.IsEmpty() && device.JID.ToNonAD() != selfBare {
				peer = device.JID
				break
			}
		}
		if !peer.IsEmpty() {
			break
		}
	}
	if peer.IsEmpty() || update.Relay == nil || len(update.Relay.Key) == 0 {
		return types.EmptyJID, types.RelayEndpoint{}, false
	}

	for _, endpoint := range update.Relay.Endpoints {
		if endpoint.IsFNA || len(endpoint.Address) != 6 ||
			endpoint.TokenID >= uint32(len(update.Relay.Tokens)) ||
			endpoint.AuthTokenID >= uint32(len(update.Relay.AuthTokens)) {
			continue
		}
		port := binary.BigEndian.Uint16(endpoint.Address[4:])
		token := update.Relay.Tokens[endpoint.TokenID]
		authToken := update.Relay.AuthTokens[endpoint.AuthTokenID]
		if port == 0 || len(token) == 0 || len(authToken) == 0 {
			continue
		}
		return peer, types.RelayEndpoint{
			RelayID:     endpoint.RelayID,
			TokenID:     endpoint.TokenID,
			AuthTokenID: endpoint.AuthTokenID,
			RelayName:   endpoint.RelayName,
			IsFNA:       endpoint.IsFNA,
			IPv4:        net.IP(endpoint.Address[:4]).String(),
			Port:        port,
			Key:         bytes.Clone(update.Relay.Key),
			Token:       bytes.Clone(token),
			AuthToken:   bytes.Clone(authToken),
		}, true
	}
	return types.EmptyJID, types.RelayEndpoint{}, false
}
