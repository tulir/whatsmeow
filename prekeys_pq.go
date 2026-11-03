// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"encoding/binary"
	"fmt"

	"go.mau.fi/libsignal/state/record"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

const (
	WantedKyberPreKeyCount = 50
	MinKyberPreKeyCount    = 5
)

func kyberPreKeyToNode(tag string, key *record.KyberPreKey) (waBinary.Node, error) {
	pub, err := key.PublicKey()
	if err != nil {
		return waBinary.Node{}, err
	}
	var keyID [4]byte
	binary.BigEndian.PutUint32(keyID[:], key.ID)
	return waBinary.Node{
		Tag: tag,
		Content: []waBinary.Node{
			{Tag: "id", Content: keyID[1:]},
			{Tag: "value", Content: pub},
			{Tag: "signature", Content: key.Signature},
		},
	}, nil
}

func (cli *Client) getServerKyberPreKeyCount(ctx context.Context) (int, error) {
	resp, err := cli.sendIQ(ctx, infoQuery{
		Namespace: "encrypt",
		Type:      "get",
		To:        types.ServerJID,
		Content:   []waBinary.Node{{Tag: "count"}},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get prekey count on server: %w", err)
	}
	pqCount, ok := resp.GetOptionalChildByTag("pq_count")
	if !ok {
		return 0, nil
	}
	ag := pqCount.AttrGetter()
	return ag.Int("value"), ag.Error()
}

func (cli *Client) uploadKyberPreKeys(ctx context.Context) error {
	if !cli.EnablePQPreKeys || cli.Store.KyberPreKeys == nil {
		return nil
	}
	cli.uploadPreKeysLock.Lock()
	defer cli.uploadPreKeysLock.Unlock()
	identity := cli.Store.GetIdentityKeyPair()
	lastResort, err := cli.Store.KyberPreKeys.GetOrGenKyberLastResortKey(ctx, identity)
	if err != nil {
		return fmt.Errorf("failed to get kyber last resort key: %w", err)
	}
	preKeys, err := cli.Store.KyberPreKeys.GetOrGenKyberPreKeys(ctx, WantedKyberPreKeyCount, identity)
	if err != nil {
		return fmt.Errorf("failed to get kyber prekeys to upload: %w", err)
	}
	list := make([]waBinary.Node, len(preKeys))
	for i, key := range preKeys {
		list[i], err = kyberPreKeyToNode("key", key)
		if err != nil {
			return fmt.Errorf("failed to encode kyber prekey %d: %w", key.ID, err)
		}
	}
	lastResortNode, err := kyberPreKeyToNode("pq_last_resort_key", lastResort)
	if err != nil {
		return fmt.Errorf("failed to encode kyber last resort key: %w", err)
	}
	cli.Log.Infof("Uploading %d new kyber prekeys to server", len(preKeys))
	_, err = cli.sendIQ(ctx, infoQuery{
		Namespace: "encrypt",
		Type:      "set",
		To:        types.ServerJID,
		Content: []waBinary.Node{
			{Tag: "pq_list", Content: list},
			lastResortNode,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to upload kyber prekeys: %w", err)
	}
	if len(preKeys) > 0 {
		err = cli.Store.KyberPreKeys.MarkKyberPreKeysAsUploaded(ctx, preKeys[len(preKeys)-1].ID)
		if err != nil {
			return fmt.Errorf("failed to mark kyber prekeys as uploaded: %w", err)
		}
	}
	return nil
}

func (cli *Client) kyberLastResortKeyNode(ctx context.Context) *waBinary.Node {
	if !cli.EnablePQPreKeys || cli.Store.KyberPreKeys == nil {
		return nil
	}
	key, err := cli.Store.KyberPreKeys.GetOrGenKyberLastResortKey(ctx, cli.Store.GetIdentityKeyPair())
	if err != nil {
		cli.Log.Warnf("Failed to get kyber last resort key for retry receipt: %v", err)
		return nil
	}
	node, err := kyberPreKeyToNode("pq", key)
	if err != nil {
		cli.Log.Warnf("Failed to encode kyber last resort key for retry receipt: %v", err)
		return nil
	}
	return &node
}
