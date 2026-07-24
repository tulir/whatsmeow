// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package voip

import (
	"strconv"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

const (
	videoDecH264  = "H264"
	videoDecReply = "H264,AV1"
)

func buildCallVideoState(callID string, to, creator types.JID, wrapperID string, state types.CallVideoState, orientation *int) waBinary.Node {
	attrs := waBinary.Attrs{
		"call-id":      callID,
		"call-creator": creator,
		"state":        strconv.Itoa(int(state)),
	}
	switch state {
	case types.CallVideoStateUpgradeRequestV2:
		attrs["dec"] = videoDecH264
		attrs["voip_settings"] = "video"
	case types.CallVideoStateUpgradeAccept:
		attrs["dec"] = videoDecReply
	}
	if orientation != nil {
		attrs["device_orientation"] = strconv.Itoa(*orientation)
	}
	return waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"to": to, "id": wrapperID},
		Content: []waBinary.Node{{
			Tag:   "video",
			Attrs: attrs,
		}},
	}
}

func buildCallVideoAck(original *waBinary.Node) (waBinary.Node, bool) {
	if original == nil {
		return waBinary.Node{}, false
	}
	ag := original.AttrGetter()
	id := ag.String("id")
	from := ag.JID("from")
	if id == "" || from.IsEmpty() {
		return waBinary.Node{}, false
	}
	attrs := waBinary.Attrs{"class": "call", "id": id, "to": from, "type": "video"}
	if participant := ag.JID("participant"); !participant.IsEmpty() && participant != from {
		attrs["participant"] = participant
	}
	if recipient := ag.JID("recipient"); !recipient.IsEmpty() {
		attrs["recipient"] = recipient
	}
	return waBinary.Node{Tag: "ack", Attrs: attrs}, true
}

func callVideoOfferNode() waBinary.Node {
	return waBinary.Node{Tag: "video", Attrs: waBinary.Attrs{
		"enc":                "h.264",
		"dec":                videoDecH264,
		"screen_width":       "1920",
		"screen_height":      "1080",
		"device_orientation": "0",
	}}
}

func callVideoAcceptNode() waBinary.Node {
	return waBinary.Node{Tag: "video", Attrs: waBinary.Attrs{
		"dec":                videoDecH264,
		"device_orientation": "0",
	}}
}

func callVideoPreacceptNode() waBinary.Node {
	return waBinary.Node{Tag: "video", Attrs: waBinary.Attrs{
		"dec":                videoDecH264,
		"device_orientation": "0",
		"screen_width":       "0",
		"screen_height":      "0",
	}}
}

func callOfferHasVideo(offer *waBinary.Node) bool {
	return childByTag(offer, "video") != nil
}

// BuildVideoState builds one independent local video-flow transition.
func BuildVideoState(callID string, to, creator types.JID, wrapperID string, state types.CallVideoState, orientation *int) waBinary.Node {
	return buildCallVideoState(callID, to, creator, wrapperID, state, orientation)
}

// BuildVideoAck builds the typed acknowledgement required by a video stanza.
func BuildVideoAck(original *waBinary.Node) (waBinary.Node, bool) {
	return buildCallVideoAck(original)
}

// OfferHasVideo reports whether a call offer contains a video capability.
func OfferHasVideo(offer *waBinary.Node) bool {
	return callOfferHasVideo(offer)
}
