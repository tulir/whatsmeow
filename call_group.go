// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/voip"
)

// onCallGroupUpdate parses and dispatches one group-call snapshot.
func (cli *Client) onCallGroupUpdate(ctx context.Context, child *waBinary.Node, meta types.BasicCallMeta) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/48c2391ce9f7dcc2b3f223f72f1b5f0c627ad943/datasheets/voip-group-update-ingest.md#L105-L148
	update, err := voip.ParseGroupUpdate(child)
	if err != nil {
		cli.Log.Warnf("Failed to parse group call update, call_id: %s: %v", meta.CallID, err)
		cli.dispatchEvent(&events.UnknownCallEvent{Node: child})
		return
	}
	if !cli.applyGroupUpdate(*update) {
		cli.Log.Debugf(
			"Ignoring unapplied group call update, call_id: %s, transaction_id: %d",
			update.CallID,
			update.TransactionID,
		)
		return
	}
	meta.GroupJID = update.GroupJID
	cli.dispatchEvent(&events.CallGroupUpdate{
		BasicCallMeta: meta,
		Update:        *update,
		Data:          child,
	})
	// Source of truth: https://github.com/purpshell/meowcaller/blob/d9df3eb9d96ea5260ffcd4036b6669499a1c1bc2/datasheets/voip-group-key-epoch-fanout.md#L99-L162
	if update.RekeyRequested {
		if err = cli.distributeRequestedGroupEpoch(ctx, meta, *update); err != nil {
			cli.Log.Warnf(
				"Failed to distribute requested group call key epoch, call_id: %s, transaction_id: %d: %v",
				update.CallID,
				update.TransactionID,
				err,
			)
		}
	}
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L183-L189
	if cs := cli.getCall(update.CallID); cs != nil {
		cli.maybeEmitMediaReady(cs)
	}
}
