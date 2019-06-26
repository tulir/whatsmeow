package whatsapp

import (
	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/binary/proto"
	"log"
	"time"
)

const strNotFoundError = "server responded with 404"

type MessageOffsetInfo struct {
	FirstMessageId    string
	FirstMessageOwner bool
}

func decodeMessages(n *binary.Node) []*proto.WebMessageInfo {

	var messages = make([]*proto.WebMessageInfo, 0)

	if n == nil || n.Attributes == nil || n.Content == nil {
		return messages
	}

	for _, msg := range n.Content.([]interface{}) {
		switch msg.(type) {
		case *proto.WebMessageInfo:
			messages = append(messages, msg.(*proto.WebMessageInfo))
		default:
			log.Println("decodeMessages: Non WebMessage encountered")
		}
	}

	return messages
}

// owner = search for owner's message; if handlers == nil the func will use default handlers
func (wac *Conn) LoadChatHistoryBefore(jid string, count int, messageId string, owner bool, handlers ...Handler) {
	if count <= 0 {
		return
	}

	if handlers == nil {
		handlers = wac.handler
	}

	strOwner := "false"
	if owner {
		strOwner = "true"
	}

	node, err := wac.query("message", jid, messageId, "before", strOwner, "", count, 0)
	if err != nil {
		wac.handleWithCustomHandlers(err, handlers)
	}

	for _, msg := range decodeMessages(node) {
		wac.handleWithCustomHandlers(ParseProtoMessage(msg), handlers)
		wac.handleWithCustomHandlers(msg, handlers)
	}

}

// chunkSize = how many messages to load with one query; if handlers == nil the func will use default handlers;
// pauseBetweenQueries = how much time to sleep between queries
func (wac *Conn) LoadFullChatHistory(jid string, chunkSize int,
	pauseBetweenQueries time.Duration, handlers ...Handler) {
	if chunkSize <= 0 {
		return
	}

	if handlers == nil {
		handlers = wac.handler
	}

	beforeMsg := ""
	beforeMsgIsOwner := "true"

	for {
		node, err := wac.query("message", jid, beforeMsg, "before", beforeMsgIsOwner, "", chunkSize, 0)

		if err != nil {
			wac.handleWithCustomHandlers(err, handlers)
		} else {

			msgs := decodeMessages(node)
			for _, msg := range msgs {
				wac.handleWithCustomHandlers(ParseProtoMessage(msg), handlers)
				wac.handleWithCustomHandlers(msg, handlers)
			}

			if len(msgs) == 0 {
				break
			}

			beforeMsg = *msgs[0].Key.Id
			beforeMsgIsOwner = "false"
			if *msgs[0].Key.FromMe {
				beforeMsgIsOwner = "true"
			}

		}

		<-time.After(pauseBetweenQueries)

	}

}

func (wac *Conn) LoadFullChatHistoryAfter(jid string, messageId string, chunkSize int,
	pauseBetweenQueries time.Duration, handlers ...Handler) {

	if chunkSize <= 0 {
		return
	}

	if handlers == nil {
		handlers = wac.handler
	}

	msgOwner := "true"

	for {
		node, err := wac.query("message", jid, messageId, "after", msgOwner, "", chunkSize, 0)

		if err != nil {
			if err.Error() == strNotFoundError && msgOwner == "true" {
				// reverse initial msgOwner value and retry
				msgOwner = "false"
				<-time.After(time.Second)
				continue
			}

			wac.handleWithCustomHandlers(err, handlers)
		} else {

			msgs := decodeMessages(node)
			for _, msg := range msgs {
				wac.handleWithCustomHandlers(ParseProtoMessage(msg), handlers)
				wac.handleWithCustomHandlers(msg, handlers)
			}

			if len(msgs) != chunkSize {
				break
			}

			messageId = *msgs[0].Key.Id
			msgOwner = "false"
			if *msgs[0].Key.FromMe {
				msgOwner = "true"
			}

		}

		<-time.After(pauseBetweenQueries)

	}

}
