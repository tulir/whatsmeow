package whatsmeow

import (
	"bytes"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestPerDevicePlaintextsAndOwnDSM(t *testing.T) {
	own := types.NewADJID("100", 0, 9)
	ownLID := types.NewJID("101", types.HiddenUserServer)
	ownLID.Device = 9
	recipient := types.NewJID("200", types.HiddenUserServer)
	linked := recipient
	linked.Device = 3
	ownPhone := own
	ownPhone.Device = 0
	ownLinked := ownLID
	ownLinked.Device = 2
	devices := []types.JID{recipient, linked, ownPhone, ownLinked, own}
	base := &waE2E.Message{InteractiveMessage: &waE2E.InteractiveMessage{Body: &waE2E.InteractiveMessage_Body{Text: proto.String("base")}}}
	primary := &waE2E.Message{ViewOnceMessage: &waE2E.FutureProofMessage{Message: base}}
	called := map[types.JID]bool{}
	hook := func(jid types.JID) *waE2E.Message {
		called[jid] = true
		if jid.Device == 0 {
			return primary
		}
		return nil
	}
	variants, err := marshalPerDeviceMessages(recipient, devices, own, ownLID, hook)
	if err != nil {
		t.Fatal(err)
	}
	raw, dsm, err := marshalMessage(recipient, base)
	if err != nil {
		t.Fatal(err)
	}
	for _, jid := range devices {
		data := selectDevicePlaintext(jid, own, ownLID, raw, dsm, variants)
		var msg waE2E.Message
		if err = proto.Unmarshal(data, &msg); err != nil {
			t.Fatal(err)
		}
		switch {
		case isOwnMessageDevice(jid, own, ownLID):
			if called[jid] || msg.DeviceSentMessage == nil || !proto.Equal(msg.DeviceSentMessage.Message, base) || msg.DeviceSentMessage.GetDestinationJID() != recipient.String() {
				t.Fatal("own device did not receive base DSM")
			}
		case jid.Device == 0:
			if !proto.Equal(&msg, primary) || bytes.Equal(data, raw) {
				t.Fatal("primary not overridden")
			}
		default:
			if !proto.Equal(&msg, base) {
				t.Fatal("linked did not receive base")
			}
		}
	}
	// A hook cannot override own DSM even if a caller supplies a conflicting map.
	variants[ownPhone] = []byte("wrong")
	if !bytes.Equal(selectDevicePlaintext(ownPhone, own, ownLID, raw, dsm, variants), dsm) {
		t.Fatal("own override won")
	}
	if _, err = marshalPerDeviceMessages(types.NewJID("group", types.GroupServer), devices, own, ownLID, hook); err == nil {
		t.Fatal("group accepted")
	}
	if participantListHashV2(devices) != participantListHashV2([]types.JID{own, ownLinked, ownPhone, linked, recipient}) {
		t.Fatal("hash depends on order")
	}
}

func TestPerDeviceNilHookPreservesOriginal(t *testing.T) {
	variants, err := marshalPerDeviceMessages(types.EmptyJID, nil, types.EmptyJID, types.EmptyJID, nil)
	if err != nil || variants != nil {
		t.Fatal("nil hook changed behavior")
	}
	jid := types.NewJID("200", types.DefaultUserServer)
	if string(selectDevicePlaintext(jid, types.EmptyJID, types.EmptyJID, []byte("base"), nil)) != "base" {
		t.Fatal("base changed")
	}
}

func TestPerDeviceRetryAndHostedOwnDevices(t *testing.T) {
	own := types.NewADJID("100", 0, 9)
	ownLID := types.NewJID("101", types.HiddenUserServer)
	recipient := types.NewJID("200", types.HiddenUserServer)
	base := &waE2E.Message{Conversation: proto.String("base")}
	primary := &waE2E.Message{Conversation: proto.String("primary")}
	rm := &RecentMessage{wa: base, perDeviceMessage: func(jid types.JID) *waE2E.Message {
		if jid.Device == 0 {
			return primary
		}
		return nil
	}}
	if !proto.Equal(rm.forDeviceRetry(recipient, own, ownLID, false).wa, primary) {
		t.Fatal("primary retry lost variant")
	}
	if !proto.Equal(rm.wa, base) {
		t.Fatal("retry mutated cache")
	}
	linked := recipient
	linked.Device = 2
	for _, jid := range []types.JID{linked, own, types.NewJID("100", types.HostedServer), types.NewJID("101", types.HostedLIDServer)} {
		if !proto.Equal(rm.forDeviceRetry(jid, own, ownLID, false).wa, base) {
			t.Fatal("linked/own retry changed")
		}
	}
	if !proto.Equal(rm.forDeviceRetry(recipient, own, ownLID, true).wa, base) {
		t.Fatal("group/DSM retry overridden")
	}
}
