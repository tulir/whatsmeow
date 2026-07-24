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

const voipSettingsSample = `{"encode":{"complexity":"5","frame_ms":"60","min_bitrate":"4200","use_mlow_codec_v1":"true"},"rc":{"dtx":"1","target_bitrate":"24000"}}`

func TestVoipSettingsParse(t *testing.T) {
	vs, err := parseVoipSettings([]byte(voipSettingsSample))
	if err != nil {
		t.Fatalf("parseVoipSettings: %v", err)
	}
	if !vs.UseMlowCodecV1 {
		t.Error("UseMlowCodecV1 = false, want true (sample sets it true)")
	}
	if vs.FrameMs != 60 {
		t.Errorf("FrameMs = %d, want 60", vs.FrameMs)
	}
	if vs.TargetBitrate != 24000 {
		t.Errorf("TargetBitrate = %d, want 24000", vs.TargetBitrate)
	}
	if got := selectCallCodec(vs); got != types.CallCodecMLow {
		t.Errorf("selectCallCodec = %v, want mlow", got)
	}
}

func TestVoipSettingsOpus(t *testing.T) {
	vs, err := parseVoipSettings([]byte(`{"encode":{"use_mlow_codec_v1":"false","frame_ms":"60"}}`))
	if err != nil {
		t.Fatalf("parseVoipSettings: %v", err)
	}
	if vs.UseMlowCodecV1 {
		t.Error("UseMlowCodecV1 = true, want false")
	}
	if got := selectCallCodec(vs); got != types.CallCodecOpus {
		t.Errorf("selectCallCodec = %v, want opus", got)
	}
}

func TestVoipSettingsEmptyDefaultsToMlow(t *testing.T) {
	vs, err := parseVoipSettings(nil)
	if err != nil {
		t.Fatalf("parseVoipSettings: %v", err)
	}
	if !vs.UseMlowCodecV1 {
		t.Error("UseMlowCodecV1 = false, want true (empty blob defaults to mlow)")
	}
	if got := selectCallCodec(vs); got != types.CallCodecMLow {
		t.Errorf("selectCallCodec = %v, want mlow", got)
	}
}

func TestVoipSettingsMalformedJSONErrors(t *testing.T) {
	if _, err := parseVoipSettings([]byte(`{not json`)); err == nil {
		t.Error("parseVoipSettings(malformed) = nil error, want error")
	}
}

func TestVoipSelectCallCodecNilDefaultsToMlow(t *testing.T) {
	if got := selectCallCodec(nil); got != types.CallCodecMLow {
		t.Errorf("selectCallCodec(nil) = %v, want mlow", got)
	}
}

func syntheticRelayNode() waBinary.Node {
	return waBinary.Node{
		Tag: "relay",
		Content: []waBinary.Node{
			{Tag: "key", Content: []byte("relay-integrity-key")},
			{Tag: "token", Attrs: waBinary.Attrs{"id": "0"}, Content: []byte("token-zero")},
			{Tag: "token", Attrs: waBinary.Attrs{"id": "1"}, Content: []byte("token-one")},
			{
				Tag: "te2",
				Attrs: waBinary.Attrs{
					"relay_id":      "5",
					"relay_name":    "gru1c02",
					"token_id":      "0",
					"auth_token_id": "1",
					"is_fna":        "0",
				},
				Content: []byte{10, 0, 0, 1, 0x1f, 0x90}, // 10.0.0.1:8080
			},
		},
	}
}

func TestRelayParseElectsEndpoint(t *testing.T) {
	node := syntheticRelayNode()
	ep := parseElectedRelay(&node, types.CallDirectionOutgoing)
	if ep == nil {
		t.Fatal("parseElectedRelay = nil, want a resolved endpoint")
	}
	if ep.RelayID != 5 {
		t.Errorf("RelayID = %d, want 5", ep.RelayID)
	}
	if ep.RelayName != "gru1c02" {
		t.Errorf("RelayName = %q, want gru1c02", ep.RelayName)
	}
	if ep.TokenID != 0 {
		t.Errorf("TokenID = %d, want 0", ep.TokenID)
	}
	if ep.AuthTokenID != 1 {
		t.Errorf("AuthTokenID = %d, want 1", ep.AuthTokenID)
	}
	if ep.IsFNA {
		t.Error("IsFNA = true, want false")
	}
	if ep.IPv4 != "10.0.0.1" {
		t.Errorf("IPv4 = %q, want 10.0.0.1", ep.IPv4)
	}
	if ep.Port != 8080 {
		t.Errorf("Port = %d, want 8080", ep.Port)
	}
	if !bytes.Equal(ep.Key, []byte("relay-integrity-key")) {
		t.Errorf("Key = %q, want relay-integrity-key", ep.Key)
	}
	if !bytes.Equal(ep.Token, []byte("token-zero")) {
		t.Errorf("Token = %q, want token-zero", ep.Token)
	}
	if !bytes.Equal(ep.AuthToken, []byte("token-one")) {
		t.Errorf("AuthToken = %q, want token-one", ep.AuthToken)
	}
}

func TestRelayParseResolvesElectedPeerDevice(t *testing.T) {
	primary := types.NewJID("242653052539031", types.HiddenUserServer)
	companion := primary
	companion.Device = 7
	node := syntheticRelayNode()
	node.Attrs = waBinary.Attrs{"peer_pid": "2", "self_pid": "1"}
	node.Content = append(node.GetChildren(),
		waBinary.Node{Tag: "participant", Attrs: waBinary.Attrs{"pid": "0", "jid": primary}},
		waBinary.Node{Tag: "participant", Attrs: waBinary.Attrs{"pid": "2", "jid": companion}},
	)

	if peer := parseRelayPeer(&node); peer != companion {
		t.Fatalf("parseRelayPeer = %s, want %s", peer, companion)
	}
}

func TestRelayParseElectsFNAForIncomingCalls(t *testing.T) {
	node := syntheticRelayNode()
	children := node.GetChildren()
	children = append(children, waBinary.Node{
		Tag: "te2",
		Attrs: waBinary.Attrs{
			"relay_id": "8", "relay_name": "gru1c02-fna", "token_id": "0",
			"auth_token_id": "0", "is_fna": "1",
		},
		Content: []byte{10, 0, 0, 2, 0x1f, 0x91},
	})
	node.Content = children

	incoming := parseElectedRelay(&node, types.CallDirectionIncoming)
	if incoming == nil || !incoming.IsFNA || incoming.RelayID != 8 {
		t.Fatalf("incoming endpoint = %+v, want FNA relay 8", incoming)
	}
	outgoing := parseElectedRelay(&node, types.CallDirectionOutgoing)
	if outgoing == nil || outgoing.IsFNA || outgoing.RelayID != 5 {
		t.Fatalf("outgoing endpoint = %+v, want non-FNA relay 5", outgoing)
	}
}

func TestRelayParseFindsNestedRelay(t *testing.T) {
	relay := syntheticRelayNode()
	offer := waBinary.Node{Tag: "offer", Content: []waBinary.Node{relay}}
	ep := parseElectedRelay(&offer, types.CallDirectionOutgoing)
	if ep == nil {
		t.Fatal("parseElectedRelay = nil, want a resolved endpoint")
	}
	if ep.IPv4 != "10.0.0.1" || ep.Port != 8080 {
		t.Errorf("endpoint = %+v, want 10.0.0.1:8080", ep)
	}
}

func TestRelayParseNoRelayReturnsNil(t *testing.T) {
	node := waBinary.Node{Tag: "offer"}
	if ep := parseElectedRelay(&node, types.CallDirectionOutgoing); ep != nil {
		t.Errorf("parseElectedRelay = %+v, want nil", ep)
	}
}

func TestRelayParseTokenOutOfBoundsIsNil(t *testing.T) {
	node := waBinary.Node{
		Tag: "relay",
		Content: []waBinary.Node{
			{Tag: "key", Content: []byte("k")},
			{
				Tag: "te2",
				Attrs: waBinary.Attrs{
					"relay_id": "1", "token_id": "9", "auth_token_id": "9", "is_fna": "0",
				},
				Content: []byte{127, 0, 0, 1, 0x00, 0x50},
			},
		},
	}
	ep := parseElectedRelay(&node, types.CallDirectionOutgoing)
	if ep == nil {
		t.Fatal("parseElectedRelay = nil, want a resolved endpoint")
	}
	if ep.Token != nil {
		t.Errorf("Token = %q, want nil (index 9 never populated)", ep.Token)
	}
	if ep.AuthToken != nil {
		t.Errorf("AuthToken = %q, want nil (index 9 never populated)", ep.AuthToken)
	}
}

func TestRelayParseNegativeTokenIDIsIgnored(t *testing.T) {
	node := waBinary.Node{
		Tag: "relay",
		Content: []waBinary.Node{
			{Tag: "key", Content: []byte("k")},
			{Tag: "token", Attrs: waBinary.Attrs{"id": "-1"}, Content: []byte("evil")},
			{Tag: "token", Attrs: waBinary.Attrs{"id": "0"}, Content: []byte("token-zero")},
			{
				Tag: "te2",
				Attrs: waBinary.Attrs{
					"relay_id": "1", "token_id": "0", "auth_token_id": "0", "is_fna": "0",
				},
				Content: []byte{127, 0, 0, 1, 0x00, 0x50},
			},
		},
	}
	ep := parseElectedRelay(&node, types.CallDirectionOutgoing)
	if ep == nil {
		t.Fatal("parseElectedRelay = nil, want a resolved endpoint")
	}
	if !bytes.Equal(ep.Token, []byte("token-zero")) {
		t.Errorf("Token = %q, want token-zero (negative-id token must be dropped, not corrupt index 0)", ep.Token)
	}
}
