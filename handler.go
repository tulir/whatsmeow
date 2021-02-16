package whatsapp

import (
	"encoding/json"
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
	HandleEvent(event interface{})
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
	defer func() {
		if errIfc := recover(); errIfc != nil {
			if err, ok := errIfc.(error); ok {
				wac.unsafeHandle(fmt.Errorf("panic in WhatsApp handler: %w", err))
			} else {
				wac.unsafeHandle(fmt.Errorf("panic in WhatsApp handler: %v", errIfc))
			}
		}
	}()
	wac.unsafeHandle(message)
}

func (wac *Conn) unsafeHandle(message interface{}) {
	wac.handleWithCustomHandlers(message, wac.handler)
}

func (wac *Conn) handleWithCustomHandlers(message interface{}, handlers []Handler) {
	if message == ErrMessageTypeNotImplemented {
		return
	}
	for _, h := range handlers {
		h.HandleEvent(message)
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
	wac.unsafeHandle(contactList)
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
	wac.unsafeHandle(chatList)
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
						wac.handle(ParseProtoMessage(v))
					}

					if v, ok := con[a].(binary.Node); ok {
						wac.handle(ParseNodeMessage(v))
					}
				}
			} else if con, ok := message.Content.([]binary.Node); ok {
				for a := range con {
					wac.handle(ParseNodeMessage(con[a]))
				}
			} else {
				wac.handle(message)
			}
		} else if message.Description == "response" && message.Attributes["type"] == "contacts" {
			wac.updateContacts(message.Content)
			wac.handleContacts(message.Content)
		} else if message.Description == "response" && message.Attributes["type"] == "chat" {
			wac.updateChats(message.Content)
			wac.handleChats(message.Content)
		} else {
			wac.handle(message)
		}
	case error:
		wac.handle(message)
	case string:
		wac.handleJSONMessage(message)
		wac.handle(json.RawMessage(message))
	default:
		fmt.Fprintf(os.Stderr, "unknown type in dipatcher chan: %T", msg)
	}
}
