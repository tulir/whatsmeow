// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package voip

import (
	"fmt"
	"strconv"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

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
	if params.CallID == "" {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build group invite offer: call ID is required")
	}
	if params.To.IsEmpty() {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build group invite offer: target is required")
	}
	if params.CallCreator.IsEmpty() {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build group invite offer: call creator is required")
	}
	if len(params.TargetDevices) == 0 {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build group invite offer: target devices are required")
	}
	if len(params.Participants) == 0 {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build group invite offer: participants are required")
	}

	users := make([]waBinary.Node, len(params.Participants))
	for participantIndex, participant := range params.Participants {
		userAttrs := waBinary.Attrs{"jid": participant.JID}
		if participant.State != "" {
			userAttrs["state"] = participant.State
		}
		devices := make([]waBinary.Node, len(participant.Devices))
		for deviceIndex, device := range participant.Devices {
			var content []waBinary.Node
			if device.Capability != nil {
				capabilityAttrs := make(waBinary.Attrs)
				if device.CapabilityVersion != 0 {
					capabilityAttrs["ver"] = strconv.FormatUint(uint64(device.CapabilityVersion), 10)
				}
				content = []waBinary.Node{{
					Tag:     "capability",
					Attrs:   capabilityAttrs,
					Content: device.Capability,
				}}
			}
			devices[deviceIndex] = waBinary.Node{
				Tag:     "device",
				Attrs:   waBinary.Attrs{"jid": device.JID},
				Content: content,
			}
		}
		users[participantIndex] = waBinary.Node{
			Tag:     "user",
			Attrs:   userAttrs,
			Content: devices,
		}
	}

	children := []waBinary.Node{
		audioOpus("16000"),
		{Tag: "net", Attrs: waBinary.Attrs{"medium": "2"}},
		destinationTo(params.TargetDevices),
		{Tag: "group_info", Content: users},
	}
	return callWrap(params.To, nil, offerAction("offer", params.CallID, params.CallCreator, children)), nil
}
