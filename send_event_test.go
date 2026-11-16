package whatsmeow

import (
	"testing"

	waBinary "go.mau.fi/whatsmeow/binary"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestGetTypeFromMessageEvent(t *testing.T) {
	msg := &waE2E.Message{EventMessage: &waE2E.EventMessage{Name: proto.String("x"), StartTime: proto.Int64(1)}}
	if got := getTypeFromMessage(msg); got != "event" {
		t.Fatalf("event message type = %q, want event", got)
	}
}

func TestEventMessageContentMeta(t *testing.T) {
	cli := &Client{}
	content := cli.getMessageContent(waBinary.Node{Tag: "enc"}, &waE2E.Message{EventMessage: &waE2E.EventMessage{}}, waBinary.Attrs{"type": "event"}, false, nodeExtraParams{})
	if len(content) != 2 || content[1].Tag != "meta" || content[1].Attrs["event_type"] != "creation" {
		t.Fatalf("unexpected content: %#v", content)
	}
}
