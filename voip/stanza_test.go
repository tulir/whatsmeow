// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package voip

import (
	"bytes"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func stanzaPeerJID() types.JID {
	return types.JID{User: "214482127208608", Server: types.HiddenUserServer}
}

func stanzaCreatorJID() types.JID {
	return types.JID{User: "243426515787784", Server: types.HiddenUserServer, Device: 19}
}

func stanzaContentNodes(t *testing.T, n waBinary.Node) []waBinary.Node {
	t.Helper()
	nodes, ok := n.Content.([]waBinary.Node)
	if !ok {
		t.Fatalf("node %q content is not []Node: %T", n.Tag, n.Content)
	}
	return nodes
}

func stanzaChildTags(t *testing.T, call waBinary.Node) []string {
	t.Helper()
	action := stanzaContentNodes(t, call)[0]
	var tags []string
	for _, c := range stanzaContentNodes(t, action) {
		tags = append(tags, c.Tag)
	}
	return tags
}

func stanzaGetChild(t *testing.T, n waBinary.Node, tag string) (waBinary.Node, bool) {
	t.Helper()
	for _, c := range stanzaContentNodes(t, n) {
		if c.Tag == tag {
			return c, true
		}
	}
	return waBinary.Node{}, false
}

func stanzaAttrString(n waBinary.Node, key string) (string, bool) {
	v, ok := n.Attrs[key].(string)
	return v, ok
}

func stanzaEqTags(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestOfferChildOrderIsLoadBearing(t *testing.T) {
	peer, creator := stanzaPeerJID(), stanzaCreatorJID()
	dk := offerDeviceKey{DeviceJID: peer, Ciphertext: []byte{1, 2, 3}, EncType: "pkmsg"}
	call := buildCallOffer(&offerParams{
		CallID: "CID", To: peer, CallCreator: creator,
		DeviceKeys:   []offerDeviceKey{dk},
		PrivacyToken: []byte{0xaa, 0xbb}, Capability: capabilityOffer, DeviceIdentity: []byte{0xcc},
	})
	want := []string{"privacy", "audio", "audio", "net", "capability", "enc", "encopt", "device-identity"}
	if got := stanzaChildTags(t, call); !stanzaEqTags(got, want) {
		t.Errorf("child tags = %v, want %v", got, want)
	}
	if call.Tag != "call" {
		t.Errorf("outer tag = %q, want call", call.Tag)
	}
	offer := stanzaContentNodes(t, call)[0]
	if offer.Tag != "offer" {
		t.Errorf("action tag = %q, want offer", offer.Tag)
	}
	if id, _ := stanzaAttrString(offer, "call-id"); id != "CID" {
		t.Errorf("call-id = %q, want CID", id)
	}
}

func TestOfferMultiDeviceUsesDestination(t *testing.T) {
	peer, creator := stanzaPeerJID(), stanzaCreatorJID()
	keys := []offerDeviceKey{
		{DeviceJID: peer, Ciphertext: []byte{1}, EncType: "pkmsg"},
		{DeviceJID: creator, Ciphertext: []byte{2}, EncType: "msg"},
	}
	call := buildCallOffer(&offerParams{CallID: "CID", To: peer, CallCreator: creator, DeviceKeys: keys})
	tags := stanzaChildTags(t, call)
	hasDest, hasEnc := false, false
	for _, tg := range tags {
		if tg == "destination" {
			hasDest = true
		}
		if tg == "enc" {
			hasEnc = true
		}
	}
	if !hasDest || hasEnc {
		t.Errorf("tags = %v, want destination present and enc absent", tags)
	}
}

func TestAcceptAndPreacceptShape(t *testing.T) {
	peer, creator := stanzaPeerJID(), stanzaCreatorJID()
	accept := buildAccept(&acceptParams{
		CallID: "CID", To: peer, CallCreator: creator,
		AudioRates: []string{"16000"}, RelayTe: make([]byte, 6), Capability: capabilityOffer,
	})
	if got := stanzaChildTags(t, accept); !stanzaEqTags(got, []string{"audio", "te", "net", "encopt", "capability"}) {
		t.Errorf("accept tags = %v", got)
	}
	pre := buildPreaccept("CID", peer, creator, "abcd1234", []string{"8000", "16000"}, false)
	if got := stanzaChildTags(t, pre); !stanzaEqTags(got, []string{"audio", "audio", "encopt", "capability"}) {
		t.Errorf("preaccept tags = %v", got)
	}
	if id, _ := stanzaAttrString(pre, "id"); id != "abcd1234" {
		t.Errorf("preaccept id = %q, want abcd1234", id)
	}
}

func TestTransportNetProtocolRule(t *testing.T) {
	peer, creator := stanzaPeerJID(), stanzaCreatorJID()
	round, t1type := "1", "1"
	t1 := buildTransport(&transportParams{
		CallID: "CID", To: peer, CallCreator: creator,
		P2PCandRound: &round, TransportMessageType: &t1type, RelayTe: make([]byte, 6),
	})
	action := stanzaContentNodes(t, t1)[0]
	if mt, _ := stanzaAttrString(action, "transport-message-type"); mt != "1" {
		t.Errorf("transport-message-type = %q, want 1", mt)
	}
	net1, ok := stanzaGetChild(t, action, "net")
	if !ok {
		t.Fatal("net child missing")
	}
	if proto, _ := stanzaAttrString(net1, "protocol"); proto != "0" {
		t.Errorf("net protocol = %q, want 0", proto)
	}

	t9type := "9"
	t9 := buildTransport(&transportParams{CallID: "CID", To: peer, CallCreator: creator, TransportMessageType: &t9type})
	net9, _ := stanzaGetChild(t, stanzaContentNodes(t, t9)[0], "net")
	if _, has := net9.Attrs["protocol"]; has {
		t.Error("type 9 net must not carry a protocol attr")
	}
}

func TestRelayLatencyEncodingAndHeartbeat(t *testing.T) {
	if got := encodeLatency(45); got != "33554477" {
		t.Errorf("encodeLatency(45) = %q, want 33554477", got)
	}
	peer, creator := stanzaPeerJID(), stanzaCreatorJID()
	rl := buildRelayLatency(&relayLatencyParams{
		CallID: "CID", To: peer, CallCreator: creator,
		LatencyMs: 45, RelayName: "gru1c02", AddressBytes: []byte{1, 2, 3, 4, 5, 6}, Devices: []types.JID{peer},
	})
	action := stanzaContentNodes(t, rl)[0]
	te, ok := stanzaGetChild(t, action, "te")
	if !ok {
		t.Fatal("te child missing")
	}
	if lat, _ := stanzaAttrString(te, "latency"); lat != "33554477" {
		t.Errorf("te latency = %q", lat)
	}
	if rn, _ := stanzaAttrString(te, "relay_name"); rn != "gru1c02" {
		t.Errorf("te relay_name = %q", rn)
	}
	if _, ok := stanzaGetChild(t, action, "destination"); !ok {
		t.Error("destination missing")
	}

	hb := buildHeartbeat("CALLID", creator, "DEADBEEF")
	if to, _ := stanzaAttrString(hb, "to"); to != "CALLID@call" {
		t.Errorf("heartbeat to = %q, want CALLID@call", to)
	}
	if id, _ := stanzaAttrString(hb, "id"); id != "DEADBEEF" {
		t.Errorf("heartbeat id = %q, want DEADBEEF", id)
	}
}

func TestTerminateWithTargets(t *testing.T) {
	peer, creator := stanzaPeerJID(), stanzaCreatorJID()
	reason := "accepted_elsewhere"
	term := buildTerminate(&terminateParams{
		CallID: "CID", To: peer, CallCreator: creator, Reason: &reason, TargetDevices: []types.JID{peer},
	})
	action := stanzaContentNodes(t, term)[0]
	if r, _ := stanzaAttrString(action, "reason"); r != "accepted_elsewhere" {
		t.Errorf("reason = %q", r)
	}
	if _, ok := stanzaGetChild(t, action, "destination"); !ok {
		t.Error("destination missing")
	}
}

func TestBuildEagerPreacceptCarriesCapabilityOffer(t *testing.T) {
	peer, creator := stanzaPeerJID(), stanzaCreatorJID()
	pre := buildEagerPreaccept("CID", peer, creator, "requestid1", false)

	if pre.Tag != "call" {
		t.Errorf("outer tag = %q, want call", pre.Tag)
	}
	if to, _ := pre.Attrs["to"].(types.JID); to != peer {
		t.Errorf("to = %v, want %v", to, peer)
	}
	if id, _ := stanzaAttrString(pre, "id"); id != "requestid1" {
		t.Errorf("id = %q, want requestid1", id)
	}

	action := stanzaContentNodes(t, pre)[0]
	if action.Tag != "preaccept" {
		t.Fatalf("action tag = %q, want preaccept", action.Tag)
	}
	if cid, _ := stanzaAttrString(action, "call-id"); cid != "CID" {
		t.Errorf("call-id = %q, want CID", cid)
	}
	if got := stanzaChildTags(t, pre); !stanzaEqTags(got, []string{"audio", "encopt", "capability"}) {
		t.Errorf("preaccept tags = %v, want [audio encopt capability]", got)
	}

	cap, ok := stanzaGetChild(t, action, "capability")
	if !ok {
		t.Fatal("capability child missing")
	}
	got, ok := cap.Content.([]byte)
	if !ok {
		t.Fatalf("capability content is not []byte: %T", cap.Content)
	}
	if !bytes.Equal(got, capabilityOffer) {
		t.Errorf("capability content = %x, want capabilityOffer %x (not capabilityPreaccept %x)", got, capabilityOffer, capabilityPreaccept)
	}
}

func TestRejectAndMuteV2(t *testing.T) {
	peer, creator := stanzaPeerJID(), stanzaCreatorJID()
	rej := buildReject("CID", peer, creator)
	action := stanzaContentNodes(t, rej)[0]
	if action.Tag != "reject" {
		t.Errorf("action tag = %q, want reject", action.Tag)
	}
	if id, _ := stanzaAttrString(action, "call-id"); id != "CID" {
		t.Errorf("call-id = %q, want CID", id)
	}

	mute := buildMuteV2("CID", peer, creator, "1")
	maction := stanzaContentNodes(t, mute)[0]
	if maction.Tag != "mute_v2" {
		t.Errorf("action tag = %q, want mute_v2", maction.Tag)
	}
	if ms, _ := stanzaAttrString(maction, "mute-state"); ms != "1" {
		t.Errorf("mute-state = %q, want 1", ms)
	}
}
