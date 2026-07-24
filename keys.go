// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/voip"
)

func callKeyPlaintext(callKey []byte) ([]byte, error) {
	return proto.Marshal(&waE2E.Message{Call: &waE2E.Call{CallKey: callKey}})
}

// encryptCallKeyForDevices encrypts callKey to each of the peer's devices.
//
// NOT VALIDATED: exercised by the live call E2E; unit-covered only for the
// plaintext wrap (callKeyPlaintext).
func (cli *Client) encryptCallKeyForDevices(ctx context.Context, devices []types.JID, callKey []byte) (keys []voip.DeviceKey, deviceIdentity []byte, err error) {
	pt, err := callKeyPlaintext(callKey)
	if err != nil {
		return nil, nil, err
	}
	keys = make([]voip.DeviceKey, 0, len(devices))
	needIdentity := false
	for _, dev := range devices {
		enc, ni, encErr := cli.encryptMessageForDevice(ctx, pt, dev, nil, nil, nil)
		if encErr != nil {
			bundles := cli.fetchPreKeysNoError(ctx, []types.JID{dev})
			enc, ni, encErr = cli.encryptMessageForDevice(ctx, pt, dev, bundles[dev], nil, nil)
			if encErr != nil {
				return nil, nil, fmt.Errorf("whatsmeow: encrypt call key for %s: %w", dev, encErr)
			}
		}
		ct, ok := enc.Content.([]byte)
		if !ok {
			return nil, nil, fmt.Errorf("whatsmeow: enc node for %s has no ciphertext", dev)
		}
		needIdentity = needIdentity || ni
		keys = append(keys, voip.DeviceKey{DeviceJID: dev, Ciphertext: ct, EncType: enc.AttrGetter().String("type")})
	}
	if needIdentity {
		deviceIdentity, err = proto.Marshal(cli.Store.Account)
		if err != nil {
			return nil, nil, fmt.Errorf("whatsmeow: marshal device identity: %w", err)
		}
	}
	return keys, deviceIdentity, nil
}

// decryptIncomingCallKey decrypts the callKey carried in an incoming call offer.
//
// NOT VALIDATED: exercised by the live call E2E; unit-covered only for the
// plaintext wrap (callKeyPlaintext).
func (cli *Client) decryptIncomingCallKey(ctx context.Context, offer *events.CallOffer) ([]byte, error) {
	if offer == nil || offer.Data == nil {
		return nil, errors.New("whatsmeow: call offer has no data node")
	}
	children := offer.Data.GetChildren()
	var enc *waBinary.Node
	for i := range children {
		if children[i].Tag == "enc" {
			enc = &children[i]
			break
		}
	}
	if enc == nil {
		return nil, errors.New("whatsmeow: call offer has no enc node")
	}
	isPreKey := enc.AttrGetter().String("type") == "pkmsg"
	pt, _, err := cli.decryptDM(ctx, enc, offer.From, isPreKey, offer.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("whatsmeow: decrypt call key: %w", err)
	}
	var msg waE2E.Message
	if err = proto.Unmarshal(pt, &msg); err != nil {
		return nil, fmt.Errorf("whatsmeow: unmarshal call key message: %w", err)
	}
	callKey := msg.GetCall().GetCallKey()
	if len(callKey) == 0 {
		return nil, errors.New("whatsmeow: call offer carried no callKey")
	}
	return callKey, nil
}
