package whatsmeow

import (
	"context"
	"sync"
	"testing"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestRunCallLinkServiceRequestBuildsAndParsesCreate(t *testing.T) {
	request := func(_ context.Context, node waBinary.Node, requestID string) (*waBinary.Node, error) {
		if requestID != "REQ" || node.AttrGetter().String("id") != requestID ||
			node.AttrGetter().OptionalJIDOrEmpty("to") != types.NewJID("", "call") {
			t.Fatalf("request = %#v, id = %q", node, requestID)
		}
		return &waBinary.Node{
			Tag:   "ack",
			Attrs: waBinary.Attrs{"class": "call", "type": "link_create", "id": requestID},
			Content: []waBinary.Node{{
				Tag:   "link_create",
				Attrs: waBinary.Attrs{"media": "video", "token": "TOKEN"},
			}},
		}, nil
	}
	link, err := createCallLinkWithDependencies(
		t.Context(),
		types.CallLinkMediaVideo,
		func() string { return "REQ" },
		request,
	)
	if err != nil {
		t.Fatal(err)
	}
	if link.Token != "TOKEN" || link.Media != types.CallLinkMediaVideo {
		t.Fatalf("link = %#v", link)
	}
}

func TestNewCallLinkStateDistinguishesWaitingFromAdmitted(t *testing.T) {
	self := types.JID{User: "242653052539031", Server: types.HiddenUserServer, Device: 2}
	creator := types.JID{User: "156535032389744", Server: types.HiddenUserServer, Device: 15}
	waiting := &types.CallLinkJoin{
		Token:              "TOKEN",
		Media:              types.CallLinkMediaVideo,
		CallID:             "WAITING",
		CallCreator:        creator,
		WaitingRoomEnabled: true,
		InWaitingRoom:      true,
	}
	waitingState := newCallLinkState(self, waiting)
	if waitingState.group != nil || waitingState.connected ||
		waitingState.linkToken != "TOKEN" ||
		waitingState.signalingTarget() != types.NewJID("WAITING", "call") {
		t.Fatalf("waiting state = %#v", waitingState)
	}
	if waitingState.waitingRoom == nil || !waitingState.waitingRoom.Enabled {
		t.Fatalf("waiting room = %#v", waitingState.waitingRoom)
	}

	group := &types.GroupCallUpdate{
		CallID:        "ADMITTED",
		CallCreator:   creator,
		TransactionID: 3,
		Media:         "video",
	}
	admitted := *waiting
	admitted.CallID = "ADMITTED"
	admitted.InWaitingRoom = false
	admitted.WaitingRoomEnabled = false
	admitted.Group = group
	admittedState := newCallLinkState(self, &admitted)
	if admittedState.group == nil || !admittedState.connected ||
		admittedState.group.snapshot.TransactionID != 3 {
		t.Fatalf("admitted state = %#v", admittedState)
	}
}

func TestWaitingRoomHeartbeatStartsImmediatelyAndStops(t *testing.T) {
	creator := types.JID{User: "156535032389744", Server: types.HiddenUserServer, Device: 15}
	cs := &callState{
		meta:      types.BasicCallMeta{CallID: "CALL", CallCreator: creator},
		creator:   creator,
		linkToken: "TOKEN",
	}
	var mu sync.Mutex
	var sent int
	send := func(_ context.Context, node waBinary.Node) error {
		if node.GetChildren()[0].Tag != "heartbeat" {
			t.Fatalf("heartbeat = %#v", node)
		}
		mu.Lock()
		sent++
		mu.Unlock()
		return nil
	}
	cancel, err := startWaitingRoomHeartbeat(
		t.Context(),
		cs,
		time.Millisecond,
		func() string { return "REQ" },
		send,
	)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	cancel()
	mu.Lock()
	atCancel := sent
	mu.Unlock()
	if atCancel < 2 {
		t.Fatalf("heartbeats before cancel = %d, want immediate and scheduled", atCancel)
	}
	time.Sleep(3 * time.Millisecond)
	mu.Lock()
	afterCancel := sent
	mu.Unlock()
	if afterCancel != atCancel {
		t.Fatalf("heartbeats continued after cancel: %d -> %d", atCancel, afterCancel)
	}
}

func TestWaitingRoomUpdateAckStateAndEvent(t *testing.T) {
	cli, _, captured := routerTestClient()
	creator := types.JID{User: "156535032389744", Server: types.HiddenUserServer, Device: 15}
	user := types.JID{User: "242653052539031", Server: types.HiddenUserServer}
	cli.putCall("CALL", &callState{
		meta:      types.BasicCallMeta{CallID: "CALL", CallCreator: creator},
		creator:   creator,
		linkToken: "TOKEN",
	})
	node := waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"from": types.NewJID("CALL", "call"), "id": "UPDATE"},
		Content: []waBinary.Node{{
			Tag: "waiting_room_update",
			Attrs: waBinary.Attrs{
				"call-id":      "CALL",
				"call-creator": creator,
			},
			Content: []waBinary.Node{{
				Tag: "waiting_room",
				Attrs: waBinary.Attrs{
					"call-id":        "CALL",
					"call-creator":   creator,
					"link-token":     "TOKEN",
					"media":          "video",
					"enabled":        "1",
					"is_admin":       "1",
					"transaction-id": "4",
				},
				Content: []waBinary.Node{{
					Tag: "user",
					Attrs: waBinary.Attrs{
						"jid":   user,
						"state": "waiting_room_joined",
					},
				}},
			}},
		}},
	}
	var ack waBinary.Node
	cli.handleCallWaitingRoomUpdate(t.Context(), &node, func(_ context.Context, node waBinary.Node) error {
		ack = node
		return nil
	})
	if ack.AttrGetter().String("type") != "waiting_room_update" {
		t.Fatalf("ACK = %#v", ack)
	}
	state := cli.getCall("CALL").waitingRoom
	if state == nil || state.TransactionID != 4 || len(state.Users) != 1 || !state.IsAdmin {
		t.Fatalf("waiting room state = %#v", state)
	}
	got := captured.filter(func(event any) bool {
		_, ok := event.(*events.CallWaitingRoomUpdate)
		return ok
	})
	if len(got) != 1 {
		t.Fatalf("waiting room events = %#v", got)
	}
}
