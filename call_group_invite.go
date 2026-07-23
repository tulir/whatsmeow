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

var errGroupParticipantInviteNotImplemented = errors.New("whatsmeow: group participant invite is not implemented")

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
	// TODO
	// agent suggestion: parse the peer capability and atomically attach it to the existing call state.
	// human input:
	return errGroupParticipantInviteNotImplemented
}

func (cli *Client) groupInviteRoster(callID string) (types.JID, []types.GroupCallParticipant, error) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	// TODO
	// agent suggestion: deep-copy the canonical group roster when present, otherwise return the captured connected direct pair.
	// human input:
	return types.EmptyJID, nil, errGroupParticipantInviteNotImplemented
}

// InviteCallParticipant sends one singular invitation to add target to an active audio call.
func (cli *Client) InviteCallParticipant(ctx context.Context, callID string, target types.JID) error {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	// TODO
	// agent suggestion: resolve and validate one target, discover its devices, build one verified offer, stamp one ID, and send once.
	// human input:
	return errGroupParticipantInviteNotImplemented
}
