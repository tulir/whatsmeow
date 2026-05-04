// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"

	"go.mau.fi/whatsmeow/types"
)

func shouldSendCsToken(jid types.JID) bool {
	jid = jid.ToNonAD()
	return (jid.Server == types.DefaultUserServer || jid.Server == types.HiddenUserServer) &&
		jid.User != types.PSAJID.User &&
		!jid.IsBot()
}

// derives a cstoken for the given JID using HMAC-SHA256(nctSalt, recipientLID).
func (cli *Client) generateCsToken(ctx context.Context, jid types.JID) []byte {
	if !shouldSendCsToken(jid) {
		return nil
	}
	if cli.Store == nil || cli.Store.NCTSalt == nil {
		return nil
	}
	salt, err := cli.Store.NCTSalt.GetNCTSalt(ctx)
	if err != nil {
		cli.Log.Debugf("Failed to load NCT salt for cstoken: %v", err)
		return nil
	}
	if len(salt) == 0 {
		return nil
	}
	var recipientLID types.JID
	switch jid.Server {
	case types.HiddenUserServer:
		recipientLID = jid.ToNonAD()
	case types.DefaultUserServer:
		if cli.Store == nil || cli.Store.LIDs == nil {
			return nil
		}
		pn := jid.ToNonAD()
		lid, err := cli.Store.LIDs.GetLIDForPN(ctx, pn)
		if err != nil {
			cli.Log.Debugf("Failed to resolve LID for cstoken JID %s: %v", pn, err)
			return nil
		}
		if lid.IsEmpty() {
			return nil
		}
		recipientLID = lid.ToNonAD()
	default:
		return nil
	}

	if recipientLID.Server != types.HiddenUserServer {
		return nil
	}

	h := hmac.New(sha256.New, salt)
	h.Write([]byte(recipientLID.String()))
	return h.Sum(nil)
}

func (cli *Client) storeNCTSalt(ctx context.Context, salt []byte) error {
	if cli.Store == nil || cli.Store.NCTSalt == nil {
		return nil
	}
	if len(salt) == 0 {
		return nil
	}
	return cli.Store.NCTSalt.PutNCTSalt(ctx, salt)
}

func (cli *Client) clearNCTSalt(ctx context.Context) error {
	if cli.Store == nil || cli.Store.NCTSalt == nil {
		return nil
	}
	return cli.Store.NCTSalt.DeleteNCTSalt(ctx)
}
