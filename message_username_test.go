package whatsmeow

import (
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestMessageSenderUsername(t *testing.T) {
	ownID := types.NewJID("15550000001", types.DefaultUserServer)
	cli := NewClient(&store.Device{ID: &ownID}, nil)
	for _, tc := range []struct {
		name     string
		from     types.JID
		username string
		meta     []waBinary.Node
	}{
		{name: "LID sender", from: types.NewJID("200000000000001", types.HiddenUserServer), username: "Alice.Example_24"},
		{name: "phone sender", from: types.NewJID("15550000002", types.DefaultUserServer), username: "alice.example"},
		{name: "absent username", from: types.NewJID("200000000000001", types.HiddenUserServer)},
		{name: "bot keeps root username", from: types.NewJID("example", types.BotServer), username: "example.bot", meta: []waBinary.Node{{Tag: "meta", Attrs: waBinary.Attrs{"target_chat_jid": ownID, "username": "different.meta.value"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node := waBinary.Node{
				Tag:     "message",
				Attrs:   waBinary.Attrs{"from": tc.from, "id": "test-message", "t": "1792430649", "notify": "Display Name", "type": "text"},
				Content: tc.meta,
			}
			if tc.username != "" {
				node.Attrs["username"] = tc.username
			}
			info, err := cli.parseMessageInfo(&node)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.SenderUsername; got != tc.username {
				t.Errorf("SenderUsername = %q, want %q", got, tc.username)
			}
			if info.PushName != "Display Name" || info.Sender != tc.from {
				t.Errorf("sender or display name changed: %+v", info)
			}
		})
	}
}

func TestGroupedReceiptSenderUsername(t *testing.T) {
	ownID := types.NewJID("15550000001", types.DefaultUserServer)
	cli := NewClient(&store.Device{ID: &ownID}, nil)
	root := waBinary.Node{
		Tag:   "receipt",
		Attrs: waBinary.Attrs{"from": types.NewJID("120363000000001", types.GroupServer), "t": "1792430649", "username": "unrelated.root"},
		Content: []waBinary.Node{{Tag: "participants", Attrs: waBinary.Attrs{"key": "test-message"}, Content: []waBinary.Node{
			{Tag: "user", Attrs: waBinary.Attrs{"jid": types.NewJID("200000000000001", types.HiddenUserServer), "t": "1792430649", "username": "alice.example"}},
			{Tag: "user", Attrs: waBinary.Attrs{"jid": types.NewJID("200000000000002", types.HiddenUserServer), "t": "1792430650"}},
		}}},
	}
	partial, participants, err := cli.parseReceipt(&root)
	if err != nil {
		t.Fatal(err)
	}
	var received []*events.Receipt
	cli.AddEventHandler(func(evt any) {
		if receipt, ok := evt.(*events.Receipt); ok {
			received = append(received, receipt)
		}
	})
	for _, participant := range participants {
		cli.handleGroupedReceipt(*partial, &participant)
	}
	if len(received) != 2 {
		t.Fatalf("got %d receipts, want 2", len(received))
	}
	for i, want := range []string{"alice.example", ""} {
		if got := received[i].SenderUsername; got != want {
			t.Errorf("receipt %d SenderUsername = %q, want %q", i, got, want)
		}
		if received[i].Sender.User != []string{"200000000000001", "200000000000002"}[i] || received[i].MessageIDs[0] != "test-message" {
			t.Errorf("receipt identity changed: %+v", received[i])
		}
	}
}
