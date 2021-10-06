package binary

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Rhymen/go-whatsapp/binary/proto"
)

func TestMarshal(t *testing.T) {
	msg := new(proto.WebMessageInfo)

	{
		msg.MessageTimestamp = new(uint64)
		*msg.MessageTimestamp = 1529341084

		msg.Message = &proto.Message{
			Conversation: new(string),
		}
		*msg.Message.Conversation = "Testnachricht."

		msg.Status = new(proto.WebMessageInfo_WebMessageInfoStatus)
		*msg.Status = proto.WebMessageInfo_ERROR

		msg.Key = &proto.MessageKey{
			RemoteJid: new(string),
			FromMe:    new(bool),
			Id:        new(string),
		}
		*msg.Key.RemoteJid = "491786943536-1375979218@g.us"
		*msg.Key.FromMe = true
		*msg.Key.Id = "48386F14A1D358101F4B695DEBEBCA83"
	}

	node := &Node{
		Tag:              "action",
		LegacyAttributes: make(map[string]string),
	}
	node.LegacyAttributes["add"] = "before"
	node.LegacyAttributes["last"] = "true"
	content := make([]interface{}, 1)
	content[0] = msg
	node.Content = content

	b, err := Marshal(*node, false)
	if err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}

	ret, err := Unmarshal(b, false)
	if err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}

	fmt.Printf("%v\n", node)
	fmt.Printf("%v\n", ret)

	if node.Tag != ret.Tag {
		t.Errorf("description changed")
		t.Fail()
	}

	if !reflect.DeepEqual(node.LegacyAttributes, ret.LegacyAttributes) {
		t.Errorf("attributes changed")
		t.Fail()
	}
	if fmt.Sprintf("%v", node.Content) != fmt.Sprintf("%v", ret.Content) {
		t.Errorf("content changed")
		t.Fail()
	}
}
