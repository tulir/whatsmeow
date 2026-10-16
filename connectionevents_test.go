// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types/events"
	"github.com/stretchr/testify/assert"
)

func TestHandleConnectFailureCATNilGuard(t *testing.T) {
	client := &Client{
		RefreshCAT: nil,
	}

	node := &waBinary.Node{
		Tag: "stream:error",
		Attrs: map[string]any{
			"reason": int(events.ConnectFailureCATInvalid),
		},
	}

	assert.NotPanics(t, func() {
		client.handleConnectFailure(context.Background(), node)
	})
}
