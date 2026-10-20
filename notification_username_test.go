package whatsmeow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func mexUsernameNotification(payload, opName string) *waBinary.Node {
	return &waBinary.Node{
		Tag:   "notification",
		Attrs: waBinary.Attrs{"from": types.NewJID("123", types.HiddenUserServer), "id": "username-change", "t": "1792464475", "type": "mex"},
		Content: []waBinary.Node{{
			Tag: "update", Attrs: waBinary.Attrs{"op_name": opName}, Content: []byte(payload),
		}},
	}
}

func TestHandleMexUsernameNotification(t *testing.T) {
	for _, tc := range []struct {
		name     string
		payload  string
		username string
	}{
		{"set username", `{"data":{"xwa2_notify_username_on_change":{"lid":"123@lid","username":"alice"}}}`, "alice"},
		{"empty username", `{"data":{"xwa2_notify_username_on_change":{"lid":"123@lid","username":""}}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cli := &Client{Log: waLog.Noop}
			var received []any
			cli.AddEventHandler(func(evt any) { received = append(received, evt) })
			cli.handleMexNotification(context.Background(), mexUsernameNotification(tc.payload, "UsernameSetNotification"))
			if len(received) != 1 {
				t.Fatalf("got %d events, want one username change", len(received))
			}
			evt, ok := received[0].(*events.UsernameChange)
			if !ok {
				t.Fatalf("event type = %T, want *events.UsernameChange", received[0])
			}
			encoded, err := json.Marshal(received[0])
			if err != nil {
				t.Fatal(err)
			}
			var fields struct {
				LID      string `json:"lid"`
				Username string `json:"username"`
			}
			if err = json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if fields.LID != "123@lid" || fields.Username != tc.username {
				t.Errorf("username change = %+v, want LID 123@lid and username %q", fields, tc.username)
			}
			if evt.Mex.OpName != "UsernameSetNotification" || !evt.Mex.Timestamp.Equal(time.Unix(1792464475, 0)) {
				t.Errorf("MEX metadata = %+v, want UsernameSetNotification at Unix 1792464475", evt.Mex)
			}
			if strings.Contains(string(encoded), "Mex") {
				t.Errorf("MEX metadata unexpectedly included in JSON: %s", encoded)
			}
		})
	}
}

type mexUsernameTestLogger struct {
	waLog.Logger
	errors []string
}

func (logger *mexUsernameTestLogger) Errorf(message string, args ...any) {
	logger.errors = append(logger.errors, fmt.Sprintf(message, args...))
}

func TestHandleMexUsernameIgnoresInvalidPayload(t *testing.T) {
	for _, tc := range []struct {
		name       string
		payload    string
		parseError bool
	}{
		{"malformed JSON", `{"data":`, true},
		{"missing LID", `{"data":{"xwa2_notify_username_on_change":{"username":"alice"}}}`, false},
		{"empty LID", `{"data":{"xwa2_notify_username_on_change":{"lid":"","username":"alice"}}}`, false},
		{"missing LID user", `{"data":{"xwa2_notify_username_on_change":{"lid":"@lid","username":"alice"}}}`, false},
		{"non-LID identity", `{"data":{"xwa2_notify_username_on_change":{"lid":"123@s.whatsapp.net","username":"alice"}}}`, false},
		{"malformed LID", `{"data":{"xwa2_notify_username_on_change":{"lid":"123:device@lid","username":"alice"}}}`, true},
		{"unknown MEX event", `{"data":{"xwa2_unknown_notification":{"lid":"123@lid","username":"alice"}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := &mexUsernameTestLogger{Logger: waLog.Noop}
			cli := &Client{Log: logger}
			var received []any
			cli.AddEventHandler(func(evt any) { received = append(received, evt) })
			cli.handleMexNotification(context.Background(), mexUsernameNotification(tc.payload, "UsernameSetNotification"))
			if len(received) != 0 {
				t.Errorf("got %d events for invalid or unknown payload: %+v", len(received), received)
			}
			if tc.parseError && (len(logger.errors) != 1 || !strings.Contains(logger.errors[0], "Failed to unmarshal JSON in mex event")) {
				t.Errorf("got error logs %q, want the MEX JSON parse error", logger.errors)
			}
		})
	}
}

func TestHandleMexNewsletterNotification(t *testing.T) {
	cli := &Client{Log: waLog.Noop}
	var received []any
	cli.AddEventHandler(func(evt any) { received = append(received, evt) })
	cli.handleMexNotification(context.Background(), mexUsernameNotification(
		`{"data":{"xwa2_notify_newsletter_on_leave":{"id":"456@newsletter","role":"SUBSCRIBER"}}}`,
		"NewsletterLeaveNotification",
	))
	if len(received) != 1 {
		t.Fatalf("got %d events, want one newsletter leave", len(received))
	}
	evt, ok := received[0].(*events.NewsletterLeave)
	if !ok {
		t.Fatalf("event type = %T, want *events.NewsletterLeave", received[0])
	}
	if evt.ID != types.NewJID("456", types.NewsletterServer) || evt.Role != types.NewsletterRoleSubscriber || evt.Mex.OpName != "NewsletterLeaveNotification" || !evt.Mex.Timestamp.Equal(time.Unix(1792464475, 0)) {
		t.Errorf("newsletter event = %+v, want 456@newsletter subscriber and original MEX metadata", evt)
	}
}
