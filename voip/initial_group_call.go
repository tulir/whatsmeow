// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package voip

import (
	"bytes"
	"fmt"
	"strconv"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

// InitialGroupOfferParams contains the wire fields for starting a group call.
type InitialGroupOfferParams struct {
	CallID       string
	CallCreator  types.JID
	GroupJID     types.JID
	Participants []types.GroupCallParticipant
}

// BuildInitialGroupOffer builds the initial offer for an ad-hoc or group-bound group call.
func BuildInitialGroupOffer(params InitialGroupOfferParams) (waBinary.Node, error) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L63-L101
	if params.CallID == "" {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build initial group offer: call ID is required")
	}
	if params.CallCreator.IsEmpty() {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build initial group offer: call creator is required")
	}
	if len(params.Participants) < 3 {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build initial group offer: self and at least two remote participants are required")
	}

	users := make([]waBinary.Node, len(params.Participants))
	for participantIndex, participant := range params.Participants {
		if participant.JID.IsEmpty() {
			return waBinary.Node{}, fmt.Errorf("whatsmeow: build initial group offer: participant %d JID is required", participantIndex)
		}
		if len(participant.Devices) == 0 {
			return waBinary.Node{}, fmt.Errorf("whatsmeow: build initial group offer: participant %d devices are required", participantIndex)
		}
		devices := make([]waBinary.Node, len(participant.Devices))
		for deviceIndex, device := range participant.Devices {
			if device.JID.IsEmpty() {
				return waBinary.Node{}, fmt.Errorf(
					"whatsmeow: build initial group offer: participant %d device %d JID is required",
					participantIndex,
					deviceIndex,
				)
			}
			var content []waBinary.Node
			if device.Capability != nil {
				capabilityAttrs := make(waBinary.Attrs)
				if device.CapabilityVersion != 0 {
					capabilityAttrs["ver"] = strconv.FormatUint(uint64(device.CapabilityVersion), 10)
				}
				content = []waBinary.Node{{
					Tag:     "capability",
					Attrs:   capabilityAttrs,
					Content: bytes.Clone(device.Capability),
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
			Attrs:   waBinary.Attrs{"jid": participant.JID},
			Content: devices,
		}
	}

	children := []waBinary.Node{
		audioOpus("8000"),
		audioOpus("16000"),
		{Tag: "net", Attrs: waBinary.Attrs{"medium": "3"}},
		{Tag: "group_info", Content: users},
	}
	offer := offerAction("offer", params.CallID, params.CallCreator, children)
	if !params.GroupJID.IsEmpty() {
		offer.Attrs["group-jid"] = params.GroupJID
	}
	return callWrap(types.NewJID(params.CallID, "call"), nil, offer), nil
}

// ParseInitialGroupCallAck parses the group snapshot carried by an initial offer ACK.
func ParseInitialGroupCallAck(node *waBinary.Node) (*types.GroupCallUpdate, bool, error) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L27-L33
	if node == nil {
		return nil, false, fmt.Errorf("whatsmeow: parse initial group call ACK: nil node")
	}
	if node.Tag != "ack" {
		return nil, false, fmt.Errorf("whatsmeow: parse initial group call ACK: unexpected tag %q", node.Tag)
	}
	groupInfo, ok := node.GetOptionalChildByTag("group_info")
	if !ok {
		return nil, false, nil
	}

	attrs := groupInfo.AttrGetter()
	update := &types.GroupCallUpdate{
		CallID:      attrs.String("call-id"),
		CallCreator: attrs.JID("call-creator"),
	}
	if err := attrs.Error(); err != nil {
		return nil, true, fmt.Errorf("whatsmeow: parse initial group call identity: %w", err)
	}
	if err := parseGroupInfo(&groupInfo, update); err != nil {
		return nil, true, err
	}
	if relay, ok := node.GetOptionalChildByTag("relay"); ok {
		parsed, err := parseGroupRelay(&relay)
		if err != nil {
			return nil, true, err
		}
		update.Relay = parsed
	}
	return update, true, nil
}
