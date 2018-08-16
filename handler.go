package whatsapp

import (
	"fmt"
	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/binary/proto"
	"os"
)

/*
The Handler interface is the minimal interface that needs to be implemented
to be accepted as a valid handler for our dispatching system.
The minimal handler is used to dispatch error messages. These errors occur on unexpected behavior by the websocket
connection or if we are unable to handle or interpret an incoming message. Error produced by user actions are not
dispatched through this handler. They are returned as an error on the specific function call.
*/
type Handler interface {
	HandleError(err error)
}

/*
The TextMessageHandler interface needs to be implemented to receive text messages dispatched by the dispatcher.
*/
type TextMessageHandler interface {
	Handler
	HandleTextMessage(message TextMessage)
}

/*
The ImageMessageHandler interface needs to be implemented to receive image messages dispatched by the dispatcher.
*/
type ImageMessageHandler interface {
	Handler
	HandleImageMessage(message ImageMessage)
}

/*
The VideoMessageHandler interface needs to be implemented to receive video messages dispatched by the dispatcher.
*/
type VideoMessageHandler interface {
	Handler
	HandleVideoMessage(message VideoMessage)
}

/*
The AudioMessageHandler interface needs to be implemented to receive audio messages dispatched by the dispatcher.
*/
type AudioMessageHandler interface {
	Handler
	HandleAudioMessage(message AudioMessage)
}

/*
The DocumentMessageHandler interface needs to be implemented to receive document messages dispatched by the dispatcher.
*/
type DocumentMessageHandler interface {
	Handler
	HandleDocumentMessage(message DocumentMessage)
}

/*
The JsonMessageHandler interface needs to be implemented to receive json messages dispatched by the dispatcher.
These json messages contain status updates of every kind sent by WhatsAppWeb servers. WhatsAppWeb uses these messages
to built a Store, which is used to save these "secondary" information. These messages may contain
presence (available, last see) information, or just the battery status of your phone.
*/
type JsonMessageHandler interface {
	Handler
	HandleJsonMessage(message string)
}

/*
AddHandler adds an handler to the list of handler that receive dispatched messages.
The provided handler must at least implement the Handler interface. Additionally implemented
handlers(TextMessageHandler, ImageMessageHandler) are optional. At runtime it is checked if they are implemented
and they are called if so and needed.
*/
func (wac *Conn) AddHandler(handler Handler) {
	wac.handler = append(wac.handler, handler)
}

func (wac *Conn) handle(message interface{}) {
	switch m := message.(type) {
	case error:
		for _, h := range wac.handler {
			go h.HandleError(m)
		}
	case string:
		for _, h := range wac.handler {
			x, ok := h.(JsonMessageHandler)
			if !ok {
				continue
			}
			go x.HandleJsonMessage(m)
		}
	case TextMessage:
		for _, h := range wac.handler {
			x, ok := h.(TextMessageHandler)
			if !ok {
				continue
			}
			go x.HandleTextMessage(m)
		}
	case ImageMessage:
		for _, h := range wac.handler {
			x, ok := h.(ImageMessageHandler)
			if !ok {
				continue
			}
			go x.HandleImageMessage(m)
		}
	case VideoMessage:
		for _, h := range wac.handler {
			x, ok := h.(VideoMessageHandler)
			if !ok {
				continue
			}
			go x.HandleVideoMessage(m)
		}
	case AudioMessage:
		for _, h := range wac.handler {
			x, ok := h.(AudioMessageHandler)
			if !ok {
				continue
			}
			go x.HandleAudioMessage(m)
		}
	case DocumentMessage:
		for _, h := range wac.handler {
			x, ok := h.(DocumentMessageHandler)
			if !ok {
				continue
			}
			go x.HandleDocumentMessage(m)
		}
	}

}

func (wac *Conn) dispatch(msg interface{}) {
	if msg == nil {
		return
	}

	switch message := msg.(type) {
	case *binary.Node:
		if message.Description == "action" {
			if con, ok := message.Content.([]interface{}); ok {
				for a := range con {
					if v, ok := con[a].(*proto.WebMessageInfo); ok {
						wac.handle(parseProtoMessage(v))
					}
				}
			}
		} else if message.Description == "response" && message.Attributes["type"] == "contacts" {
			wac.updateContacts(message.Content)
		}
	case error:
		wac.handle(message)
	case string:
		wac.handle(message)
	default:
		fmt.Fprintf(os.Stderr, "unknown type in dipatcher chan: %T", msg)
	}
}
