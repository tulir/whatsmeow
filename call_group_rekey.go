// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/voip"
)

func (cli *Client) onCallEncRekey(ctx context.Context, child *waBinary.Node, meta types.BasicCallMeta) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L67-L73
	// NOT VALIDATED: validated once a live rekey dispatches its decoded Call.callKey and authenticates participant media.
	rekey, err := voip.ParseGroupCallEncRekey(child)
	if err != nil {
		cli.Log.Warnf("Failed to parse group call rekey, call_id: %s: %v", meta.CallID, err)
		cli.dispatchEvent(&events.UnknownCallEvent{Node: child})
		return
	}
	enc, ok := child.GetOptionalChildByTag("enc")
	if !ok {
		cli.Log.Warnf(
			"Failed to locate encrypted group call rekey, call_id: %s, transaction_id: %d",
			meta.CallID,
			rekey.TransactionID,
		)
		cli.dispatchEvent(&events.UnknownCallEvent{Node: child})
		return
	}
	plaintext, _, err := cli.decryptDM(
		ctx,
		&enc,
		meta.From,
		rekey.EncryptionType == "pkmsg",
		meta.Timestamp,
	)
	if err != nil {
		cli.Log.Warnf(
			"Failed to decrypt group call rekey, call_id: %s, transaction_id: %d, author: %s, encryption_type: %s, ciphertext_bytes: %d: %v",
			meta.CallID,
			rekey.TransactionID,
			meta.From,
			rekey.EncryptionType,
			len(rekey.Ciphertext),
			err,
		)
		cli.dispatchEvent(&events.UnknownCallEvent{Node: child})
		return
	}
	rawKey, err := decodeCallEncRekeyPlaintext(plaintext)
	if err != nil {
		cli.Log.Warnf(
			"Failed to decode decrypted group call rekey, call_id: %s, transaction_id: %d, author: %s, plaintext_bytes: %d: %v",
			meta.CallID,
			rekey.TransactionID,
			meta.From,
			len(plaintext),
			err,
		)
		cli.dispatchEvent(&events.UnknownCallEvent{Node: child})
		return
	}
	event, err := newCallEncRekeyEvent(meta, rekey, rawKey, child)
	if err != nil {
		cli.Log.Warnf(
			"Failed to validate decrypted group call rekey, call_id: %s, transaction_id: %d, author: %s, raw_key_bytes: %d: %v",
			meta.CallID,
			rekey.TransactionID,
			meta.From,
			len(rawKey),
			err,
		)
		cli.dispatchEvent(&events.UnknownCallEvent{Node: child})
		return
	}
	if err = cli.installGroupKeyEpoch(
		event.BasicCallMeta,
		event.Rekey,
		event.RawKey,
		event.Data,
		false,
	); err != nil {
		cli.Log.Warnf(
			"Failed to install group call key epoch, call_id: %s, transaction_id: %d, author: %s, raw_key_bytes: %d: %v",
			meta.CallID,
			rekey.TransactionID,
			meta.From,
			len(rawKey),
			err,
		)
		cli.dispatchEvent(&events.UnknownCallEvent{Node: child})
	}
}

func (cli *Client) installGroupKeyEpoch(
	meta types.BasicCallMeta,
	rekey types.GroupCallEncRekey,
	rawKey []byte,
	data *waBinary.Node,
	local bool,
) error {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/d9df3eb9d96ea5260ffcd4036b6669499a1c1bc2/datasheets/voip-group-key-epoch-fanout.md#L85-L143
	if cli == nil {
		return ErrClientIsNil
	}
	if meta.CallID == "" || rekey.CallID == "" || meta.CallID != rekey.CallID {
		return fmt.Errorf("whatsmeow: install group key epoch: call ID mismatch")
	}
	if rekey.TransactionID == 0 {
		return fmt.Errorf("whatsmeow: install group key epoch: transaction ID is required")
	}
	if len(rawKey) != 32 {
		return fmt.Errorf("whatsmeow: install group key epoch: raw key is %d bytes, want 32", len(rawKey))
	}

	cli.callsLock.Lock()
	cs := cli.calls[meta.CallID]
	if cs == nil {
		cli.callsLock.Unlock()
		return fmt.Errorf("whatsmeow: install group key epoch: unknown call %s", meta.CallID)
	}
	if cs.group == nil {
		cli.callsLock.Unlock()
		return fmt.Errorf("whatsmeow: install group key epoch: call %s has no group state", meta.CallID)
	}
	if cs.hasGroupKeyEpoch {
		switch {
		case rekey.TransactionID < cs.groupKeyTransactionID:
			cli.callsLock.Unlock()
			return nil
		case rekey.TransactionID == cs.groupKeyTransactionID &&
			bytes.Equal(cs.callKey, rawKey):
			cli.callsLock.Unlock()
			return nil
		case rekey.TransactionID == cs.groupKeyTransactionID:
			cli.callsLock.Unlock()
			return fmt.Errorf(
				"whatsmeow: install group key epoch: conflicting key for transaction %d",
				rekey.TransactionID,
			)
		}
	}
	cs.callKey = bytes.Clone(rawKey)
	cs.groupKeyTransactionID = rekey.TransactionID
	cs.hasGroupKeyEpoch = true
	cli.callsLock.Unlock()

	eventRekey := rekey
	eventRekey.Ciphertext = bytes.Clone(rekey.Ciphertext)
	cli.dispatchEvent(&events.CallEncRekey{
		BasicCallMeta: meta,
		Rekey:         eventRekey,
		RawKey:        bytes.Clone(rawKey),
		Data:          data,
		Local:         local,
	})
	cli.maybeEmitMediaReady(cs)
	return nil
}

func decodeCallEncRekeyPlaintext(plaintext []byte) ([]byte, error) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L53-L65
	var message waE2E.Message
	if err := proto.Unmarshal(plaintext, &message); err != nil {
		return nil, fmt.Errorf("whatsmeow: unmarshal group call rekey message: %w", err)
	}
	rawKey := message.GetCall().GetCallKey()
	if len(rawKey) != 32 {
		return nil, fmt.Errorf("whatsmeow: group call rekey raw key is %d bytes, want 32", len(rawKey))
	}
	return rawKey, nil
}

func newCallEncRekeyEvent(
	meta types.BasicCallMeta,
	rekey *types.GroupCallEncRekey,
	rawKey []byte,
	child *waBinary.Node,
) (*events.CallEncRekey, error) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L75-L123
	if rekey == nil {
		return nil, fmt.Errorf("whatsmeow: group call rekey is nil")
	}
	if child == nil {
		return nil, fmt.Errorf("whatsmeow: group call rekey data node is nil")
	}
	if len(rawKey) != 32 {
		return nil, fmt.Errorf("whatsmeow: group call rekey raw key is %d bytes, want 32", len(rawKey))
	}
	eventRekey := *rekey
	eventRekey.Ciphertext = bytes.Clone(rekey.Ciphertext)
	return &events.CallEncRekey{
		BasicCallMeta: meta,
		Rekey:         eventRekey,
		RawKey:        bytes.Clone(rawKey),
		Data:          child,
	}, nil
}
