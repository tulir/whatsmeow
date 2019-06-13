package whatsapp

import (
	"fmt"
	"os"
	"strings"

	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/binary/proto"
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

/**
The RawMessageHandler interface needs to be implemented to receive raw messages dispatched by the dispatcher.
Raw messages are the raw protobuf structs instead of the easy-to-use structs in TextMessageHandler, ImageMessageHandler, etc..
*/
type RawMessageHandler interface {
	Handler
	HandleRawMessage(message *proto.WebMessageInfo)
}

/**
The ContactListHandler interface needs to be implemented to applky custom actions to contact lists dispatched by the dispatcher.
*/
type ContactListHandler interface {
	Handler
	HandleContactList(contacts []Contact)
}

/**
The ChatListHandler interface needs to be implemented to apply custom actions to chat lists dispatched by the dispatcher.
*/
type ChatListHandler interface {
	Handler
	HandleChatList(contacts []Chat)
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

// RemoveHandler removes a handler from the list of handlers that receive dispatched messages.
func (wac *Conn) RemoveHandler(handler Handler) bool {
	i := -1
	for k, v := range wac.handler {
		if v == handler {
			i = k
			break
		}
	}
	if i > -1 {
		wac.handler = append(wac.handler[:i], wac.handler[i+1:]...)
		return true
	}
	return false
}

// RemoveHandlers empties the list of handlers that receive dispatched messages.
func (wac *Conn) RemoveHandlers() {
	wac.handler = make([]Handler, 0)
}

func (wac *Conn) handle(message interface{}) {
	switch m := message.(type) {
	case error:
		for _, h := range wac.handler {
			go h.HandleError(m)
		}
	case string:
		for _, h := range wac.handler {
			if x, ok := h.(JsonMessageHandler); ok {
				go x.HandleJsonMessage(m)
			}
		}
	case TextMessage:
		for _, h := range wac.handler {
			if x, ok := h.(TextMessageHandler); ok {
				go x.HandleTextMessage(m)
			}
		}
	case ImageMessage:
		for _, h := range wac.handler {
			if x, ok := h.(ImageMessageHandler); ok {
				go x.HandleImageMessage(m)
			}
		}
	case VideoMessage:
		for _, h := range wac.handler {
			if x, ok := h.(VideoMessageHandler); ok {
				go x.HandleVideoMessage(m)
			}
		}
	case AudioMessage:
		for _, h := range wac.handler {
			if x, ok := h.(AudioMessageHandler); ok {
				go x.HandleAudioMessage(m)
			}
		}
	case DocumentMessage:
		for _, h := range wac.handler {
			if x, ok := h.(DocumentMessageHandler); ok {
				go x.HandleDocumentMessage(m)
			}
		}
	case *proto.WebMessageInfo:
		for _, h := range wac.handler {
			if x, ok := h.(RawMessageHandler); ok {
				go x.HandleRawMessage(m)
			}
		}
	}

}

func (wac *Conn) handleContacts(contacts interface{}) {
	var contactList []Contact
	c, ok := contacts.([]interface{})
	if !ok {
		return
	}
	for _, contact := range c {
		contactNode, ok := contact.(binary.Node)
		if !ok {
			continue
		}

		jid := strings.Replace(contactNode.Attributes["jid"], "@c.us", "@s.whatsapp.net", 1)
		contactList = append(contactList, Contact{
			jid,
			contactNode.Attributes["notify"],
			contactNode.Attributes["name"],
			contactNode.Attributes["short"],
		})
	}
	for _, h := range wac.handler {
		if x, ok := h.(ContactListHandler); ok {
			go x.HandleContactList(contactList)
		}
	}
}

func (wac *Conn) handleChats(chats interface{}) {
	var chatList []Chat
	c, ok := chats.([]interface{})
	if !ok {
		return
	}
	for _, chat := range c {
		chatNode, ok := chat.(binary.Node)
		if !ok {
			continue
		}

		jid := strings.Replace(chatNode.Attributes["jid"], "@c.us", "@s.whatsapp.net", 1)
		chatList = append(chatList, Chat{
			jid,
			chatNode.Attributes["name"],
			chatNode.Attributes["count"],
			chatNode.Attributes["t"],
			chatNode.Attributes["mute"],
			chatNode.Attributes["spam"],
		})
	}
	for _, h := range wac.handler {
		if x, ok := h.(ChatListHandler); ok {
			go x.HandleChatList(chatList)
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
						wac.handle(v)
						wac.handle(parseProtoMessage(v))
					}
				}
			}
		} else if message.Description == "response" && message.Attributes["type"] == "contacts" {
			wac.updateContacts(message.Content)
			wac.handleContacts(message.Content)
		} else if message.Description == "response" && message.Attributes["type"] == "chat" {
			wac.updateChats(message.Content)
			wac.handleChats(message.Content)
		}
	case error:
		wac.handle(message)
	case string:
		wac.handle(message)
	default:
		fmt.Fprintf(os.Stderr, "unknown type in dipatcher chan: %T", msg)
	}
}
