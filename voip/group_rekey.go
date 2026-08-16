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

// GroupEncRekeyParams contains the wire fields for one direct shared group-key
// epoch delivery.
type GroupEncRekeyParams struct {
	CallID        string
	To            types.JID
	CallCreator   types.JID
	TransactionID uint32
	RequestID     string
	DeviceKey     DeviceKey
}

// BuildGroupEncRekey builds one direct keygen-v2 group epoch stanza.
func BuildGroupEncRekey(params GroupEncRekeyParams) (waBinary.Node, error) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/d9df3eb9d96ea5260ffcd4036b6669499a1c1bc2/datasheets/voip-group-key-epoch-fanout.md#L20-L72
	if params.CallID == "" {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build group rekey: call ID is required")
	}
	if params.To.IsEmpty() {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build group rekey: recipient is required")
	}
	if params.CallCreator.IsEmpty() {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build group rekey: call creator is required")
	}
	if params.TransactionID == 0 {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build group rekey: transaction ID is required")
	}
	if params.RequestID == "" {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build group rekey: request ID is required")
	}
	if params.DeviceKey.DeviceJID != params.To {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build group rekey: encrypted device does not match recipient")
	}
	if len(params.DeviceKey.Ciphertext) == 0 {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build group rekey: ciphertext is required")
	}
	if params.DeviceKey.EncType != "msg" && params.DeviceKey.EncType != "pkmsg" {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build group rekey: unsupported encryption type %q", params.DeviceKey.EncType)
	}
	action := waBinary.Node{
		Tag: "enc_rekey",
		Attrs: waBinary.Attrs{
			"call-id":        params.CallID,
			"call-creator":   params.CallCreator,
			"transaction-id": strconv.FormatUint(uint64(params.TransactionID), 10),
		},
		Content: []waBinary.Node{
			{Tag: "encopt", Attrs: waBinary.Attrs{"keygen": "2"}},
			{
				Tag: "enc",
				Attrs: waBinary.Attrs{
					"v": "2", "type": params.DeviceKey.EncType, "count": "0",
				},
				Content: bytes.Clone(params.DeviceKey.Ciphertext),
			},
		},
	}
	return callWrap(params.To, &params.RequestID, action), nil
}

// ParseGroupCallEncRekey parses an enc_rekey call-signaling node.
func ParseGroupCallEncRekey(node *waBinary.Node) (*types.GroupCallEncRekey, error) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L39-L65
	if node == nil {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey: nil node")
	}
	if node.Tag != "enc_rekey" {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey: unexpected tag %q", node.Tag)
	}

	attrs := node.AttrGetter()
	rekey := &types.GroupCallEncRekey{
		CallID:      attrs.String("call-id"),
		CallCreator: attrs.JID("call-creator"),
	}
	var err error
	rekey.TransactionID, err = requiredUint32Attr(attrs, "transaction-id")
	if err != nil {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey transaction ID: %w", err)
	}
	if err = attrs.Error(); err != nil {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey attributes: %w", err)
	}

	children := node.GetChildren()
	var encopt, enc *waBinary.Node
	for i := range children {
		switch children[i].Tag {
		case "encopt":
			if encopt != nil {
				return nil, fmt.Errorf("whatsmeow: parse group call rekey: duplicate encopt")
			}
			encopt = &children[i]
		case "enc":
			if enc != nil {
				return nil, fmt.Errorf("whatsmeow: parse group call rekey: duplicate enc")
			}
			enc = &children[i]
		}
	}
	if encopt == nil {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey: missing encopt")
	}
	if enc == nil {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey: missing enc")
	}

	encoptAttrs := encopt.AttrGetter()
	rekey.KeyGeneration, err = requiredUint32Attr(encoptAttrs, "keygen")
	if err != nil {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey key generation: %w", err)
	}
	if err = encoptAttrs.Error(); err != nil {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey encopt attributes: %w", err)
	}
	if rekey.KeyGeneration != 2 {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey: unsupported key generation %d", rekey.KeyGeneration)
	}

	encAttrs := enc.AttrGetter()
	rekey.EncryptionType = encAttrs.String("type")
	rekey.EncryptionVersion, err = requiredUint32Attr(encAttrs, "v")
	if err != nil {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey encryption version: %w", err)
	}
	if err = encAttrs.Error(); err != nil {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey enc attributes: %w", err)
	}
	if rekey.EncryptionType != "msg" && rekey.EncryptionType != "pkmsg" {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey: unsupported encryption type %q", rekey.EncryptionType)
	}
	if rekey.EncryptionVersion != 2 {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey: unsupported encryption version %d", rekey.EncryptionVersion)
	}

	ciphertext, ok := enc.Content.([]byte)
	if !ok {
		return nil, fmt.Errorf("whatsmeow: parse group call rekey: ciphertext is %T, not bytes", enc.Content)
	}
	rekey.Ciphertext = bytes.Clone(ciphertext)
	return rekey, nil
}
