// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/voip"
)

type groupEpochFanoutDependencies struct {
	random    io.Reader
	encrypt   func(context.Context, []types.JID, []byte) ([]voip.DeviceKey, error)
	requestID func() string
	send      func(context.Context, waBinary.Node) error
	install   func(types.BasicCallMeta, types.GroupCallEncRekey, []byte, *waBinary.Node, bool) error
}

func groupRekeyRecipients(update types.GroupCallUpdate, self types.JID) []types.JID {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/d9df3eb9d96ea5260ffcd4036b6669499a1c1bc2/datasheets/voip-group-key-epoch-fanout.md#L145-L162
	seen := make(map[types.JID]struct{})
	var recipients []types.JID
	for _, participant := range update.Participants {
		if participant.State != "connected" {
			continue
		}
		for _, device := range participant.Devices {
			if !device.HasPID || device.JID.IsEmpty() || device.JID == self {
				continue
			}
			if _, exists := seen[device.JID]; exists {
				continue
			}
			seen[device.JID] = struct{}{}
			recipients = append(recipients, device.JID)
		}
	}
	return recipients
}

func runRequestedGroupEpochFanout(
	ctx context.Context,
	meta types.BasicCallMeta,
	update types.GroupCallUpdate,
	self types.JID,
	deps groupEpochFanoutDependencies,
) error {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/d9df3eb9d96ea5260ffcd4036b6669499a1c1bc2/datasheets/voip-group-key-epoch-fanout.md#L99-L162
	if !update.RekeyRequested {
		return fmt.Errorf("whatsmeow: distribute group key epoch: server did not request rekey")
	}
	if meta.CallID == "" || update.CallID == "" || meta.CallID != update.CallID {
		return fmt.Errorf("whatsmeow: distribute group key epoch: call ID mismatch")
	}
	if update.CallCreator.IsEmpty() || update.TransactionID == 0 || self.IsEmpty() {
		return fmt.Errorf("whatsmeow: distribute group key epoch: incomplete call identity")
	}
	if deps.random == nil || deps.encrypt == nil || deps.requestID == nil ||
		deps.send == nil || deps.install == nil {
		return fmt.Errorf("whatsmeow: distribute group key epoch: incomplete dependencies")
	}
	recipients := groupRekeyRecipients(update, self)
	if len(recipients) == 0 {
		return fmt.Errorf("whatsmeow: distribute group key epoch: no remote connected devices")
	}
	rawKey := make([]byte, 32)
	if _, err := io.ReadFull(deps.random, rawKey); err != nil {
		return fmt.Errorf("whatsmeow: generate group key epoch: %w", err)
	}
	deviceKeys, err := deps.encrypt(ctx, recipients, rawKey)
	if err != nil {
		return fmt.Errorf("whatsmeow: encrypt group key epoch: %w", err)
	}
	if len(deviceKeys) != len(recipients) {
		return fmt.Errorf(
			"whatsmeow: encrypt group key epoch: got %d device keys for %d recipients",
			len(deviceKeys),
			len(recipients),
		)
	}
	nodes := make([]waBinary.Node, len(recipients))
	for i, recipient := range recipients {
		nodes[i], err = voip.BuildGroupEncRekey(voip.GroupEncRekeyParams{
			CallID:        update.CallID,
			To:            recipient,
			CallCreator:   update.CallCreator,
			TransactionID: update.TransactionID,
			RequestID:     deps.requestID(),
			DeviceKey:     deviceKeys[i],
		})
		if err != nil {
			return err
		}
	}
	for _, node := range nodes {
		if err = deps.send(ctx, node); err != nil {
			return fmt.Errorf("whatsmeow: send group key epoch: %w", err)
		}
	}
	localMeta := meta
	localMeta.From = self
	localMeta.CallCreator = update.CallCreator
	rekey := types.GroupCallEncRekey{
		CallID:            update.CallID,
		CallCreator:       update.CallCreator,
		TransactionID:     update.TransactionID,
		KeyGeneration:     2,
		EncryptionVersion: 2,
	}
	var data *waBinary.Node
	if actions := nodes[0].GetChildren(); len(actions) == 1 {
		action := actions[0]
		data = &action
	}
	if err = deps.install(localMeta, rekey, rawKey, data, true); err != nil {
		return fmt.Errorf("whatsmeow: install local group key epoch: %w", err)
	}
	return nil
}

func (cli *Client) distributeRequestedGroupEpoch(
	ctx context.Context,
	meta types.BasicCallMeta,
	update types.GroupCallUpdate,
) error {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/d9df3eb9d96ea5260ffcd4036b6669499a1c1bc2/datasheets/voip-group-key-epoch-fanout.md#L77-L162
	if cli == nil {
		return ErrClientIsNil
	}
	cli.callsLock.Lock()
	cs := cli.calls[update.CallID]
	if cs == nil {
		cli.callsLock.Unlock()
		return fmt.Errorf("whatsmeow: distribute group key epoch: unknown call %s", update.CallID)
	}
	self := cs.selfLID
	cli.callsLock.Unlock()
	return runRequestedGroupEpochFanout(ctx, meta, update, self, groupEpochFanoutDependencies{
		random: rand.Reader,
		encrypt: func(ctx context.Context, recipients []types.JID, rawKey []byte) ([]voip.DeviceKey, error) {
			keys, _, err := cli.encryptCallKeyForDevices(ctx, recipients, rawKey)
			return keys, err
		},
		requestID: cli.generateRequestID,
		send:      cli.sendNode,
		install:   cli.installGroupKeyEpoch,
	})
}
