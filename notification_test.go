package whatsmeow

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/socket"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func contactChangeNode() waBinary.Node {
	return waBinary.Node{
		Tag:   "notification",
		Attrs: waBinary.Attrs{"type": "contacts", "from": "s.whatsapp.net", "id": "change"},
		Content: []waBinary.Node{{
			Tag: "modify",
			Attrs: waBinary.Attrs{
				"old": "101@s.whatsapp.net", "new": "202@s.whatsapp.net",
				"old_lid": "701@lid", "new_lid": "702@lid",
			},
		}},
	}
}

func contactChangeAttrs(node *waBinary.Node) waBinary.Attrs {
	return node.Content.([]waBinary.Node)[0].Attrs
}

type notificationTestLogger struct {
	waLog.Logger
	debug func(string, ...any)
	warn  func(string, ...any)
}

func (log notificationTestLogger) Debugf(format string, args ...any) {
	if log.debug != nil {
		log.debug(format, args...)
	}
}

func (log notificationTestLogger) Warnf(format string, args ...any) {
	if log.warn != nil {
		log.warn(format, args...)
	}
}

type notificationHarness struct {
	cli        *Client
	stores     *notificationStores
	sent       chan waBinary.Node
	ackDone    chan struct{}
	rejections chan string
}

func newNotificationHarness(t *testing.T, synchronous bool) *notificationHarness {
	t.Helper()
	stores := &notificationStores{
		mappings: map[types.JID]types.JID{types.NewJID("701", types.HiddenUserServer): types.NewJID("101", types.DefaultUserServer)},
		contacts: map[types.JID]types.ContactInfo{types.NewJID("101", types.DefaultUserServer): {FullName: "Test Contact"}},
	}
	cli := NewClient(&store.Device{LIDs: stores, Contacts: stores}, waLog.Noop)
	cli.SynchronousAck = synchronous
	harness := &notificationHarness{cli: cli, stores: stores, sent: make(chan waBinary.Node, 16), ackDone: make(chan struct{}, 16), rejections: make(chan string, 16)}
	cli.sendLog = notificationTestLogger{Logger: waLog.Noop, debug: func(_ string, args ...any) {
		if len(args) != 1 {
			t.Errorf("unexpected send log arguments: %d", len(args))
			return
		}
		node, ok := args[0].(*waBinary.Node)
		if !ok {
			t.Errorf("unexpected send log value: %T", args[0])
			return
		}
		select {
		case harness.sent <- *node:
		default:
			t.Error("too many outgoing nodes")
		}
	}}
	cli.Log = notificationTestLogger{Logger: waLog.Noop, debug: func(format string, args ...any) {
		if format != "Ignoring contacts notification: %s" || len(args) != 1 {
			t.Error("rejection log contains unexpected fields")
			return
		}
		reason, ok := args[0].(string)
		if !ok || !validContactChangeRejection(reason) {
			t.Error("rejection log contains an unbounded reason")
			return
		}
		harness.rejections <- reason
	}, warn: func(format string, _ ...any) {
		if format == "Failed to send acknowledgement for %s %s: %v" {
			harness.ackDone <- struct{}{}
		} else {
			t.Errorf("unexpected warning format: %s", format)
		}
	}}
	handshake := socket.NewNoiseHandshake()
	handshake.Start("notification test", nil)
	var err error
	transport := &socket.FrameSocket{}
	if transport.IsConnected() || transport.Context() != nil {
		t.Fatal("test transport must be disconnected")
	}
	cli.socket, err = handshake.Finish(transport, func(context.Context, []byte) {}, func(context.Context, *socket.NoiseSocket, bool) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cli.socket.Stop(false, false)
		if transport.IsConnected() || transport.Context() != nil {
			t.Error("notification connected the test transport")
		}
	})
	return harness
}

func (harness *notificationHarness) frame(t *testing.T, node waBinary.Node) *waBinary.Node {
	t.Helper()
	data, err := waBinary.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}
	queue := make(chan *waBinary.Node, 1)
	harness.cli.handleFrame(context.Background(), data, queue)
	select {
	case decoded := <-queue:
		return decoded
	default:
		t.Fatal("notification was not queued by handleFrame")
		return nil
	}
}

func (harness *notificationHarness) deliver(t *testing.T, ctx context.Context, node waBinary.Node) *waBinary.Node {
	t.Helper()
	decoded := harness.frame(t, node)
	handler, ok := harness.cli.nodeHandlers[decoded.Tag]
	if !ok {
		t.Fatal("notification handler was not registered")
	}
	handler(ctx, decoded)
	return decoded
}

func (harness *notificationHarness) finishAck(t *testing.T, expected bool) []waBinary.Node {
	t.Helper()
	if expected {
		select {
		case <-harness.ackDone:
		case <-time.After(5 * time.Second):
			t.Fatal("acknowledgement did not finish on the disconnected transport")
		}
	}
	var sent []waBinary.Node
	for {
		select {
		case node := <-harness.sent:
			sent = append(sent, node)
		default:
			if expected && len(sent) != 1 || !expected && len(sent) != 0 {
				t.Fatalf("got %d outgoing nodes, expected ACK: %t", len(sent), expected)
			}
			return sent
		}
	}
}

func (harness *notificationHarness) assertStoresUntouched(t *testing.T) {
	t.Helper()
	harness.stores.mu.Lock()
	defer harness.stores.mu.Unlock()
	if calls := harness.stores.calls.Load(); calls != 0 {
		t.Errorf("notification made %d store calls", calls)
	}
	expectedMappings := map[types.JID]types.JID{types.NewJID("701", types.HiddenUserServer): types.NewJID("101", types.DefaultUserServer)}
	expectedContacts := map[types.JID]types.ContactInfo{types.NewJID("101", types.DefaultUserServer): {FullName: "Test Contact"}}
	if !reflect.DeepEqual(harness.stores.mappings, expectedMappings) || !reflect.DeepEqual(harness.stores.contacts, expectedContacts) {
		t.Error("notification changed pre-existing store contents")
	}
}

func validContactChangeRejection(reason string) bool {
	switch reason {
	case "invalid envelope", "invalid sender", "invalid stanza ID", "unsupported action", "invalid old PN", "invalid new PN", "invalid old LID", "invalid new LID":
		return true
	default:
		return false
	}
}

func TestContactNumberChangedFrames(t *testing.T) {
	cases := []struct {
		name   string
		edit   func(*waBinary.Node)
		oldPN  string
		newPN  string
		oldLID string
		newLID string
	}{
		{name: "strings", oldPN: "101", newPN: "202", oldLID: "701", newLID: "702"},
		{name: "typed", edit: func(node *waBinary.Node) {
			node.Attrs["from"] = types.ServerJID
			for key, value := range contactChangeAttrs(node) {
				jid, err := types.ParseJID(value.(string))
				if err != nil {
					t.Fatal(err)
				}
				contactChangeAttrs(node)[key] = jid
			}
		}, oldPN: "101", newPN: "202", oldLID: "701", newLID: "702"},
		{name: "mixed", edit: func(node *waBinary.Node) {
			contactChangeAttrs(node)["old"] = types.NewJID("101", types.DefaultUserServer)
			contactChangeAttrs(node)["new_lid"] = types.NewJID("702", types.HiddenUserServer)
		}, oldPN: "101", newPN: "202", oldLID: "701", newLID: "702"},
		{name: "no LIDs", edit: func(node *waBinary.Node) {
			delete(contactChangeAttrs(node), "old_lid")
			delete(contactChangeAttrs(node), "new_lid")
		}, oldPN: "101", newPN: "202"},
		{name: "old LID only", edit: func(node *waBinary.Node) { delete(contactChangeAttrs(node), "new_lid") }, oldPN: "101", newPN: "202", oldLID: "701"},
		{name: "new LID only", edit: func(node *waBinary.Node) { delete(contactChangeAttrs(node), "old_lid") }, oldPN: "101", newPN: "202", newLID: "702"},
		{name: "same PN", edit: func(node *waBinary.Node) { contactChangeAttrs(node)["new"] = "101@s.whatsapp.net" }, oldPN: "101", newPN: "101", oldLID: "701", newLID: "702"},
		{name: "same PN and LID", edit: func(node *waBinary.Node) {
			contactChangeAttrs(node)["new"] = "101@s.whatsapp.net"
			contactChangeAttrs(node)["new_lid"] = "701@lid"
		}, oldPN: "101", newPN: "101", oldLID: "701", newLID: "701"},
		{name: "beyond uint64", edit: func(node *waBinary.Node) {
			contactChangeAttrs(node)["old"] = "123456789012345678901234567890@s.whatsapp.net"
		}, oldPN: "123456789012345678901234567890", newPN: "202", oldLID: "701", newLID: "702"},
		{name: "leading zeroes", edit: func(node *waBinary.Node) { contactChangeAttrs(node)["old"] = "00101@s.whatsapp.net" }, oldPN: "00101", newPN: "202", oldLID: "701", newLID: "702"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			harness := newNotificationHarness(t, true)
			var received []*events.ContactNumberChanged
			harness.cli.AddEventHandler(func(event any) {
				if changed, ok := event.(*events.ContactNumberChanged); ok {
					received = append(received, changed)
				}
			})
			node := contactChangeNode()
			if tc.edit != nil {
				tc.edit(&node)
			}
			harness.deliver(t, context.Background(), node)
			harness.finishAck(t, true)
			if len(received) != 1 {
				t.Fatalf("expected one event, got %d", len(received))
			}
			expected := &events.ContactNumberChanged{StanzaID: "change", OldPN: types.NewJID(tc.oldPN, types.DefaultUserServer), NewPN: types.NewJID(tc.newPN, types.DefaultUserServer)}
			if tc.oldLID != "" {
				jid := types.NewJID(tc.oldLID, types.HiddenUserServer)
				expected.OldLID = &jid
			}
			if tc.newLID != "" {
				jid := types.NewJID(tc.newLID, types.HiddenUserServer)
				expected.NewLID = &jid
			}
			if !reflect.DeepEqual(received[0], expected) {
				t.Fatalf("unexpected event: %+v", received[0])
			}
			encoded, err := json.Marshal(received[0])
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err = json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if fields["OldPN"] != expected.OldPN.String() || fields["NewPN"] != expected.NewPN.String() {
				t.Fatal("JSON lost PN domains")
			}
			for key, jid := range map[string]*types.JID{"OldLID": expected.OldLID, "NewLID": expected.NewLID} {
				if jid == nil && fields[key] != nil || jid != nil && fields[key] != jid.String() {
					t.Fatalf("JSON lost optional %s", key)
				}
			}
			harness.assertStoresUntouched(t)
		})
	}
}

func TestContactNumberChangedInvalidFrames(t *testing.T) {
	cases := []struct {
		name string
		edit func(*waBinary.Node)
	}{
		{"missing old", func(n *waBinary.Node) { delete(contactChangeAttrs(n), "old") }},
		{"missing new", func(n *waBinary.Node) { delete(contactChangeAttrs(n), "new") }},
		{"missing ID", func(n *waBinary.Node) { delete(n.Attrs, "id") }},
		{"empty ID", func(n *waBinary.Node) { n.Attrs["id"] = []byte{} }},
		{"missing sender", func(n *waBinary.Node) { delete(n.Attrs, "from") }},
		{"user sender", func(n *waBinary.Node) { n.Attrs["from"] = types.NewJID("101", types.DefaultUserServer) }},
		{"group sender", func(n *waBinary.Node) { n.Attrs["from"] = types.NewJID("101-202", types.GroupServer) }},
		{"sender at prefix", func(n *waBinary.Node) { n.Attrs["from"] = "@s.whatsapp.net" }},
		{"no child", func(n *waBinary.Node) { n.Content = nil }},
		{"multiple actions", func(n *waBinary.Node) { n.Content = append(n.GetChildren(), waBinary.Node{Tag: "sync"}) }},
		{"repeated modify", func(n *waBinary.Node) { n.Content = append(n.GetChildren(), n.GetChildren()[0]) }},
		{"nested action", func(n *waBinary.Node) { n.Content = []waBinary.Node{{Tag: "wrapper", Content: n.Content}} }},
		{"nested modify content", func(n *waBinary.Node) { n.GetChildren()[0].Content = []waBinary.Node{{Tag: "modify"}} }},
		{"text modify content", func(n *waBinary.Node) { n.GetChildren()[0].Content = []byte("ignored") }},
	}
	for _, action := range []string{"update", "sync", "add", "remove"} {
		cases = append(cases, struct {
			name string
			edit func(*waBinary.Node)
		}{action, func(n *waBinary.Node) { n.GetChildren()[0].Tag = action }})
	}
	for _, key := range []string{"old", "new", "old_lid", "new_lid"} {
		server := types.DefaultUserServer
		if key == "old_lid" || key == "new_lid" {
			server = types.HiddenUserServer
		}
		otherServer := types.HiddenUserServer
		if server == types.HiddenUserServer {
			otherServer = types.DefaultUserServer
		}
		for name, value := range map[string]any{
			"opposite personal domain": "101@" + otherServer,
			"empty":                    []byte{}, "empty user": "@" + server, "non-numeric": "test@" + server,
			"non-ASCII": "１２３@" + server, "extra at": "101@" + server + "@other",
			"wrong domain": "101@g.us", "hosted": "101@hosted", "hosted LID": "101@hosted.lid",
			"device":      types.JID{User: "101", Server: server, Device: 2},
			"device text": "101:2@" + server, "agent text": "101.1:0@" + server,
			"zero device text": "101:0@" + server, "whitespace": " 101@" + server,
			"negative": "-101@" + server, "no domain": "101",
		} {
			cases = append(cases, struct {
				name string
				edit func(*waBinary.Node)
			}{key + "/" + name, func(n *waBinary.Node) { contactChangeAttrs(n)[key] = value }})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			harness := newNotificationHarness(t, true)
			var count int
			harness.cli.AddEventHandler(func(any) { count++ })
			node := contactChangeNode()
			tc.edit(&node)
			decoded := harness.deliver(t, context.Background(), node)
			harness.finishAck(t, true)
			if tc.name == "empty ID" {
				if value, present := decoded.Attrs["id"]; !present || value != "" {
					t.Fatal("empty ID was not preserved by the codec")
				}
			}
			for _, key := range []string{"old", "new", "old_lid", "new_lid"} {
				if tc.name == key+"/empty" {
					if value, present := decoded.GetChildren()[0].Attrs[key]; !present || value != "" {
						t.Fatalf("empty %s was not preserved by the codec", key)
					}
				}
			}
			if count != 0 {
				t.Fatalf("invalid notification emitted %d events", count)
			}
			select {
			case <-harness.rejections:
			default:
				t.Error("missing bounded rejection log")
			}
			harness.assertStoresUntouched(t)
		})
	}
}

func TestContactNumberChangedParserMetadata(t *testing.T) {
	for _, field := range []string{"old", "new", "old_lid", "new_lid", "from"} {
		for _, metadata := range []string{"agent", "device", "integrator"} {
			t.Run(field+"/"+metadata, func(t *testing.T) {
				node := contactChangeNode()
				server := types.DefaultUserServer
				if field == "old_lid" || field == "new_lid" {
					server = types.HiddenUserServer
				}
				jid := types.NewJID("101", server)
				if field == "from" {
					jid = types.ServerJID
				}
				switch metadata {
				case "agent":
					jid.RawAgent = 1
				case "device":
					jid.Device = 1
				case "integrator":
					jid.Integrator = 1
				}
				if field == "from" {
					node.Attrs[field] = jid
				} else {
					contactChangeAttrs(&node)[field] = jid
				}
				if event, reason := parseContactNumberChanged(&node); event != nil || !validContactChangeRejection(reason) {
					t.Fatalf("metadata admitted: %+v, %q", event, reason)
				}
			})
		}
	}
	for _, tc := range []struct {
		name string
		edit func(*waBinary.Node)
	}{
		{"ID integer", func(n *waBinary.Node) { n.Attrs["id"] = 123 }},
		{"wrong tag", func(n *waBinary.Node) { n.Tag = "message" }},
		{"wrong type", func(n *waBinary.Node) { n.Attrs["type"] = "status" }},
		{"nil optional", func(n *waBinary.Node) { contactChangeAttrs(n)["old_lid"] = nil }},
		{"wrong attribute type", func(n *waBinary.Node) { contactChangeAttrs(n)["new"] = []byte("202@s.whatsapp.net") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node := contactChangeNode()
			tc.edit(&node)
			if event, reason := parseContactNumberChanged(&node); event != nil || !validContactChangeRejection(reason) {
				t.Fatalf("invalid node admitted: %+v, %q", event, reason)
			}
		})
	}
	node := contactChangeNode()
	event, reason := parseContactNumberChanged(&node)
	if event == nil {
		t.Fatalf("valid parser input rejected: %s", reason)
	}
	contactChangeAttrs(&node)["old_lid"] = "999@lid"
	contactChangeAttrs(&node)["new_lid"] = "999@lid"
	contactChangeAttrs(&node)["old"] = "999@s.whatsapp.net"
	contactChangeAttrs(&node)["new"] = "999@s.whatsapp.net"
	node.Attrs["id"] = "changed"
	if event.StanzaID != "change" || event.OldPN.User != "101" || event.NewPN.User != "202" || event.OldLID.User != "701" || event.NewLID.User != "702" {
		t.Fatal("event retained mutable node data")
	}
}

func TestContactNumberChangedClientsAndReplay(t *testing.T) {
	a := newNotificationHarness(t, true)
	b := newNotificationHarness(t, true)
	var receivedA, receivedB []events.ContactNumberChanged
	staleA, staleB := false, false
	mark := func(received *[]events.ContactNumberChanged, stale *bool) EventHandler {
		return func(value any) {
			if event, ok := value.(*events.ContactNumberChanged); ok {
				*received = append(*received, *event)
				if event.OldPN.User == "101" || event.NewPN.User == "101" {
					*stale = true
				}
			}
		}
	}
	a.cli.AddEventHandler(mark(&receivedA, &staleA))
	b.cli.AddEventHandler(mark(&receivedB, &staleB))
	node := contactChangeNode()
	for range 2 {
		a.deliver(t, context.Background(), node)
		a.finishAck(t, true)
	}
	if len(receivedA) != 2 || !reflect.DeepEqual(receivedA[0], receivedA[1]) || !staleA || staleB || len(receivedB) != 0 {
		t.Fatal("replay or client scope was not preserved")
	}
	node.Attrs["id"] = "forward"
	a.deliver(t, context.Background(), node)
	a.finishAck(t, true)
	node.Attrs["id"] = "back"
	contactChangeAttrs(&node)["old"] = "202@s.whatsapp.net"
	contactChangeAttrs(&node)["new"] = "101@s.whatsapp.net"
	a.deliver(t, context.Background(), node)
	a.finishAck(t, true)
	if len(receivedA) != 4 || receivedA[2].StanzaID != "forward" || receivedA[3].StanzaID != "back" || receivedA[3].NewPN != receivedA[2].OldPN {
		t.Fatal("return to the old number was lost")
	}
	if len(receivedB) != 0 || staleB {
		t.Fatal("same remote identifiers notified another client")
	}
	b.deliver(t, context.Background(), contactChangeNode())
	b.finishAck(t, true)
	if len(receivedA) != 4 || len(receivedB) != 1 || !staleB {
		t.Fatal("origin was inferred from shared remote identifiers")
	}
	a.assertStoresUntouched(t)
	b.assertStoresUntouched(t)
}

func TestContactNumberChangedAck(t *testing.T) {
	for _, synchronous := range []bool{false, true} {
		for _, mode := range []string{"success", "failure", "cancelled"} {
			t.Run(fmt.Sprintf("sync=%t/%s", synchronous, mode), func(t *testing.T) {
				var outputs [][]waBinary.Node
				for _, control := range []bool{false, true} {
					harness := newNotificationHarness(t, synchronous)
					ctx, cancel := context.WithCancel(context.Background())
					if mode == "cancelled" {
						cancel()
					}
					var calls int
					harness.cli.AddEventHandlerWithSuccessStatus(func(any) bool { calls++; return mode != "failure" })
					node := contactChangeNode()
					if control {
						node.GetChildren()[0].Tag = "sync"
					}
					decoded := harness.deliver(t, ctx, node)
					expectedAck := !synchronous || mode != "cancelled"
					sent := harness.finishAck(t, expectedAck)
					cancel()
					if !control && calls != 1 || control && calls != 0 {
						t.Fatalf("unexpected handler count %d", calls)
					}
					if expectedAck {
						expected := waBinary.Node{Tag: "ack", Attrs: waBinary.Attrs{"class": "notification", "id": decoded.Attrs["id"], "to": decoded.Attrs["from"], "type": "contacts"}}
						if !reflect.DeepEqual(sent[0], expected) {
							t.Fatalf("unexpected outgoing node: %+v", sent[0])
						}
					}
					outputs = append(outputs, sent)
					harness.assertStoresUntouched(t)
				}
				if !reflect.DeepEqual(outputs[0], outputs[1]) {
					t.Fatal("ACK differs from ignored notification control")
				}
			})
		}
	}
}

func TestContactNumberChangedAckWaitsForHandler(t *testing.T) {
	for _, synchronous := range []bool{false, true} {
		t.Run(fmt.Sprintf("sync=%t", synchronous), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				harness := newNotificationHarness(t, synchronous)
				entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
				var unblock sync.Once
				harness.cli.AddEventHandler(func(any) { close(entered); <-release })
				node := harness.frame(t, contactChangeNode())
				go func() { defer close(finished); harness.cli.nodeHandlers[node.Tag](context.Background(), node) }()
				defer func() {
					unblock.Do(func() { close(release) })
					<-finished
				}()

				synctest.Wait()
				select {
				case <-entered:
				default:
					t.Fatal("handler did not start")
				}
				if synchronous {
					select {
					case <-harness.sent:
						t.Error("ACK sent before handler returned")
					default:
					}
				} else {
					select {
					case <-harness.ackDone:
					default:
						t.Error("asynchronous ACK waited for handler")
					}
				}
				unblock.Do(func() { close(release) })
				<-finished
				if synchronous {
					harness.finishAck(t, true)
				} else {
					select {
					case node := <-harness.sent:
						if node.Tag != "ack" {
							t.Fatal("unexpected outgoing node")
						}
					default:
						t.Fatal("missing asynchronous ACK")
					}
				}
				harness.assertStoresUntouched(t)
			})
		})
	}
}

type notificationStores struct {
	calls    atomic.Int32
	mu       sync.Mutex
	mappings map[types.JID]types.JID
	contacts map[types.JID]types.ContactInfo
}

func (s *notificationStores) PutManyLIDMappings(ctx context.Context, mappings []store.LIDMapping) error {
	s.calls.Add(1)
	for _, mapping := range mappings {
		s.PutLIDMapping(ctx, mapping.LID, mapping.PN)
	}
	return nil
}
func (s *notificationStores) PutLIDMapping(_ context.Context, lid, pn types.JID) error {
	s.calls.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mappings[lid] = pn
	return nil
}
func (s *notificationStores) GetPNForLID(_ context.Context, lid types.JID) (types.JID, error) {
	s.calls.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.mappings[lid], nil
}
func (s *notificationStores) GetLIDForPN(_ context.Context, pn types.JID) (types.JID, error) {
	s.calls.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	for lid, mapped := range s.mappings {
		if mapped == pn {
			return lid, nil
		}
	}
	return types.EmptyJID, nil
}
func (s *notificationStores) GetManyLIDsForPNs(ctx context.Context, pns []types.JID) (map[types.JID]types.JID, error) {
	s.calls.Add(1)
	result := make(map[types.JID]types.JID)
	for _, pn := range pns {
		result[pn], _ = s.GetLIDForPN(ctx, pn)
	}
	return result, nil
}
func (s *notificationStores) PutPushName(_ context.Context, _ types.JID, _ string) (bool, string, error) {
	s.calls.Add(1)
	return false, "", nil
}
func (s *notificationStores) PutBusinessName(_ context.Context, _ types.JID, _ string) (bool, string, error) {
	s.calls.Add(1)
	return false, "", nil
}
func (s *notificationStores) PutContactName(_ context.Context, _ types.JID, _, _ string) error {
	s.calls.Add(1)
	return nil
}
func (s *notificationStores) PutAllContactNames(_ context.Context, _ []store.ContactEntry) error {
	s.calls.Add(1)
	return nil
}
func (s *notificationStores) PutManyRedactedPhones(_ context.Context, _ []store.RedactedPhoneEntry) error {
	s.calls.Add(1)
	return nil
}
func (s *notificationStores) GetContact(_ context.Context, user types.JID) (types.ContactInfo, error) {
	s.calls.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contacts[user], nil
}
func (s *notificationStores) GetAllContacts(_ context.Context) (map[types.JID]types.ContactInfo, error) {
	s.calls.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[types.JID]types.ContactInfo)
	for jid, contact := range s.contacts {
		result[jid] = contact
	}
	return result, nil
}
