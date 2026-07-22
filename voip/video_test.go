// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package voip

import (
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func TestCallVideoOfferShape(t *testing.T) {
	peer, creator := stanzaPeerJID(), stanzaCreatorJID()
	call := buildCallOffer(&offerParams{
		CallID: "CID", To: peer, CallCreator: creator, Video: true,
		Capability: capabilityOffer,
		DeviceKeys: []offerDeviceKey{{DeviceJID: peer, Ciphertext: []byte{1}, EncType: "msg"}},
	})
	want := []string{"audio", "audio", "video", "net", "capability", "enc", "encopt"}
	if got := stanzaChildTags(t, call); !stanzaEqTags(got, want) {
		t.Fatalf("video offer tags = %v, want %v", got, want)
	}
	offer := stanzaContentNodes(t, call)[0]
	video, ok := stanzaGetChild(t, offer, "video")
	if !ok {
		t.Fatal("video offer is missing video child")
	}
	if video.AttrGetter().String("enc") != "h.264" || video.AttrGetter().String("dec") != "H264" {
		t.Fatalf("video offer codecs = %v", video.Attrs)
	}
	capability, _ := stanzaGetChild(t, offer, "capability")
	if got := capability.Content.([]byte); string(got) != string(capabilityVideoOffer) {
		t.Fatalf("video capability = %x, want %x", got, capabilityVideoOffer)
	}
}

func TestCallVideoAcceptAndPreacceptShapes(t *testing.T) {
	peer, creator := stanzaPeerJID(), stanzaCreatorJID()
	accept := buildAccept(&acceptParams{CallID: "CID", To: peer, CallCreator: creator, AudioRates: []string{"16000"}, Video: true})
	if got, want := stanzaChildTags(t, accept), []string{"audio", "video", "net", "encopt"}; !stanzaEqTags(got, want) {
		t.Fatalf("video accept tags = %v, want %v", got, want)
	}
	preaccept := buildEagerPreaccept("CID", peer, creator, "wrapper", true)
	if got, want := stanzaChildTags(t, preaccept), []string{"audio", "video", "encopt", "capability"}; !stanzaEqTags(got, want) {
		t.Fatalf("video preaccept tags = %v, want %v", got, want)
	}
}

func TestCallVideoTransitionShapes(t *testing.T) {
	peer, creator := stanzaPeerJID(), stanzaCreatorJID()
	orientation := 3
	request := buildCallVideoState("CID", peer, creator, "request", types.CallVideoStateUpgradeRequestV2, &orientation)
	video := stanzaContentNodes(t, request)[0]
	if got := video.AttrGetter().String("state"); got != "11" {
		t.Fatalf("request state = %q, want 11", got)
	}
	if got := video.AttrGetter().String("dec"); got != "H264" {
		t.Fatalf("request dec = %q, want H264", got)
	}
	if got := video.AttrGetter().String("voip_settings"); got != "video" {
		t.Fatalf("request voip_settings = %q, want video", got)
	}
	if got := video.AttrGetter().String("device_orientation"); got != "3" {
		t.Fatalf("request orientation = %q, want 3", got)
	}

	accept := buildCallVideoState("CID", peer, creator, "accept", types.CallVideoStateUpgradeAccept, nil)
	video = stanzaContentNodes(t, accept)[0]
	if got := video.AttrGetter().String("dec"); got != "H264,AV1" {
		t.Fatalf("accept dec = %q, want H264,AV1", got)
	}
}

func TestCallVideoAckPreservesRouting(t *testing.T) {
	from := types.JID{User: "123", Server: types.HiddenUserServer, Device: 7}
	recipient := types.JID{User: "456", Server: types.HiddenUserServer}
	original := waBinary.Node{Tag: "call", Attrs: waBinary.Attrs{
		"id": "wrapper", "from": from, "participant": from, "recipient": recipient,
	}}
	ack, ok := buildCallVideoAck(&original)
	if !ok {
		t.Fatal("buildCallVideoAck rejected routable stanza")
	}
	if ack.AttrGetter().String("type") != "video" || ack.AttrGetter().JID("to") != from {
		t.Fatalf("video ack attrs = %v", ack.Attrs)
	}
	if ack.AttrGetter().JID("recipient") != recipient {
		t.Fatalf("video ack recipient = %s, want %s", ack.AttrGetter().JID("recipient"), recipient)
	}
}
