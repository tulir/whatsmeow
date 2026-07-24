// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package voip implements WhatsApp call stanza construction and relay parsing.
package voip

import (
	"bytes"
	"strconv"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

var capabilityOffer = []byte{0x01, 0x05, 0xf7, 0x09, 0xe0, 0xbb, 0x13}

var capabilityVideoOffer = []byte{0x01, 0x05, 0xf7, 0x09, 0xe0, 0xfa, 0x13}

var capabilityPreaccept = []byte{0x01, 0x05, 0xf7, 0x09, 0xe0, 0xbb, 0x07}

func encodeLatency(rttMs uint32) string {
	return strconv.FormatUint(uint64(0x02000000+rttMs), 10)
}

type offerDeviceKey struct {
	DeviceJID  types.JID
	Ciphertext []byte
	EncType    string
}

type offerParams struct {
	CallID         string
	To             types.JID
	CallCreator    types.JID
	DeviceKeys     []offerDeviceKey
	PrivacyToken   []byte
	Capability     []byte
	DeviceIdentity []byte
	Video          bool
}

func buildCallOffer(p *offerParams) waBinary.Node {
	var children []waBinary.Node
	if p.PrivacyToken != nil {
		children = append(children, waBinary.Node{Tag: "privacy", Content: p.PrivacyToken})
	}
	children = append(children, audioOpus("8000"), audioOpus("16000"))
	if p.Video {
		children = append(children, callVideoOfferNode())
	}
	children = append(children, waBinary.Node{Tag: "net", Attrs: waBinary.Attrs{"medium": "3"}})
	capability := p.Capability
	if p.Video && bytes.Equal(capability, capabilityOffer) {
		capability = capabilityVideoOffer
	}
	if capability != nil {
		children = append(children, waBinary.Node{Tag: "capability", Attrs: waBinary.Attrs{"ver": "1"}, Content: capability})
	}
	if len(p.DeviceKeys) > 1 {
		tos := make([]waBinary.Node, len(p.DeviceKeys))
		for i, dk := range p.DeviceKeys {
			tos[i] = waBinary.Node{Tag: "to", Attrs: waBinary.Attrs{"jid": dk.DeviceJID}, Content: []waBinary.Node{encNode(dk)}}
		}
		children = append(children, waBinary.Node{Tag: "destination", Content: tos})
	} else if len(p.DeviceKeys) == 1 {
		children = append(children, encNode(p.DeviceKeys[0]))
	}
	children = append(children, waBinary.Node{Tag: "encopt", Attrs: waBinary.Attrs{"keygen": "2"}})
	if p.DeviceIdentity != nil {
		children = append(children, waBinary.Node{Tag: "device-identity", Content: p.DeviceIdentity})
	}
	return callWrap(p.To, nil, offerAction("offer", p.CallID, p.CallCreator, children))
}

func encNode(dk offerDeviceKey) waBinary.Node {
	return waBinary.Node{
		Tag:     "enc",
		Attrs:   waBinary.Attrs{"v": "2", "type": dk.EncType, "count": "0"},
		Content: dk.Ciphertext,
	}
}

type acceptParams struct {
	CallID       string
	To           types.JID
	CallCreator  types.JID
	AudioRates   []string
	RelayTe      []byte
	Rte          []byte
	VoipSettings []byte
	Capability   []byte
	Metadata     waBinary.Attrs
	Video        bool
}

func buildAccept(p *acceptParams) waBinary.Node {
	children := make([]waBinary.Node, 0, len(p.AudioRates)+5)
	for _, rate := range p.AudioRates {
		children = append(children, audioOpus(rate))
	}
	if p.Video {
		children = append(children, callVideoAcceptNode())
	}
	if p.RelayTe != nil {
		children = append(children, waBinary.Node{Tag: "te", Attrs: waBinary.Attrs{"priority": "2"}, Content: p.RelayTe})
	}
	children = append(children, waBinary.Node{Tag: "net", Attrs: waBinary.Attrs{"medium": "2"}})
	children = append(children, waBinary.Node{Tag: "encopt", Attrs: waBinary.Attrs{"keygen": "2"}})
	if p.Capability != nil {
		children = append(children, waBinary.Node{Tag: "capability", Attrs: waBinary.Attrs{"ver": "1"}, Content: p.Capability})
	}
	if p.Metadata != nil {
		children = append(children, waBinary.Node{Tag: "metadata", Attrs: p.Metadata})
	}
	if p.Rte != nil {
		children = append(children, waBinary.Node{Tag: "rte", Content: p.Rte})
	}
	if p.VoipSettings != nil {
		children = append(children, waBinary.Node{Tag: "voip_settings", Attrs: waBinary.Attrs{"uncompressed": "1"}, Content: p.VoipSettings})
	}
	return callWrap(p.To, nil, offerAction("accept", p.CallID, p.CallCreator, children))
}

func audioOpus(rate string) waBinary.Node {
	return waBinary.Node{Tag: "audio", Attrs: waBinary.Attrs{"enc": "opus", "rate": rate}}
}

func buildPreaccept(callID string, to, callCreator types.JID, wrapperID string, audioRates []string, video bool) waBinary.Node {
	children := make([]waBinary.Node, 0, len(audioRates)+3)
	for _, rate := range audioRates {
		children = append(children, audioOpus(rate))
	}
	if video {
		children = append(children, callVideoPreacceptNode())
	}
	children = append(children, waBinary.Node{Tag: "encopt", Attrs: waBinary.Attrs{"keygen": "2"}})
	capability := capabilityPreaccept
	if video {
		capability = capabilityOffer
	}
	children = append(children, waBinary.Node{Tag: "capability", Attrs: waBinary.Attrs{"ver": "1"}, Content: capability})
	return callWrap(to, &wrapperID, offerAction("preaccept", callID, callCreator, children))
}

func buildEagerPreaccept(callID string, to, callCreator types.JID, requestID string, video bool) waBinary.Node {
	node := buildPreaccept(callID, to, callCreator, requestID, []string{"16000"}, video)
	actions := node.GetChildren()
	children := actions[0].GetChildren()
	for i := range children {
		if children[i].Tag == "capability" {
			children[i].Content = capabilityOffer
		}
	}
	actions[0].Content = children
	node.Content = actions
	return node
}

type transportParams struct {
	CallID               string
	To                   types.JID
	CallCreator          types.JID
	P2PCandRound         *string
	TransportMessageType *string
	RelayTe              []byte
}

func buildTransport(p *transportParams) waBinary.Node {
	attrs := waBinary.Attrs{"call-id": p.CallID, "call-creator": p.CallCreator}
	if p.P2PCandRound != nil {
		attrs["p2p-cand-round"] = *p.P2PCandRound
	}
	if p.TransportMessageType != nil {
		attrs["transport-message-type"] = *p.TransportMessageType
	}
	var children []waBinary.Node
	if p.RelayTe != nil {
		children = append(children, waBinary.Node{Tag: "te", Attrs: waBinary.Attrs{"priority": "1"}, Content: p.RelayTe})
	}
	netAttrs := waBinary.Attrs{"medium": "2"}
	if p.TransportMessageType == nil || *p.TransportMessageType != "9" {
		netAttrs["protocol"] = "0"
	}
	children = append(children, waBinary.Node{Tag: "net", Attrs: netAttrs})
	return callWrap(p.To, nil, waBinary.Node{Tag: "transport", Attrs: attrs, Content: children})
}

type relayLatencyParams struct {
	CallID       string
	To           types.JID
	CallCreator  types.JID
	LatencyMs    uint32
	RelayName    string
	AddressBytes []byte
	Devices      []types.JID
}

func buildRelayLatency(p *relayLatencyParams) waBinary.Node {
	children := []waBinary.Node{{
		Tag:     "te",
		Attrs:   waBinary.Attrs{"latency": encodeLatency(p.LatencyMs), "relay_name": p.RelayName},
		Content: p.AddressBytes,
	}}
	if len(p.Devices) > 0 {
		children = append(children, destinationTo(p.Devices))
	}
	return callWrap(p.To, nil, offerAction("relaylatency", p.CallID, p.CallCreator, children))
}

func buildHeartbeat(callID string, callCreator types.JID, wrapperID string) waBinary.Node {
	action := waBinary.Node{Tag: "heartbeat", Attrs: waBinary.Attrs{"call-id": callID, "call-creator": callCreator}}
	return waBinary.Node{
		Tag:     "call",
		Attrs:   waBinary.Attrs{"to": callID + "@call", "id": wrapperID},
		Content: []waBinary.Node{action},
	}
}

type terminateParams struct {
	CallID        string
	To            types.JID
	CallCreator   types.JID
	Reason        *string
	TargetDevices []types.JID
}

func buildTerminate(p *terminateParams) waBinary.Node {
	attrs := waBinary.Attrs{"call-id": p.CallID, "call-creator": p.CallCreator}
	if p.Reason != nil {
		attrs["reason"] = *p.Reason
	}
	var content []waBinary.Node
	if len(p.TargetDevices) > 0 {
		content = []waBinary.Node{destinationTo(p.TargetDevices)}
	}
	return callWrap(p.To, nil, waBinary.Node{Tag: "terminate", Attrs: attrs, Content: content})
}

func buildMuteV2(callID string, to, callCreator types.JID, muteState string) waBinary.Node {
	action := waBinary.Node{Tag: "mute_v2", Attrs: waBinary.Attrs{"call-id": callID, "call-creator": callCreator, "mute-state": muteState}}
	return callWrap(to, nil, action)
}

func buildReject(callID string, to, callCreator types.JID) waBinary.Node {
	action := waBinary.Node{Tag: "reject", Attrs: waBinary.Attrs{"call-id": callID, "call-creator": callCreator}}
	return callWrap(to, nil, action)
}

func offerAction(tag, callID string, callCreator types.JID, children []waBinary.Node) waBinary.Node {
	return waBinary.Node{
		Tag:     tag,
		Attrs:   waBinary.Attrs{"call-id": callID, "call-creator": callCreator},
		Content: children,
	}
}

func destinationTo(devices []types.JID) waBinary.Node {
	tos := make([]waBinary.Node, len(devices))
	for i, jid := range devices {
		tos[i] = waBinary.Node{Tag: "to", Attrs: waBinary.Attrs{"jid": jid}}
	}
	return waBinary.Node{Tag: "destination", Content: tos}
}

func callWrap(to types.JID, id *string, action waBinary.Node) waBinary.Node {
	attrs := waBinary.Attrs{"to": to}
	if id != nil {
		attrs["id"] = *id
	}
	return waBinary.Node{Tag: "call", Attrs: attrs, Content: []waBinary.Node{action}}
}

// DeviceKey is an encrypted call key addressed to one companion device.
type DeviceKey = offerDeviceKey

// OfferParams contains the wire fields needed to build a call offer.
type OfferParams = offerParams

// AcceptParams contains the wire fields needed to build a call accept.
type AcceptParams = acceptParams

// RelayLatencyParams contains one relay latency response.
type RelayLatencyParams = relayLatencyParams

// TerminateParams contains the wire fields needed to terminate a call.
type TerminateParams = terminateParams

// CapabilityOffer is the capability blob used in an audio call offer.
var CapabilityOffer = capabilityOffer

// BuildOffer builds an outgoing call offer stanza.
func BuildOffer(p *OfferParams) waBinary.Node {
	return buildCallOffer(p)
}

// BuildAccept builds a call acceptance stanza.
func BuildAccept(p *AcceptParams) waBinary.Node {
	return buildAccept(p)
}

// BuildEagerPreaccept builds the immediate preaccept sent for an incoming offer.
func BuildEagerPreaccept(callID string, to, creator types.JID, requestID string, video bool) waBinary.Node {
	return buildEagerPreaccept(callID, to, creator, requestID, video)
}

// BuildRelayLatency builds a relay latency response stanza.
func BuildRelayLatency(p *RelayLatencyParams) waBinary.Node {
	return buildRelayLatency(p)
}

// BuildTerminate builds a call termination stanza.
func BuildTerminate(p *TerminateParams) waBinary.Node {
	return buildTerminate(p)
}

// BuildMute builds a local mute state stanza.
func BuildMute(callID string, to, creator types.JID, state string) waBinary.Node {
	return buildMuteV2(callID, to, creator, state)
}
