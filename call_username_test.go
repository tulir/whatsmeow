package whatsmeow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type callParticipantOutput struct {
	JID         types.JID
	PhoneNumber types.JID
	Username    string
	State       string
}

func captureCallEvent(t *testing.T, node *waBinary.Node) any {
	t.Helper()
	device := *store.NoopDevice
	ownID := types.NewJID("15550000001", types.DefaultUserServer)
	device.ID = &ownID
	cli := NewClient(&device, nil)
	cli.SynchronousAck = true
	var received []any
	cli.AddEventHandler(func(evt any) { received = append(received, evt) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cli.handleCallEvent(ctx, node)
	if len(received) != 1 {
		t.Fatalf("got %d call events, want 1", len(received))
	}
	return received[0]
}

func TestCallParticipantUsernames(t *testing.T) {
	lid := types.NewJID("200000000000001", types.HiddenUserServer)
	phone := types.NewJID("15550000002", types.DefaultUserServer)
	group := types.NewJID("120363000000001", types.GroupServer)
	outerGroup := types.NewJID("120363000000002", types.GroupServer)
	for _, tc := range []struct {
		name         string
		childTag     string
		containerTag string
		group        types.JID
		users        []waBinary.Node
		wantType     string
		wantGroup    types.JID
		want         []callParticipantOutput
	}{
		{
			name:     "offer preserves every participant and nested group",
			childTag: "offer", containerTag: "group_info", wantType: "*events.CallOffer", wantGroup: group,
			users: []waBinary.Node{
				{Tag: "user", Attrs: waBinary.Attrs{"jid": lid, "user_pn": phone, "username": "Alice.Example_24", "state": "invited"}, Content: []waBinary.Node{{Tag: "device", Attrs: waBinary.Attrs{"jid": lid, "username": "device.username"}}}},
				{Tag: "user", Attrs: waBinary.Attrs{"jid": phone, "username": "bob.example", "state": "connected"}},
				{Tag: "user", Attrs: waBinary.Attrs{"jid": types.NewJID("200000000000003", types.HiddenUserServer), "state": "invited"}},
				{Tag: "device", Attrs: waBinary.Attrs{"jid": phone, "username": "ignored.device"}},
				{Tag: "user", Attrs: waBinary.Attrs{"username": "missing.jid"}},
				{Tag: "user", Attrs: waBinary.Attrs{"jid": "invalid jid", "username": "invalid.jid"}},
				{Tag: "user", Attrs: waBinary.Attrs{"jid": types.EmptyJID, "username": "empty.jid"}},
				{Tag: "user", Attrs: waBinary.Attrs{"jid": types.NewJID("", types.HiddenUserServer), "username": "missing.user"}},
			},
			want: []callParticipantOutput{
				{JID: lid, PhoneNumber: phone, Username: "Alice.Example_24", State: "invited"},
				{JID: phone, Username: "bob.example", State: "connected"},
				{JID: types.NewJID("200000000000003", types.HiddenUserServer), State: "invited"},
			},
		},
		{
			name:     "group update emits a structured event and preserves outer group",
			childTag: "group_update", containerTag: "group_info", group: outerGroup, wantType: "*events.CallGroupUpdate", wantGroup: outerGroup,
			users: []waBinary.Node{{Tag: "user", Attrs: waBinary.Attrs{"jid": lid, "username": "alice.updated", "state": "connected"}}},
			want:  []callParticipantOutput{{JID: lid, Username: "alice.updated", State: "connected"}},
		},
		{
			name:     "terminate extracts call summary participants",
			childTag: "terminate", containerTag: "call_summary", wantType: "*events.CallTerminate",
			users: []waBinary.Node{
				{Tag: "user", Attrs: waBinary.Attrs{"jid": lid, "username": "alice.summary", "state": "connected"}},
				{Tag: "user", Attrs: waBinary.Attrs{"jid": types.NewJID("200000000000003", types.HiddenUserServer), "user_pn": phone, "state": "invited"}},
			},
			want: []callParticipantOutput{
				{JID: lid, Username: "alice.summary", State: "connected"},
				{JID: types.NewJID("200000000000003", types.HiddenUserServer), PhoneNumber: phone, State: "invited"},
			},
		},
		{
			name:     "offer without participants remains valid",
			childTag: "offer", wantType: "*events.CallOffer",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := waBinary.Node{Tag: tc.childTag, Attrs: waBinary.Attrs{"call-creator": lid, "call-id": "test-call", "reason": "timeout"}}
			if !tc.group.IsEmpty() {
				child.Attrs["group-jid"] = tc.group
			}
			if tc.containerTag != "" {
				container := waBinary.Node{Tag: tc.containerTag, Content: tc.users}
				if tc.containerTag == "group_info" {
					container.Attrs = waBinary.Attrs{"group-jid": group}
				}
				child.Content = []waBinary.Node{container}
			}
			node := waBinary.Node{Tag: "call", Attrs: waBinary.Attrs{"from": lid, "id": "test-stanza", "t": "1792430649", "platform": "android", "version": "2.26.1"}, Content: []waBinary.Node{child}}
			evt := captureCallEvent(t, &node)
			if got := fmt.Sprintf("%T", evt); got != tc.wantType {
				t.Fatalf("event type = %s, want %s", got, tc.wantType)
			}
			encoded, err := json.Marshal(evt)
			if err != nil {
				t.Fatal(err)
			}
			var output struct {
				From         types.JID
				CallCreator  types.JID
				CallID       string
				GroupJID     types.JID
				Participants []callParticipantOutput
				Data         json.RawMessage
			}
			if err = json.Unmarshal(encoded, &output); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(output.Participants, tc.want) {
				t.Errorf("Participants = %#v, want %#v", output.Participants, tc.want)
			}
			if output.GroupJID != tc.wantGroup {
				t.Errorf("GroupJID = %s, want %s", output.GroupJID, tc.wantGroup)
			}
			if output.From != lid || output.CallCreator != lid || output.CallID != "test-call" {
				t.Errorf("call identity changed: %s", encoded)
			}
			wantData, err := json.Marshal(child)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(output.Data, wantData) {
				t.Errorf("raw call data = %s, want %s", output.Data, wantData)
			}
		})
	}
}

func TestCallUnknownEventsRemainRaw(t *testing.T) {
	for _, children := range [][]waBinary.Node{
		{{Tag: "future_call_event", Attrs: waBinary.Attrs{"username": "future.example"}}},
		nil,
		{{Tag: "offer"}, {Tag: "terminate"}},
	} {
		node := waBinary.Node{Tag: "call", Content: children}
		evt := captureCallEvent(t, &node)
		unknown, ok := evt.(*events.UnknownCallEvent)
		if !ok || unknown.Node != &node {
			t.Errorf("unknown call event = %#v, want original raw node", evt)
		}
	}
}
