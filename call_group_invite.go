// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/voip"
)

func parseCallInviteDevice(device types.JID, node *waBinary.Node) (types.GroupCallDevice, error) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	if device.IsEmpty() {
		return types.GroupCallDevice{}, errors.New("whatsmeow: parse call invite device: device is required")
	}
	if node == nil {
		return types.GroupCallDevice{}, errors.New("whatsmeow: parse call invite device: nil node")
	}
	capability := voip.FindChild(node, "capability")
	if capability == nil {
		return types.GroupCallDevice{}, errors.New("whatsmeow: parse call invite device: missing capability")
	}
	attrs := capability.AttrGetter()
	version, ok := attrs.GetUint64("ver", true)
	if err := attrs.Error(); err != nil {
		return types.GroupCallDevice{}, fmt.Errorf("whatsmeow: parse call invite device capability: %w", err)
	}
	if !ok || version > uint64(^uint32(0)) {
		return types.GroupCallDevice{}, fmt.Errorf("whatsmeow: parse call invite device: invalid capability version %d", version)
	}
	value := voip.NodeBytes(capability)
	if len(value) == 0 {
		return types.GroupCallDevice{}, errors.New("whatsmeow: parse call invite device: empty capability")
	}
	return types.GroupCallDevice{
		JID:               device,
		CapabilityVersion: uint32(version),
		Capability:        bytes.Clone(value),
	}, nil
}

func (cli *Client) capturePeerInviteDevice(callID string, device types.JID, node *waBinary.Node) error {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	parsed, err := parseCallInviteDevice(device, node)
	if err != nil {
		return err
	}
	cli.callsLock.Lock()
	defer cli.callsLock.Unlock()
	cs := cli.calls[callID]
	if cs == nil {
		return fmt.Errorf("whatsmeow: capture peer invite device: unknown call %s", callID)
	}
	cs.invitePeerDevice = parsed
	return nil
}

func (cli *Client) groupInviteRoster(callID string) (types.JID, []types.GroupCallParticipant, bool, error) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	if cli == nil {
		return types.EmptyJID, nil, false, ErrClientIsNil
	}
	cli.callsLock.Lock()
	defer cli.callsLock.Unlock()
	cs := cli.calls[callID]
	if cs == nil {
		return types.EmptyJID, nil, false, fmt.Errorf("whatsmeow: group invite roster: unknown call %s", callID)
	}
	if !cs.connected {
		return types.EmptyJID, nil, false, errors.New("whatsmeow: group invite roster: call is not connected")
	}
	video := cs.localVideo || cs.remoteVideo
	if cs.creator.IsEmpty() {
		return types.EmptyJID, nil, false, errors.New("whatsmeow: group invite roster: call creator is unavailable")
	}

	if cs.group != nil {
		if len(cs.group.snapshot.Participants) == 0 {
			return types.EmptyJID, nil, false, errors.New("whatsmeow: group invite roster: group roster is empty")
		}
		participants := make([]types.GroupCallParticipant, len(cs.group.snapshot.Participants))
		for participantIndex, participant := range cs.group.snapshot.Participants {
			participant.Devices = append([]types.GroupCallDevice(nil), participant.Devices...)
			for deviceIndex := range participant.Devices {
				participant.Devices[deviceIndex].Capability = bytes.Clone(participant.Devices[deviceIndex].Capability)
			}
			participants[participantIndex] = participant
		}
		return cs.creator, participants, video, nil
	}

	if cs.selfLID.IsEmpty() || cs.inviteSelfDevice.JID.IsEmpty() || len(cs.inviteSelfDevice.Capability) == 0 {
		return types.EmptyJID, nil, false, errors.New("whatsmeow: group invite roster: local active device capability is unavailable")
	}
	if cs.peerLID.IsEmpty() || cs.invitePeerDevice.JID.IsEmpty() || len(cs.invitePeerDevice.Capability) == 0 {
		return types.EmptyJID, nil, false, errors.New("whatsmeow: group invite roster: peer active device capability is unavailable")
	}
	participants := []types.GroupCallParticipant{
		{
			JID:   cs.selfLID.ToNonAD(),
			State: "connected",
			Devices: []types.GroupCallDevice{{
				JID:               cs.inviteSelfDevice.JID,
				CapabilityVersion: cs.inviteSelfDevice.CapabilityVersion,
				Capability:        bytes.Clone(cs.inviteSelfDevice.Capability),
			}},
		},
		{
			JID:   cs.peerLID.ToNonAD(),
			State: "connected",
			Devices: []types.GroupCallDevice{{
				JID:               cs.invitePeerDevice.JID,
				CapabilityVersion: cs.invitePeerDevice.CapabilityVersion,
				Capability:        bytes.Clone(cs.invitePeerDevice.Capability),
			}},
		},
	}
	return cs.creator, participants, video, nil
}

// InviteCallParticipant sends one singular invitation to add target to an active audio call.
func (cli *Client) InviteCallParticipant(ctx context.Context, callID string, target types.JID) error {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	if cli == nil {
		return ErrClientIsNil
	}
	if callID == "" {
		return errors.New("whatsmeow: invite call participant: call ID is required")
	}
	if target.IsEmpty() {
		return errors.New("whatsmeow: invite call participant: target is required")
	}

	creator, participants, video, err := cli.groupInviteRoster(callID)
	if err != nil {
		return err
	}
	peer, err := cli.resolvePeerCallLID(ctx, target)
	if err != nil {
		return fmt.Errorf("whatsmeow: resolve call invite target: %w", err)
	}
	peer = peer.ToNonAD()
	for _, participant := range participants {
		if participant.JID.ToNonAD() == peer {
			return fmt.Errorf("whatsmeow: invite target %s already belongs to the call", peer)
		}
	}

	devices, err := cli.GetUserDevices(ctx, []types.JID{peer})
	if err != nil {
		return fmt.Errorf("whatsmeow: call invite device discovery: %w", err)
	}
	if len(devices) == 0 {
		return fmt.Errorf("whatsmeow: call invite target %s has no devices", peer)
	}
	offer, err := voip.BuildGroupInviteOffer(voip.GroupInviteOfferParams{
		CallID:        callID,
		To:            peer,
		CallCreator:   creator,
		TargetDevices: devices,
		Participants:  participants,
		Video:         video,
	})
	if err != nil {
		return err
	}
	offer.Attrs["id"] = cli.GenerateMessageID()
	if err = cli.sendNode(ctx, offer); err != nil {
		return fmt.Errorf("whatsmeow: send group participant invite: %w", err)
	}
	cli.Log.Debugf(
		"Sent group participant invite, call_id: %s, target_lid: %s, device_count: %d, participant_count: %d",
		callID, peer, len(devices), len(participants),
	)
	return nil
}
