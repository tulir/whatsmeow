package whatsmeow

import (
	"context"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestSetCallHandRaisedRoutesGroupCall(t *testing.T) {
	creator := types.JID{User: "156535032389744", Server: types.HiddenUserServer, Device: 15}
	self := creator.ToNonAD()
	cli := &Client{calls: map[string]*callState{}}
	cli.putCall("CALL", &callState{
		meta:    types.BasicCallMeta{CallID: "CALL", CallCreator: creator},
		selfLID: self,
		to:      types.JID{User: "242653052539031", Server: types.HiddenUserServer},
		creator: creator,
		group: &groupCallState{snapshot: types.GroupCallUpdate{
			CallID: "CALL",
		}},
	})
	var sent waBinary.Node
	err := cli.setCallHandRaisedWithDependencies(
		t.Context(),
		"CALL",
		true,
		func() string { return "REQ" },
		func(_ context.Context, node waBinary.Node) error {
			sent = node
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := sent.AttrGetter().JID("to"); got != types.NewJID("CALL", "call") {
		t.Fatalf("to = %s, want CALL@call", got)
	}
	action := sent.GetChildren()[0]
	if action.Tag != "user_action" ||
		action.GetChildren()[0].AttrGetter().String("raise-hand-state") != "1" {
		t.Fatalf("raise-hand stanza = %#v", sent)
	}
	if !cli.getCall("CALL").raisedHands[self] {
		t.Fatal("local raised-hand state was not retained")
	}
}

func TestIncomingParticipantControlsAckAndDispatchTypedEvents(t *testing.T) {
	cli, _, captured := routerTestClient()
	participant := types.JID{User: "242653052539031", Server: types.HiddenUserServer}
	creator := types.JID{User: "156535032389744", Server: types.HiddenUserServer, Device: 15}
	cli.putCall("CALL", &callState{
		meta:         types.BasicCallMeta{CallID: "CALL", CallCreator: creator},
		creator:      creator,
		raisedHands:  make(map[types.JID]bool),
		screenShares: make(map[types.JID]types.CallScreenShare),
	})

	raise := waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"from": participant, "id": "RAISE", "t": "1"},
		Content: []waBinary.Node{{
			Tag: "user_action",
			Attrs: waBinary.Attrs{
				"call-id":      "CALL",
				"call-creator": creator,
				"action":       "raise_hand",
			},
			Content: []waBinary.Node{{
				Tag:   "raise_hand",
				Attrs: waBinary.Attrs{"raise-hand-state": "1"},
			}},
		}},
	}
	screen := waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"from": participant, "id": "SCREEN", "t": "2"},
		Content: []waBinary.Node{{
			Tag: "screen_share",
			Attrs: waBinary.Attrs{
				"call-id":           "CALL",
				"call-creator":      creator,
				"screenshare_state": "1",
				"version":           "2",
				"screen_share_id":   "1",
			},
		}},
	}
	var acks []waBinary.Node
	send := func(_ context.Context, node waBinary.Node) error {
		acks = append(acks, node)
		return nil
	}
	cli.handleCallParticipantState(t.Context(), &raise, send)
	cli.handleCallParticipantState(t.Context(), &screen, send)

	if len(acks) != 2 ||
		acks[0].AttrGetter().String("type") != "user_action" ||
		acks[1].AttrGetter().String("type") != "screen_share" {
		t.Fatalf("ACKs = %#v", acks)
	}
	handEvents := captured.filter(func(event any) bool {
		_, ok := event.(*events.CallHandRaise)
		return ok
	})
	screenEvents := captured.filter(func(event any) bool {
		_, ok := event.(*events.CallScreenShare)
		return ok
	})
	if len(handEvents) != 1 || !handEvents[0].(*events.CallHandRaise).Raised {
		t.Fatalf("hand events = %#v", handEvents)
	}
	screenEvent := screenEvents[0].(*events.CallScreenShare)
	if len(screenEvents) != 1 ||
		screenEvent.Participant != participant ||
		screenEvent.State != types.CallScreenShareStateStarted ||
		!screenEvent.HasScreenShareID {
		t.Fatalf("screen events = %#v", screenEvents)
	}
}

func TestParticipantDepartureStopsScreenShareAndClearsHand(t *testing.T) {
	cli, _, captured := routerTestClient()
	participant := types.JID{User: "242653052539031", Server: types.HiddenUserServer}
	cli.putCall("CALL", &callState{
		meta: types.BasicCallMeta{CallID: "CALL"},
		raisedHands: map[types.JID]bool{
			participant: true,
		},
		screenShares: map[types.JID]types.CallScreenShare{
			participant: {State: types.CallScreenShareStateStarted, Version: 2},
		},
	})
	cli.clearDepartedParticipantState(types.GroupCallUpdate{
		CallID: "CALL",
		Participants: []types.GroupCallParticipant{{
			JID:   participant,
			State: "left",
		}},
	})
	cs := cli.getCall("CALL")
	if cs.raisedHands[participant] {
		t.Fatal("departed participant retained raised hand")
	}
	if _, ok := cs.screenShares[participant]; ok {
		t.Fatal("departed participant retained screen share")
	}
	evts := captured.filter(func(event any) bool {
		screenEvent, ok := event.(*events.CallScreenShare)
		return ok && screenEvent.Synthetic
	})
	if len(evts) != 1 || evts[0].(*events.CallScreenShare).State != types.CallScreenShareStateStopped {
		t.Fatalf("synthetic stops = %#v", evts)
	}
}
