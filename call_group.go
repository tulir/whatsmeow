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
)

// onCallGroupUpdate parses and dispatches one group-call snapshot.
func (cli *Client) onCallGroupUpdate(ctx context.Context, child *waBinary.Node, meta types.BasicCallMeta) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7f2d29c19f410b3127067973322a093325bbea1e/datasheets/voip-group-update-ingest.md#L101-L105
	// TODO
	// agent suggestion: parse with voip.ParseGroupUpdate; dispatch a value snapshot plus
	// the raw child, or warn, dispatch UnknownCallEvent, and leave deferred ACK intact.
	// human input:
}
