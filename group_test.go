// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mau.fi/whatsmeow/binary/waBinary"
	"go.mau.fi/whatsmeow/types"
)

func TestParseGroupChangeMultipleParticipantChanges(t *testing.T) {
	cli := &Client{}

	// Construct a notification node with both <add> and <promote> children
	addParticipantJID := types.NewJID("1234567890", types.HiddenUserServer)
	addPNJID := types.NewJID("1999999999", types.PhoneNumberServer)

	promoteParticipantJID := types.NewJID("9876543210", types.HiddenUserServer)
	promotePNJID := types.NewJID("1888888888", types.PhoneNumberServer)

	node := waBinary.Node{
		Tag: "notification",
		Attrs: waBinary.Attrs{
			"from": types.NewJID("group", types.GroupServer),
			"t":    types.UnixTime(time.Now()),
		},
		Content: []any{
			waBinary.Node{
				Tag: "add",
				Attrs: waBinary.Attrs{
					"v_id": "v1",
				},
				Content: []any{
					waBinary.Node{
						Tag: "participant",
						Attrs: waBinary.Attrs{
							"jid":          addParticipantJID,
							"phone_number": addPNJID,
						},
					},
				},
			},
			waBinary.Node{
				Tag: "promote",
				Attrs: waBinary.Attrs{
					"v_id": "v1",
				},
				Content: []any{
					waBinary.Node{
						Tag: "participant",
						Attrs: waBinary.Attrs{
							"jid":          promoteParticipantJID,
							"phone_number": promotePNJID,
						},
					},
				},
			},
		},
	}

	_, lidPairs, err := cli.parseGroupChange(&node)
	assert.NoError(t, err)
	assert.Len(t, lidPairs, 2)

	expectedPairs := map[types.JID]types.JID{
		addParticipantJID:     addPNJID,
		promoteParticipantJID: promotePNJID,
	}

	for _, pair := range lidPairs {
		expectedPN, ok := expectedPairs[pair.LID]
		assert.True(t, ok)
		assert.Equal(t, expectedPN, pair.PN)
	}
}
