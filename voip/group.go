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

// ParseGroupUpdate parses a group_update call-signaling node.
func ParseGroupUpdate(node *waBinary.Node) (*types.GroupCallUpdate, error) {
	if node == nil {
		return nil, fmt.Errorf("whatsmeow: parse group update: nil node")
	}
	if node.Tag != "group_update" {
		return nil, fmt.Errorf("whatsmeow: parse group update: unexpected tag %q", node.Tag)
	}

	attrs := node.AttrGetter()
	update := &types.GroupCallUpdate{
		CallID:      attrs.String("call-id"),
		CallCreator: attrs.JID("call-creator"),
	}
	if err := attrs.Error(); err != nil {
		return nil, fmt.Errorf("whatsmeow: parse group update attributes: %w", err)
	}

	groupInfo, ok := node.GetOptionalChildByTag("group_info")
	if !ok {
		return nil, fmt.Errorf("whatsmeow: parse group update: missing group_info")
	}
	if err := parseGroupInfo(&groupInfo, update); err != nil {
		return nil, err
	}
	if avUpgrade, ok := node.GetOptionalChildByTag("av_upgrade"); ok {
		update.AVUpgradable = avUpgrade.AttrGetter().OptionalString("av-upgradable") == "1"
	}
	if relay, ok := node.GetOptionalChildByTag("relay"); ok {
		parsed, err := parseGroupRelay(&relay)
		if err != nil {
			return nil, err
		}
		update.Relay = parsed
	}
	return update, nil
}

func parseGroupInfo(node *waBinary.Node, update *types.GroupCallUpdate) error {
	attrs := node.AttrGetter()
	update.GroupJID = attrs.JID("group-jid")
	update.Media = attrs.String("media")
	update.Joinable = attrs.OptionalString("joinable") == "1"
	var err error
	update.TransactionID, err = requiredUint32Attr(attrs, "transaction-id")
	if err != nil {
		return fmt.Errorf("whatsmeow: parse group transaction ID: %w", err)
	}
	update.ConnectedLimit, err = requiredUint32Attr(attrs, "connected-limit")
	if err != nil {
		return fmt.Errorf("whatsmeow: parse group connected limit: %w", err)
	}
	if err := attrs.Error(); err != nil {
		return fmt.Errorf("whatsmeow: parse group_info attributes: %w", err)
	}

	for _, child := range node.GetChildren() {
		if child.Tag != "user" {
			continue
		}
		participant, err := parseGroupParticipant(&child)
		if err != nil {
			return err
		}
		update.Participants = append(update.Participants, participant)
	}
	return nil
}

func parseGroupParticipant(node *waBinary.Node) (types.GroupCallParticipant, error) {
	attrs := node.AttrGetter()
	participant := types.GroupCallParticipant{
		JID:   attrs.JID("jid"),
		PN:    attrs.OptionalJIDOrEmpty("user_pn"),
		State: attrs.String("state"),
	}
	if err := attrs.Error(); err != nil {
		return participant, fmt.Errorf("whatsmeow: parse group participant attributes: %w", err)
	}

	for _, child := range node.GetChildren() {
		if child.Tag != "device" {
			continue
		}
		device, err := parseGroupDevice(&child)
		if err != nil {
			return participant, err
		}
		participant.Devices = append(participant.Devices, device)
	}
	return participant, nil
}

func parseGroupDevice(node *waBinary.Node) (types.GroupCallDevice, error) {
	attrs := node.AttrGetter()
	device := types.GroupCallDevice{
		JID:      attrs.JID("jid"),
		Platform: attrs.OptionalString("platform"),
	}
	var err error
	device.PID, device.HasPID, err = optionalUint32Attr(attrs, "pid")
	if err != nil {
		return device, fmt.Errorf("whatsmeow: parse group device PID: %w", err)
	}
	if attrErr := attrs.Error(); attrErr != nil {
		return device, fmt.Errorf("whatsmeow: parse group device attributes: %w", attrErr)
	}

	if capability, ok := node.GetOptionalChildByTag("capability"); ok {
		capAttrs := capability.AttrGetter()
		device.CapabilityVersion, _, err = optionalUint32Attr(capAttrs, "ver")
		if err != nil {
			return device, fmt.Errorf("whatsmeow: parse group device capability: %w", err)
		}
		if attrErr := capAttrs.Error(); attrErr != nil {
			return device, fmt.Errorf("whatsmeow: parse group device capability attributes: %w", attrErr)
		}
		device.Capability = nodeBytes(&capability)
	}
	return device, nil
}

func parseGroupRelay(node *waBinary.Node) (*types.GroupCallRelay, error) {
	attrs := node.AttrGetter()
	relay := &types.GroupCallRelay{
		UUID:             attrs.String("uuid"),
		ParticipantUUID:  attrs.String("participant_uuid"),
		AttributePadding: attrs.OptionalString("attribute_padding") == "1",
		Tokens:           parseIndexedTokens(node, "token"),
		AuthTokens:       parseIndexedTokens(node, "auth_token"),
	}
	var err error
	relay.TransactionID, _, err = optionalUint32Attr(attrs, "transaction-id")
	if err != nil {
		return nil, fmt.Errorf("whatsmeow: parse group relay transaction ID: %w", err)
	}
	relay.SelfPID, relay.HasSelfPID, err = optionalUint32Attr(attrs, "self_pid")
	if err != nil {
		return nil, fmt.Errorf("whatsmeow: parse group relay self PID: %w", err)
	}
	relay.WarpMITagLength, relay.HasWarpMITagLength, err = optionalUint32Attr(attrs, "warp_mi_tag_len")
	if err != nil {
		return nil, fmt.Errorf("whatsmeow: parse group relay WARP MI tag length: %w", err)
	}
	if attrErr := attrs.Error(); attrErr != nil {
		return nil, fmt.Errorf("whatsmeow: parse group relay attributes: %w", attrErr)
	}

	for _, child := range node.GetChildren() {
		switch child.Tag {
		case "key":
			relay.Key = nodeBytes(&child)
		case "hbh_key":
			relay.HBHKey = nodeBytes(&child)
		case "te2":
			endpoint, err := parseGroupRelayEndpoint(&child)
			if err != nil {
				return nil, err
			}
			relay.Endpoints = append(relay.Endpoints, endpoint)
		}
	}
	return relay, nil
}

func parseGroupRelayEndpoint(node *waBinary.Node) (types.GroupCallRelayEndpoint, error) {
	attrs := node.AttrGetter()
	endpoint := types.GroupCallRelayEndpoint{
		RelayName:  attrs.String("relay_name"),
		DomainName: attrs.OptionalString("domain_name"),
		IsFNA:      attrs.OptionalString("is_fna") == "1",
		Address:    nodeBytes(node),
	}
	var err error
	if endpoint.RelayID, _, err = optionalUint32Attr(attrs, "relay_id"); err != nil {
		return endpoint, fmt.Errorf("whatsmeow: parse group relay ID: %w", err)
	}
	if endpoint.TokenID, _, err = optionalUint32Attr(attrs, "token_id"); err != nil {
		return endpoint, fmt.Errorf("whatsmeow: parse group relay token ID: %w", err)
	}
	if endpoint.AuthTokenID, _, err = optionalUint32Attr(attrs, "auth_token_id"); err != nil {
		return endpoint, fmt.Errorf("whatsmeow: parse group relay auth token ID: %w", err)
	}
	if endpoint.RTT, _, err = optionalUint32Attr(attrs, "c2r_rtt"); err != nil {
		return endpoint, fmt.Errorf("whatsmeow: parse group relay RTT: %w", err)
	}
	if attrErr := attrs.Error(); attrErr != nil {
		return endpoint, fmt.Errorf("whatsmeow: parse group relay endpoint attributes: %w", attrErr)
	}
	return endpoint, nil
}

func optionalUint32Attr(attrs *waBinary.AttrUtility, key string) (uint32, bool, error) {
	raw, ok := attrs.GetString(key, false)
	if !ok {
		return 0, false, nil
	}
	value, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, true, fmt.Errorf("invalid %s %q: %w", key, raw, err)
	}
	return uint32(value), true, nil
}

func requiredUint32Attr(attrs *waBinary.AttrUtility, key string) (uint32, error) {
	value, ok, err := optionalUint32Attr(attrs, key)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	return value, nil
}
