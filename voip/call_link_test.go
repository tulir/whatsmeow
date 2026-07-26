package voip

import (
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func callLinkCreatorJID() types.JID {
	return types.JID{
		User:   "156535032389744",
		Server: types.HiddenUserServer,
		Device: 15,
	}
}

func callLinkUserJID() types.JID {
	return types.JID{User: "242653052539031", Server: types.HiddenUserServer}
}

func callLinkAction(t *testing.T, node waBinary.Node) waBinary.Node {
	t.Helper()
	children := node.GetChildren()
	if len(children) != 1 {
		t.Fatalf("outer child count = %d, want 1", len(children))
	}
	return children[0]
}

func TestBuildCallLinkCreateMatchesCapture(t *testing.T) {
	node, err := BuildCallLinkCreate(types.CallLinkMediaVideo, "65149.29747-11")
	if err != nil {
		t.Fatal(err)
	}
	if node.Tag != "call" {
		t.Fatalf("outer tag = %q, want call", node.Tag)
	}
	if got := node.AttrGetter().JID("to"); got != types.NewJID("", "call") {
		t.Errorf("to = %s, want call service JID", got)
	}
	if got := node.AttrGetter().String("id"); got != "65149.29747-11" {
		t.Errorf("id = %q, want captured request ID", got)
	}
	action := callLinkAction(t, node)
	if action.Tag != "link_create" {
		t.Fatalf("action tag = %q, want link_create", action.Tag)
	}
	if got := action.AttrGetter().String("media"); got != "video" {
		t.Errorf("media = %q, want video", got)
	}
}

func TestBuildCallLinkQueryMatchesCapture(t *testing.T) {
	node, err := BuildCallLinkQuery(
		"PNtK9cGQ1OrmsKqnWPo0wN",
		types.CallLinkMediaVideo,
		"65149.29747-12",
	)
	if err != nil {
		t.Fatal(err)
	}
	action := callLinkAction(t, node)
	if action.Tag != "link_query" {
		t.Fatalf("action tag = %q, want link_query", action.Tag)
	}
	attrs := action.AttrGetter()
	if got := attrs.String("token"); got != "PNtK9cGQ1OrmsKqnWPo0wN" {
		t.Errorf("token = %q", got)
	}
	if got := attrs.String("media"); got != "video" {
		t.Errorf("media = %q, want video", got)
	}
}

func TestBuildCallLinkJoinVideoMatchesCapturedChildOrder(t *testing.T) {
	node, err := BuildCallLinkJoin(
		"PNtK9cGQ1OrmsKqnWPo0wN",
		types.CallLinkMediaVideo,
		"65149.29747-13",
	)
	if err != nil {
		t.Fatal(err)
	}
	action := callLinkAction(t, node)
	if action.Tag != "link_join" {
		t.Fatalf("action tag = %q, want link_join", action.Tag)
	}
	children := action.GetChildren()
	if len(children) != 4 {
		t.Fatalf("child count = %d, want 4", len(children))
	}
	wantTags := []string{"audio", "video", "net", "capability"}
	for i, want := range wantTags {
		if children[i].Tag != want {
			t.Errorf("child %d tag = %q, want %q", i, children[i].Tag, want)
		}
	}
	if got := children[0].AttrGetter().String("enc"); got != "opus" {
		t.Errorf("audio enc = %q, want opus", got)
	}
	if got := children[0].AttrGetter().String("rate"); got != "16000" {
		t.Errorf("audio rate = %q, want 16000", got)
	}
	if got := children[1].AttrGetter().String("dec"); got != "H264" {
		t.Errorf("video dec = %q, want H264", got)
	}
	if got := children[1].AttrGetter().String("device_orientation"); got != "0" {
		t.Errorf("device_orientation = %q, want 0", got)
	}
	if got := children[2].AttrGetter().String("medium"); got != "2" {
		t.Errorf("net medium = %q, want 2", got)
	}
	if got := nodeBytes(&children[3]); string(got) != string(capabilityOffer) {
		t.Errorf("capability = %x, want %x", got, capabilityOffer)
	}
}

func TestBuildCallLinkJoinAudioOmitsVideo(t *testing.T) {
	node, err := BuildCallLinkJoin("TOKEN", types.CallLinkMediaAudio, "REQ")
	if err != nil {
		t.Fatal(err)
	}
	action := callLinkAction(t, node)
	children := action.GetChildren()
	wantTags := []string{"audio", "net", "capability"}
	if len(children) != len(wantTags) {
		t.Fatalf("child count = %d, want %d", len(children), len(wantTags))
	}
	for i, want := range wantTags {
		if children[i].Tag != want {
			t.Errorf("child %d tag = %q, want %q", i, children[i].Tag, want)
		}
	}
}

func TestCallLinkBuildersRejectInvalidInputs(t *testing.T) {
	if _, err := BuildCallLinkCreate(types.CallLinkMedia("screen"), "REQ"); err == nil {
		t.Error("invalid media accepted")
	}
	if _, err := BuildCallLinkQuery("", types.CallLinkMediaVideo, "REQ"); err == nil {
		t.Error("empty token accepted")
	}
	if _, err := BuildCallLinkJoin("TOKEN", types.CallLinkMediaVideo, ""); err == nil {
		t.Error("empty request ID accepted")
	}
}

func TestBuildWaitingRoomControlsMatchCapturedRoutes(t *testing.T) {
	creator := callLinkCreatorJID()
	user := callLinkUserJID()
	tests := []struct {
		name string
		node waBinary.Node
		tag  string
	}{
		{
			name: "toggle",
			node: BuildWaitingRoomToggle("00AF16E8F3C700B81311EE5A19EA7531", creator, true, "REQ1"),
			tag:  "waiting_room_toggle",
		},
		{
			name: "heartbeat",
			node: BuildWaitingRoomHeartbeat("00AF16E8F3C700B81311EE5A19EA7531", creator, "REQ2"),
			tag:  "heartbeat",
		},
		{
			name: "admit",
			node: BuildWaitingRoomAdmit("00AF16E8F3C700B81311EE5A19EA7531", creator, user, "REQ3"),
			tag:  "waiting_room_admit",
		},
		{
			name: "deny",
			node: BuildWaitingRoomDeny("00AF16E8F3C700B81311EE5A19EA7531", creator, user, "REQ4"),
			tag:  "waiting_room_deny",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.node.AttrGetter().JID("to").String(); got != "00AF16E8F3C700B81311EE5A19EA7531@call" {
				t.Errorf("to = %q, want current call route", got)
			}
			action := callLinkAction(t, tc.node)
			if action.Tag != tc.tag {
				t.Fatalf("action tag = %q, want %q", action.Tag, tc.tag)
			}
			if got := action.AttrGetter().String("call-id"); got != "00AF16E8F3C700B81311EE5A19EA7531" {
				t.Errorf("call-id = %q", got)
			}
		})
	}
	toggle := callLinkAction(t, tests[0].node)
	if got := toggle.AttrGetter().String("enabled"); got != "1" {
		t.Errorf("enabled = %q, want 1", got)
	}
	heartbeat := callLinkAction(t, tests[1].node)
	if got := heartbeat.AttrGetter().String("type"); got != "waiting_room" {
		t.Errorf("heartbeat type = %q, want waiting_room", got)
	}
	for _, index := range []int{2, 3} {
		action := callLinkAction(t, tests[index].node)
		users := action.GetChildren()
		if len(users) != 1 || users[0].Tag != "user" ||
			users[0].AttrGetter().JID("jid") != user {
			t.Errorf("%s user child = %#v", tests[index].name, users)
		}
	}
}

func TestParseCallLinkCreateAndQueryAck(t *testing.T) {
	createAck := waBinary.Node{
		Tag:   "ack",
		Attrs: waBinary.Attrs{"class": "call", "type": "link_create", "id": "REQ"},
		Content: []waBinary.Node{{
			Tag: "link_create",
			Attrs: waBinary.Attrs{
				"media": "video",
				"token": "PNtK9cGQ1OrmsKqnWPo0wN",
			},
		}},
	}
	link, err := ParseCallLinkCreateAck(&createAck)
	if err != nil {
		t.Fatal(err)
	}
	if link.Token != "PNtK9cGQ1OrmsKqnWPo0wN" || link.Media != types.CallLinkMediaVideo {
		t.Fatalf("parsed link = %#v", link)
	}

	queryAck := waBinary.Node{
		Tag:   "ack",
		Attrs: waBinary.Attrs{"class": "call", "type": "link_query", "id": "REQ"},
		Content: []waBinary.Node{{
			Tag: "link_query",
			Attrs: waBinary.Attrs{
				"link_creator":    types.JID{User: "156535032389744", Server: types.HiddenUserServer},
				"link_creator_pn": types.JID{User: "96179377559", Server: types.DefaultUserServer},
				"media":           "video",
				"token":           "PNtK9cGQ1OrmsKqnWPo0wN",
			},
			Content: []waBinary.Node{{
				Tag:   "waiting_room",
				Attrs: waBinary.Attrs{"is_admin": "1", "enabled": "0"},
			}},
		}},
	}
	preview, err := ParseCallLinkQueryAck(&queryAck)
	if err != nil {
		t.Fatal(err)
	}
	if preview.WaitingRoomEnabled || !preview.IsAdmin {
		t.Fatalf("preview waiting room = %#v", preview)
	}
	if preview.Creator.String() != "156535032389744@lid" {
		t.Errorf("creator = %s", preview.Creator)
	}
}

func TestParseCallLinkJoinAckDistinguishesWaitingFromAdmitted(t *testing.T) {
	creator := callLinkCreatorJID()
	waitingAck := waBinary.Node{
		Tag:   "ack",
		Attrs: waBinary.Attrs{"class": "call", "type": "link_join", "id": "REQ"},
		Content: []waBinary.Node{{
			Tag: "waiting_room",
			Attrs: waBinary.Attrs{
				"link-token":   "PNtK9cGQ1OrmsKqnWPo0wN",
				"media":        "video",
				"call-id":      "00AF16E8F3C700B81311EE5A19EA7531",
				"call-creator": creator,
				"enabled":      "1",
			},
		}},
	}
	waiting, err := ParseCallLinkJoinAck(&waitingAck)
	if err != nil {
		t.Fatal(err)
	}
	if !waiting.InWaitingRoom || waiting.Group != nil || !waiting.WaitingRoomEnabled {
		t.Fatalf("waiting join = %#v", waiting)
	}

	groupInfo := waBinary.Node{
		Tag: "group_info",
		Attrs: waBinary.Attrs{
			"call-id":         "0067CBB85B113E16600396BD25346DB3",
			"call-creator":    creator,
			"transaction-id":  "4",
			"media":           "video",
			"connected-limit": "32",
			"link-token":      "PNtK9cGQ1OrmsKqnWPo0wN",
		},
		Content: []waBinary.Node{{
			Tag:   "user",
			Attrs: waBinary.Attrs{"jid": callLinkUserJID(), "state": "connected"},
			Content: []waBinary.Node{{
				Tag:   "device",
				Attrs: waBinary.Attrs{"jid": types.JID{User: "242653052539031", Server: types.HiddenUserServer, Device: 2}, "pid": "1"},
			}},
		}},
	}
	admittedAck := waBinary.Node{
		Tag:   "ack",
		Attrs: waBinary.Attrs{"class": "call", "type": "link_join", "id": "REQ"},
		Content: []waBinary.Node{
			groupInfo,
			{
				Tag: "waiting_room",
				Attrs: waBinary.Attrs{
					"link-token":   "PNtK9cGQ1OrmsKqnWPo0wN",
					"media":        "video",
					"call-id":      "0067CBB85B113E16600396BD25346DB3",
					"call-creator": creator,
					"enabled":      "0",
				},
			},
		},
	}
	admitted, err := ParseCallLinkJoinAck(&admittedAck)
	if err != nil {
		t.Fatal(err)
	}
	if admitted.InWaitingRoom || admitted.Group == nil {
		t.Fatalf("admitted join = %#v", admitted)
	}
	if admitted.Group.TransactionID != 4 || admitted.Group.CallID != admitted.CallID {
		t.Fatalf("group identity = %#v", admitted.Group)
	}
}

func TestParseWaitingRoomUpdate(t *testing.T) {
	creator := callLinkCreatorJID()
	node := waBinary.Node{
		Tag: "waiting_room_update",
		Attrs: waBinary.Attrs{
			"call-id":      "00AF16E8F3C700B81311EE5A19EA7531",
			"call-creator": creator,
		},
		Content: []waBinary.Node{{
			Tag: "waiting_room",
			Attrs: waBinary.Attrs{
				"is_admin":       "1",
				"link-token":     "PNtK9cGQ1OrmsKqnWPo0wN",
				"media":          "video",
				"call-id":        "00AF16E8F3C700B81311EE5A19EA7531",
				"call-creator":   creator,
				"enabled":        "1",
				"transaction-id": "2",
			},
			Content: []waBinary.Node{{
				Tag: "user",
				Attrs: waBinary.Attrs{
					"jid":     callLinkUserJID(),
					"state":   "waiting_room_joined",
					"user_pn": types.JID{User: "96170600887", Server: types.DefaultUserServer},
				},
			}},
		}},
	}
	update, err := ParseWaitingRoomUpdate(&node)
	if err != nil {
		t.Fatal(err)
	}
	if !update.Enabled || !update.IsAdmin || update.TransactionID != 2 {
		t.Fatalf("update = %#v", update)
	}
	if len(update.Users) != 1 || update.Users[0].State != "waiting_room_joined" {
		t.Fatalf("users = %#v", update.Users)
	}
}
