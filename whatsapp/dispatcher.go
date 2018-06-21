package whatsapp

import (
	"fmt"
	"github.com/Rhymen/go-whatsapp/whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/whatsapp/binary/proto"
	"os"
)

type dispatcher struct {
	toDispatch chan interface{}
	handler    []Handler
}

func newDispatcher() *dispatcher {
	return &dispatcher{
		toDispatch: make(chan interface{}),
	}
}

func (dp *dispatcher) dispatch() {
	for {
		msg := <-dp.toDispatch
		if msg == nil || len(dp.handler) == 0 {
			continue
		}
		switch message := msg.(type) {
		case *binary.Node:
			if message.Description == "action" {
				if con, ok := message.Content.([]interface{}); ok {
					for a := range con {
						if v, ok := con[a].(*proto.WebMessageInfo); ok {
							dp.dispatchProtoMessage(v)
						}
					}
				}
			}
		case error:
			dp.handle(message)
		default:
			fmt.Fprintf(os.Stderr, "unknown type in dipatcher chan: %T", msg)
		}
	}
}

func (dp *dispatcher) dispatchProtoMessage(msg *proto.WebMessageInfo) {
	switch {

	case msg.GetMessage().GetAudioMessage() != nil:
		//dp.handle(getAudioMessage(msg))

	case msg.GetMessage().GetImageMessage() != nil:
		dp.handle(getImageMessage(msg))

	case msg.GetMessage().GetVideoMessage() != nil:
		//dp.handle(getVideoMessage(msg))

	case msg.GetMessage().GetConversation() != "":
		dp.handle(getTextMessage(msg))

	default:
		//cannot match message
	}
}
