// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"reflect"
	"testing"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestGroupedReceiptMessageIDs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		attrs waBinary.Attrs
		want  string
	}{
		{"key", waBinary.Attrs{"key": "legacy"}, "legacy"},
		{"message_id", waBinary.Attrs{"message_id": "ACAC70EC"}, "ACAC70EC"},
		{"both", waBinary.Attrs{"key": "legacy", "message_id": "other"}, "legacy"},
		{"empty key", waBinary.Attrs{"key": "", "message_id": "other"}, "other"},
		{"empty alias", waBinary.Attrs{"key": "legacy", "message_id": ""}, "legacy"},
		{"missing", waBinary.Attrs{}, ""},
		{"empty key only", waBinary.Attrs{"key": ""}, ""},
		{"empty message_id", waBinary.Attrs{"message_id": ""}, ""},
		{"both empty", waBinary.Attrs{"key": "", "message_id": ""}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ownID := types.NewJID("15550000001", types.DefaultUserServer)
			chat := types.NewJID("120363000000001", types.GroupServer)
			sender := types.NewJID("10001", types.HiddenUserServer)
			recipient := types.NewJID("15550000002", types.DefaultUserServer)
			cli := NewClient(&store.Device{ID: &ownID, LID: types.NewJID("10000", types.HiddenUserServer)}, nil)
			cli.SynchronousAck = true
			var got []*events.Receipt
			cli.AddEventHandler(func(evt any) {
				receipt, ok := evt.(*events.Receipt)
				if !ok {
					t.Errorf("unexpected event %T", evt)
					return
				}
				got = append(got, receipt)
			})
			node := waBinary.Node{
				Tag:   "receipt",
				Attrs: waBinary.Attrs{"from": chat, "id": "SAGR-not-a-message-id", "t": "1793344500", "type": "read", "recipient": recipient},
				Content: []waBinary.Node{
					{Tag: "participants", Attrs: tc.attrs, Content: []waBinary.Node{
						{Tag: "unexpected"},
						{Tag: "user", Attrs: waBinary.Attrs{"jid": sender, "t": "invalid"}},
						{Tag: "user", Attrs: waBinary.Attrs{"jid": sender, "t": "1793344591", "type": "delivery"}},
						{Tag: "user", Attrs: waBinary.Attrs{"jid": recipient, "t": "1793344592"}},
					}},
					{Tag: "participants", Attrs: waBinary.Attrs{"key": "next-message"}, Content: []waBinary.Node{
						{Tag: "user", Attrs: waBinary.Attrs{"jid": sender, "t": "1793344593"}},
					}},
				},
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			cli.handleReceipt(ctx, &node)
			want := []*events.Receipt{}
			if tc.want != "" {
				for i, jid := range []types.JID{sender, recipient} {
					want = append(want, &events.Receipt{
						MessageSource: types.MessageSource{Chat: chat, Sender: jid, IsGroup: true},
						MessageIDs:    []types.MessageID{tc.want}, Timestamp: time.Unix(1793344591+int64(i), 0),
						Type: types.ReceiptTypeRead, MessageSender: recipient,
					})
				}
			}
			want = append(want, &events.Receipt{
				MessageSource: types.MessageSource{Chat: chat, Sender: sender, IsGroup: true},
				MessageIDs:    []types.MessageID{"next-message"}, Timestamp: time.Unix(1793344593, 0),
				Type: types.ReceiptTypeRead, MessageSender: recipient,
			})
			if !reflect.DeepEqual(got, want) {
				for _, receipt := range got {
					t.Logf("got: %+v", *receipt)
				}
				for _, receipt := range want {
					t.Logf("want: %+v", *receipt)
				}
				t.Fatalf("receipts differ")
			}
		})
	}
}

func TestGroupedReceiptHandlerFailure(t *testing.T) {
	cli := NewClient(&store.Device{}, nil)
	calls := 0
	cli.AddEventHandlerWithSuccessStatus(func(any) bool { calls++; return calls != 1 })
	participants := waBinary.Node{Tag: "participants", Attrs: waBinary.Attrs{"message_id": "message"}, Content: []waBinary.Node{
		{Tag: "user", Attrs: waBinary.Attrs{"jid": types.NewJID("10001", types.HiddenUserServer), "t": "1793344591"}},
		{Tag: "user", Attrs: waBinary.Attrs{"jid": types.NewJID("10002", types.HiddenUserServer), "t": "1793344592"}},
	}}
	if cancelled := cli.handleGroupedReceipt(events.Receipt{}, &participants); !cancelled || calls != 2 {
		t.Fatalf("cancelled = %v, calls = %d; want true, 2", cancelled, calls)
	}
}
