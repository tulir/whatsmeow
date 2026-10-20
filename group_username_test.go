package whatsmeow

import (
	"reflect"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func TestGroupInfoUsername(t *testing.T) {
	lid := types.NewJID("10001", types.HiddenUserServer)
	pn := types.NewJID("15550000001", types.DefaultUserServer)
	otherPN := types.NewJID("15550000002", types.DefaultUserServer)
	otherLID := types.NewJID("10002", types.HiddenUserServer)
	node := waBinary.Node{
		Tag:   "group",
		Attrs: waBinary.Attrs{"id": "100-200", "subject": "Group", "addressing_mode": "lid"},
		Content: []waBinary.Node{
			{Tag: "participant", Attrs: waBinary.Attrs{"jid": lid, "phone_number": pn, "username": "alice", "type": "admin"}},
			{Tag: "participant", Attrs: waBinary.Attrs{"jid": otherPN, "lid": otherLID, "username": "bob"}},
			{Tag: "participant", Attrs: waBinary.Attrs{"jid": types.NewJID("10003", types.HiddenUserServer)}},
		},
	}
	cli := NewClient(&store.Device{}, nil)
	info, err := cli.parseGroupNode(&node)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Participants) != 3 {
		t.Fatalf("participants = %v, want three participants", info.Participants)
	}
	want := []string{"alice", "bob", ""}
	for i, participant := range info.Participants {
		if participant.Username != want[i] {
			t.Errorf("participant %d username = %q, want %q", i, participant.Username, want[i])
		}
	}
	if first := info.Participants[0]; first.JID != lid || first.LID != lid || first.PhoneNumber != pn || !first.IsAdmin {
		t.Errorf("LID participant identity or admin changed: %+v", first)
	}
	if second := info.Participants[1]; second.JID != otherPN || second.LID != otherLID || second.PhoneNumber != otherPN {
		t.Errorf("PN participant identity changed: %+v", second)
	}
}

func TestJoinedGroupUsername(t *testing.T) {
	actor := types.NewJID("10001", types.HiddenUserServer)
	member := types.NewJID("10002", types.HiddenUserServer)
	node := waBinary.Node{
		Tag: "notification",
		Attrs: waBinary.Attrs{
			"from": types.NewJID("100-200", types.GroupServer), "type": "w:gp2", "t": "1720000000",
			"participant": actor, "participant_username": "actor", "notify": "Actor name",
		},
		Content: []waBinary.Node{{
			Tag: "create", Attrs: waBinary.Attrs{"type": "new", "key": "create-id"},
			Content: []waBinary.Node{{
				Tag: "group", Attrs: waBinary.Attrs{"id": "100-200", "subject": "Group"},
				Content: []waBinary.Node{{Tag: "participant", Attrs: waBinary.Attrs{"jid": member, "username": "alice"}}},
			}},
		}},
	}
	cli := NewClient(&store.Device{}, nil)
	raw, _, _, err := cli.parseGroupNotification(&node)
	if err != nil {
		t.Fatal(err)
	}
	evt, ok := raw.(*events.JoinedGroup)
	if !ok {
		t.Fatalf("event = %T, want JoinedGroup", raw)
	}
	if evt.SenderUsername != "actor" {
		t.Errorf("sender username = %q, want actor", evt.SenderUsername)
	}
	if len(evt.Participants) != 1 || evt.Participants[0].Username != "alice" {
		t.Errorf("member usernames = %+v, want alice", evt.Participants)
	}
	if evt.Sender == nil || *evt.Sender != actor || evt.Notify != "Actor name" {
		t.Errorf("existing sender/notify changed: %+v", evt)
	}
}

func TestGroupChangeUsername(t *testing.T) {
	actor := types.NewJID("10001", types.HiddenUserServer)
	member := types.NewJID("10002", types.HiddenUserServer)
	pn := types.NewJID("15550000002", types.DefaultUserServer)
	for _, action := range []string{"add", "remove", "promote", "demote", "modify"} {
		t.Run(action, func(t *testing.T) {
			node := groupUsernameChangeNode(actor, waBinary.Node{
				Tag: action, Attrs: waBinary.Attrs{"prev_v_id": "old", "v_id": "new"},
				Content: []waBinary.Node{{Tag: "participant", Attrs: waBinary.Attrs{
					"jid": member, "phone_number": pn, "username": "alice",
				}}},
			})
			cli := NewClient(&store.Device{}, nil)
			raw, mappings, _, err := cli.parseGroupNotification(&node)
			if err != nil {
				t.Fatal(err)
			}
			evt, ok := raw.(*events.GroupInfo)
			if !ok {
				t.Fatalf("event = %T, want GroupInfo", raw)
			}
			if evt.SenderUsername != "actor" {
				t.Errorf("sender username = %q, want actor", evt.SenderUsername)
			}
			if !reflect.DeepEqual(evt.ParticipantUsernames, map[types.JID]string{member: "alice"}) {
				t.Errorf("participant usernames = %v, want alice keyed by member JID", evt.ParticipantUsernames)
			}
			lists := map[string][]types.JID{
				"add": evt.Join, "remove": evt.Leave, "promote": evt.Promote, "demote": evt.Demote, "modify": evt.Modify,
			}
			if !reflect.DeepEqual(lists[action], []types.JID{member}) {
				t.Errorf("%s members = %v, want %v", action, lists[action], member)
			}
			if evt.PrevParticipantVersionID != "old" || evt.ParticipantVersionID != "new" {
				t.Errorf("participant versions = %q/%q, want old/new", evt.PrevParticipantVersionID, evt.ParticipantVersionID)
			}
			if len(evt.UnknownChanges) != 0 {
				t.Errorf("known action appears in UnknownChanges: %v", evt.UnknownChanges)
			}
			if !reflect.DeepEqual(mappings, []store.LIDMapping{{LID: member, PN: pn}}) {
				t.Errorf("LID mappings = %v, want member phone number mapping", mappings)
			}
		})
	}
}

func TestGroupChangeUsernameIgnoresMissingAndInvalidMembers(t *testing.T) {
	member := types.NewJID("10002", types.HiddenUserServer)
	node := groupUsernameChangeNode(types.NewJID("10001", types.HiddenUserServer), waBinary.Node{
		Tag: "add", Content: []waBinary.Node{
			{Tag: "participant", Attrs: waBinary.Attrs{"jid": member}},
			{Tag: "participant", Attrs: waBinary.Attrs{"jid": "invalid", "username": "invalid"}},
			{Tag: "participant", Attrs: waBinary.Attrs{"username": "missing-jid"}},
			{Tag: "unrelated", Attrs: waBinary.Attrs{"jid": types.NewJID("10003", types.HiddenUserServer), "username": "unrelated"}},
		},
	})
	delete(node.Attrs, "participant_username")
	cli := NewClient(&store.Device{}, nil)
	evt, _, err := cli.parseGroupChange(&node)
	if err != nil {
		t.Fatal(err)
	}
	if evt.SenderUsername != "" || len(evt.ParticipantUsernames) != 0 {
		t.Errorf("missing/invalid usernames were populated: %+v", evt)
	}
	if !reflect.DeepEqual(evt.Join, []types.JID{member}) {
		t.Errorf("join members = %v, want only valid member %v", evt.Join, member)
	}
}

func TestGroupChangeUsernamePreservesMappingsAcrossActions(t *testing.T) {
	firstLID := types.NewJID("10002", types.HiddenUserServer)
	firstPN := types.NewJID("15550000002", types.DefaultUserServer)
	secondLID := types.NewJID("10003", types.HiddenUserServer)
	secondPN := types.NewJID("15550000003", types.DefaultUserServer)
	node := groupUsernameChangeNode(types.NewJID("10001", types.HiddenUserServer),
		waBinary.Node{Tag: "add", Content: []waBinary.Node{{Tag: "participant", Attrs: waBinary.Attrs{
			"jid": firstLID, "phone_number": firstPN, "username": "alice",
		}}}},
		waBinary.Node{Tag: "remove", Content: []waBinary.Node{{Tag: "participant", Attrs: waBinary.Attrs{
			"jid": secondPN, "lid": secondLID, "username": "bob",
		}}}},
	)
	cli := NewClient(&store.Device{}, nil)
	evt, mappings, err := cli.parseGroupChange(&node)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(evt.ParticipantUsernames, map[types.JID]string{firstLID: "alice", secondPN: "bob"}) {
		t.Errorf("participant usernames = %v, want both LID and PN members", evt.ParticipantUsernames)
	}
	wantMappings := []store.LIDMapping{{LID: firstLID, PN: firstPN}, {LID: secondLID, PN: secondPN}}
	if !reflect.DeepEqual(mappings, wantMappings) {
		t.Errorf("LID mappings = %v, want %v", mappings, wantMappings)
	}
}

func TestGroupChangeUsernameIgnoresEmptyAndInvalidValues(t *testing.T) {
	node := groupUsernameChangeNode(types.NewJID("10001", types.HiddenUserServer), waBinary.Node{
		Tag: "modify", Content: []waBinary.Node{
			{Tag: "participant", Attrs: waBinary.Attrs{"jid": types.NewJID("10002", types.HiddenUserServer), "username": ""}},
			{Tag: "participant", Attrs: waBinary.Attrs{"jid": types.NewJID("10003", types.HiddenUserServer), "username": 42}},
			{Tag: "participant", Attrs: waBinary.Attrs{"jid": types.EmptyJID, "username": "invalid-jid"}},
			{Tag: "participant", Attrs: waBinary.Attrs{"jid": types.NewJID("", types.HiddenUserServer), "username": "missing-user"}},
		},
	})
	cli := NewClient(&store.Device{}, nil)
	evt, _, err := cli.parseGroupChange(&node)
	if err != nil {
		t.Fatal(err)
	}
	if len(evt.ParticipantUsernames) != 0 {
		t.Errorf("empty/invalid usernames were populated: %v", evt.ParticipantUsernames)
	}
}

func groupUsernameChangeNode(actor types.JID, changes ...waBinary.Node) waBinary.Node {
	return waBinary.Node{
		Tag: "notification",
		Attrs: waBinary.Attrs{
			"from": types.NewJID("100-200", types.GroupServer), "type": "w:gp2", "t": "1720000000",
			"participant": actor, "participant_username": "actor", "notify": "Actor name",
		},
		Content: changes,
	}
}
