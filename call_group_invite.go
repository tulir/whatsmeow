// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

var errGroupParticipantInviteNotImplemented = errors.New("whatsmeow: group participant invite is not implemented")

func parseCallInviteDevice(device types.JID, node *waBinary.Node) (types.GroupCallDevice, error) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	// TODO
	// agent suggestion: parse and clone the captured capability child into the selected active device.
	// human input:
	return types.GroupCallDevice{}, errGroupParticipantInviteNotImplemented
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
