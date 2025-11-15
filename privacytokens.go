// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"time"

	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

// equivalentPrivacyTokenJIDs returns the JIDs that should share the same privacy token.
// It always includes the provided JID and when available, the mapped PN<->LID counterpart.
func (cli *Client) equivalentPrivacyTokenJIDs(ctx context.Context, jid types.JID) ([]types.JID, error) {
	base := jid.ToNonAD()
	result := []types.JID{base}
	if cli == nil || cli.Store == nil || cli.Store.LIDs == nil {
		return result, nil
	}

	var mapped types.JID
	var err error
	switch base.Server {
	case types.DefaultUserServer:
		mapped, err = cli.Store.LIDs.GetLIDForPN(ctx, base)
	case types.HiddenUserServer:
		mapped, err = cli.Store.LIDs.GetPNForLID(ctx, base)
	default:
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	if !mapped.IsEmpty() {
		mapped = mapped.ToNonAD()
		if mapped != base {
			result = append(result, mapped)
		}
	}
	return result, nil
}

// getPrivacyToken fetches a privacy token for the provided JID or any mapped
// PN/LID counterpart.
func (cli *Client) getPrivacyToken(ctx context.Context, jid types.JID) (*store.PrivacyToken, error) {
	targets, err := cli.equivalentPrivacyTokenJIDs(ctx, jid)
	if err != nil {
		return nil, err
	}
	for _, target := range targets {
		pt, err := cli.Store.PrivacyTokens.GetPrivacyToken(ctx, target)
		if err != nil {
			return nil, err
		}
		if pt != nil {
			return pt, nil
		}
	}
	return nil, nil
}

// buildPrivacyTokenEntries expands the provided sender JID into all equivalent
// PN/LID variants so the same token value is stored against each representation.
func (cli *Client) buildPrivacyTokenEntries(ctx context.Context, sender types.JID, token []byte, ts time.Time) ([]store.PrivacyToken, error) {
	targets, err := cli.equivalentPrivacyTokenJIDs(ctx, sender)
	if err != nil {
		return nil, err
	}
	entries := make([]store.PrivacyToken, 0, len(targets))
	for _, target := range targets {
		entries = append(entries, store.PrivacyToken{
			User:      target,
			Token:     token,
			Timestamp: ts,
		})
	}
	return entries, nil
}
