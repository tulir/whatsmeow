// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package voip

import (
	"errors"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

var errGroupInviteOfferNotImplemented = errors.New("whatsmeow: group invite offer is not implemented")

// GroupInviteOfferParams contains the wire fields for inviting one participant to an active call.
type GroupInviteOfferParams struct {
	CallID        string
	To            types.JID
	CallCreator   types.JID
	TargetDevices []types.JID
	Participants  []types.GroupCallParticipant
}

// BuildGroupInviteOffer builds a singular active-call participant invite offer.
func BuildGroupInviteOffer(params GroupInviteOfferParams) (waBinary.Node, error) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/25eda415afb0f926112ca375c5892b95b4bd6f60/datasheets/voip-group-invite-offer.md#L81-L106
	// TODO
	// agent suggestion: validate the required identity, device, and roster fields, then emit the capture-ordered audio, net, destination, and group_info children.
	// human input:
	return waBinary.Node{}, errGroupInviteOfferNotImplemented
}
