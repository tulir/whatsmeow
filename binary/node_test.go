package binary

import (
	"fmt"
	"github.com/Rhymen/go-whatsapp/binary/proto"
	"reflect"
	"testing"
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

		msg.Status = new(proto.WebMessageInfo_WEB_MESSAGE_INFO_STATUS)
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
		Description: "action",
		Attributes:  make(map[string]string),
	}
	node.Attributes["add"] = "before"
	node.Attributes["last"] = "true"
	content := make([]interface{}, 1)
	content[0] = msg
	node.Content = content

	b, err := Marshal(*node)
	if err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}

	ret, err := Unmarshal(b)
	if err != nil {
		t.Errorf("%v", err)
		t.Fail()
	}

	fmt.Printf("%v\n", node)
	fmt.Printf("%v\n", ret)

	if node.Description != ret.Description {
		t.Errorf("description changed")
		t.Fail()
	}

	if !reflect.DeepEqual(node.Attributes, ret.Attributes) {
		t.Errorf("attributes changed")
		t.Fail()
	}
	if fmt.Sprintf("%v", node.Content) != fmt.Sprintf("%v", ret.Content) {
		t.Errorf("content changed")
		t.Fail()
	}
}
