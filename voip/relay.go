// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package voip

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

type voipSettings struct {
	UseMlowCodecV1 bool
	FrameMs        int
	TargetBitrate  int
}

func parseVoipSettings(content []byte) (*voipSettings, error) {
	if len(bytes.TrimSpace(content)) == 0 {
		return &voipSettings{UseMlowCodecV1: true}, nil
	}
	var doc struct {
		Encode struct {
			UseMlowCodecV1 string `json:"use_mlow_codec_v1"`
			FrameMs        string `json:"frame_ms"`
		} `json:"encode"`
		RC struct {
			TargetBitrate string `json:"target_bitrate"`
		} `json:"rc"`
	}
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("whatsmeow: parse voip_settings: %w", err)
	}
	return &voipSettings{
		UseMlowCodecV1: doc.Encode.UseMlowCodecV1 != "false",
		FrameMs:        atoiOrZero(doc.Encode.FrameMs),
		TargetBitrate:  atoiOrZero(doc.RC.TargetBitrate),
	}, nil
}

func atoiOrZero(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func selectCallCodec(vs *voipSettings) types.CallCodec {
	if vs == nil || vs.UseMlowCodecV1 {
		return types.CallCodecMLow
	}
	return types.CallCodecOpus
}

type relayAddress struct {
	ipv4 string
	port uint16
}

type relayEndpoint struct {
	relayID     uint32
	relayName   string
	tokenID     uint32
	authTokenID uint32
	isFNA       bool
	addresses   []relayAddress
}

type relayData struct {
	relayKeyASCII []byte
	relayTokens   [][]byte
	endpoints     []relayEndpoint
}

func nodeBytes(n *waBinary.Node) []byte {
	switch c := n.Content.(type) {
	case []byte:
		return c
	case string:
		return []byte(c)
	}
	return nil
}

func childByTag(n *waBinary.Node, tag string) *waBinary.Node {
	kids := n.GetChildren()
	for i := range kids {
		if kids[i].Tag == tag {
			return &kids[i]
		}
	}
	return nil
}

func findRelay(n *waBinary.Node) *waBinary.Node {
	if n == nil {
		return nil
	}
	if n.Tag == "relay" {
		return n
	}
	kids := n.GetChildren()
	for i := range kids {
		if r := findRelay(&kids[i]); r != nil {
			return r
		}
	}
	return nil
}

func findChild(n *waBinary.Node, tag string) *waBinary.Node {
	if n == nil {
		return nil
	}
	if n.Tag == tag {
		return n
	}
	kids := n.GetChildren()
	for i := range kids {
		if r := findChild(&kids[i], tag); r != nil {
			return r
		}
	}
	return nil
}

func decodeLatency(enc string) uint32 {
	v, err := strconv.ParseUint(enc, 10, 32)
	if err != nil || v < 0x0200_0000 {
		return 0
	}
	return uint32(v) - 0x0200_0000
}

func attrUint(n *waBinary.Node, key string) uint32 {
	v, _ := strconv.ParseUint(n.AttrGetter().String(key), 10, 32)
	return uint32(v)
}

const maxRelayTokens = 64

func parseIndexedTokens(node *waBinary.Node, tag string) [][]byte {
	var tokens [][]byte
	kids := node.GetChildren()
	for i := range kids {
		c := &kids[i]
		if c.Tag != tag {
			continue
		}
		b := nodeBytes(c)
		if b == nil {
			continue
		}
		id := len(tokens)
		if s := c.AttrGetter().String("id"); s != "" {
			if n, err := strconv.Atoi(s); err == nil {
				id = n
			}
		}
		if id < 0 || id >= maxRelayTokens {
			continue
		}
		for len(tokens) <= id {
			tokens = append(tokens, nil)
		}
		tokens[id] = b
	}
	return tokens
}

func parseRelayData(node *waBinary.Node) *relayData {
	rd := &relayData{}
	if key := childByTag(node, "key"); key != nil {
		rd.relayKeyASCII = nodeBytes(key)
	}
	rd.relayTokens = parseIndexedTokens(node, "token")

	kids := node.GetChildren()
	for i := range kids {
		te2 := &kids[i]
		if te2.Tag != "te2" {
			continue
		}
		ab := nodeBytes(te2)
		if len(ab) != 6 {
			continue
		}
		ep := relayEndpoint{
			relayID:     attrUint(te2, "relay_id"),
			relayName:   te2.AttrGetter().String("relay_name"),
			tokenID:     attrUint(te2, "token_id"),
			authTokenID: attrUint(te2, "auth_token_id"),
			isFNA:       te2.AttrGetter().String("is_fna") == "1",
			addresses: []relayAddress{{
				ipv4: fmt.Sprintf("%d.%d.%d.%d", ab[0], ab[1], ab[2], ab[3]),
				port: binary.BigEndian.Uint16(ab[4:6]),
			}},
		}
		rd.endpoints = append(rd.endpoints, ep)
	}
	return rd
}

func getMediaRelayEndpoint(rd *relayData, direction types.CallDirection) *relayEndpoint {
	if direction == types.CallDirectionIncoming {
		for i := range rd.endpoints {
			if e := &rd.endpoints[i]; e.isFNA {
				return e
			}
		}
	}
	for i := range rd.endpoints {
		if e := &rd.endpoints[i]; !e.isFNA && e.authTokenID != 0 {
			return e
		}
	}
	for i := range rd.endpoints {
		if e := &rd.endpoints[i]; !e.isFNA {
			return e
		}
	}
	if len(rd.endpoints) > 0 {
		return &rd.endpoints[0]
	}
	return nil
}

func relayToken(tokens [][]byte, id uint32) []byte {
	if int(id) >= len(tokens) {
		return nil
	}
	return tokens[id]
}

func parseElectedRelay(node *waBinary.Node, direction types.CallDirection) *types.RelayEndpoint {
	relay := findRelay(node)
	if relay == nil {
		return nil
	}
	rd := parseRelayData(relay)
	ep := getMediaRelayEndpoint(rd, direction)
	if ep == nil {
		return nil
	}
	out := &types.RelayEndpoint{
		RelayID:     ep.relayID,
		TokenID:     ep.tokenID,
		AuthTokenID: ep.authTokenID,
		RelayName:   ep.relayName,
		IsFNA:       ep.isFNA,
		Key:         rd.relayKeyASCII,
		Token:       relayToken(rd.relayTokens, ep.tokenID),
		AuthToken:   relayToken(rd.relayTokens, ep.authTokenID),
	}
	if len(ep.addresses) > 0 {
		out.IPv4 = ep.addresses[0].ipv4
		out.Port = ep.addresses[0].port
	}
	return out
}

// NodeBytes returns a node's byte or string content as bytes.
func NodeBytes(node *waBinary.Node) []byte {
	return nodeBytes(node)
}

// FindChild recursively finds the first child with tag.
func FindChild(node *waBinary.Node, tag string) *waBinary.Node {
	return findChild(node, tag)
}

// FindRelay recursively finds a relay node.
func FindRelay(node *waBinary.Node) *waBinary.Node {
	return findRelay(node)
}

// DecodeLatency decodes WhatsApp's relay latency representation.
func DecodeLatency(encoded string) uint32 {
	return decodeLatency(encoded)
}

// ParseCodec selects the advertised audio codec from a voip_settings payload.
func ParseCodec(content []byte) (types.CallCodec, error) {
	settings, err := parseVoipSettings(content)
	if err != nil {
		return types.CallCodecMLow, err
	}
	return selectCallCodec(settings), nil
}

// ParseRelay resolves the media endpoint appropriate for the call direction.
func ParseRelay(node *waBinary.Node, direction types.CallDirection) *types.RelayEndpoint {
	return parseElectedRelay(node, direction)
}
