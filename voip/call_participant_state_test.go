package voip

import (
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func TestBuildRaiseHandMatchesCapturedShape(t *testing.T) {
	creator := types.JID{User: "156535032389744", Server: types.HiddenUserServer, Device: 15}
	node := BuildRaiseHand(
		"006C151B7929963D0FE0DB9E0774817D",
		types.NewJID("006C151B7929963D0FE0DB9E0774817D", "call"),
		creator,
		"REQ",
		true,
	)
	if got := node.AttrGetter().JID("to"); got != types.NewJID("006C151B7929963D0FE0DB9E0774817D", "call") {
		t.Fatalf("to = %s", got)
	}
	if got := node.AttrGetter().String("id"); got != "REQ" {
		t.Fatalf("id = %q", got)
	}
	action := onlyCallAction(t, node)
	if action.Tag != "user_action" || action.AttrGetter().String("action") != "raise_hand" {
		t.Fatalf("action = %#v", action)
	}
	children := action.GetChildren()
	if len(children) != 1 || children[0].Tag != "raise_hand" ||
		children[0].AttrGetter().String("raise-hand-state") != "1" {
		t.Fatalf("raise-hand child = %#v", children)
	}
}

func TestParseRaiseHandAcceptsRaiseAndLower(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{value: "1", want: true},
		{value: "0", want: false},
	} {
		node := waBinary.Node{
			Tag: "user_action",
			Attrs: waBinary.Attrs{
				"call-id":      "CALL",
				"call-creator": types.JID{User: "1", Server: types.HiddenUserServer},
				"action":       "raise_hand",
			},
			Content: []waBinary.Node{{
				Tag:   "raise_hand",
				Attrs: waBinary.Attrs{"raise-hand-state": tc.value},
			}},
		}
		got, err := ParseRaiseHand(&node)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("state %s parsed as %t", tc.value, got)
		}
	}
}

func TestBuildAndParseScreenSharePreservesOptionalID(t *testing.T) {
	creator := types.JID{User: "156535032389744", Server: types.HiddenUserServer, Device: 15}
	screenShareID := uint32(1)
	node := BuildScreenShare(
		"006C151B7929963D0FE0DB9E0774817D",
		types.NewJID("006C151B7929963D0FE0DB9E0774817D", "call"),
		creator,
		"REQ",
		types.CallScreenShareStateStarted,
		&screenShareID,
	)
	action := onlyCallAction(t, node)
	attrs := action.AttrGetter()
	if action.Tag != "screen_share" ||
		attrs.String("screenshare_state") != "1" ||
		attrs.String("version") != "2" ||
		attrs.String("screen_share_id") != "1" {
		t.Fatalf("screen-share action = %#v", action)
	}
	parsed, err := ParseScreenShare(&action)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.State != types.CallScreenShareStateStarted ||
		!parsed.HasScreenShareID || parsed.ScreenShareID != 1 || parsed.Version != 2 {
		t.Fatalf("parsed screen share = %#v", parsed)
	}

	stop := BuildScreenShare(
		"CALL",
		types.JID{User: "2", Server: types.HiddenUserServer, Device: 3},
		creator,
		"REQ2",
		types.CallScreenShareStateStopped,
		nil,
	)
	stopAction := onlyCallAction(t, stop)
	if _, ok := stopAction.Attrs["screen_share_id"]; ok {
		t.Fatalf("direct stop unexpectedly contains screen_share_id: %#v", stopAction)
	}
}

func TestBuildCallControlAckUsesChildType(t *testing.T) {
	participant := types.JID{User: "242653052539031", Server: types.HiddenUserServer}
	node := waBinary.Node{
		Tag: "call",
		Attrs: waBinary.Attrs{
			"from":        participant,
			"id":          "1785026985-218",
			"participant": participant,
		},
		Content: []waBinary.Node{{Tag: "screen_share"}},
	}
	ack, ok := BuildCallControlAck(&node, "screen_share")
	if !ok {
		t.Fatal("failed to build ACK")
	}
	attrs := ack.AttrGetter()
	if attrs.JID("to") != participant || attrs.String("id") != "1785026985-218" ||
		attrs.String("class") != "call" || attrs.String("type") != "screen_share" {
		t.Fatalf("ACK = %#v", ack)
	}
}

func onlyCallAction(t *testing.T, node waBinary.Node) waBinary.Node {
	t.Helper()
	children := node.GetChildren()
	if node.Tag != "call" || len(children) != 1 {
		t.Fatalf("call envelope = %#v", node)
	}
	return children[0]
}
