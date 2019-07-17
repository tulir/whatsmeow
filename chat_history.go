package whatsapp

import (
	"github.com/Rhymen/go-whatsapp/binary"
	"github.com/Rhymen/go-whatsapp/binary/proto"
	"log"
	"strconv"
	"time"
)

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

// LoadChatMessages is useful to "scroll" messages, loading by count at a time
// if handlers == nil the func will use default handlers
// if after == true LoadChatMessages will load messages after the specified messageId, otherwise it will return
// message before the messageId
func (wac *Conn) LoadChatMessages(jid string, count int, messageId string, owner bool, after bool, handlers ...Handler) error {
	if count <= 0 {
		return nil
	}

	if handlers == nil {
		handlers = wac.handler
	}

	kind := "before"
	if after {
		kind = "after"
	}

	node, err := wac.query("message", jid, messageId, kind,
		strconv.FormatBool(owner), "", count, 0)

	if err != nil {
		wac.handleWithCustomHandlers(err, handlers)
		return err
	}

	for _, msg := range decodeMessages(node) {
		wac.handleWithCustomHandlers(ParseProtoMessage(msg), handlers)
		wac.handleWithCustomHandlers(msg, handlers)
	}
	return nil

}

// LoadFullChatHistory loads full chat history for the given jid
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
	beforeMsgIsOwner := true

	for {
		node, err := wac.query("message", jid, beforeMsg, "before",
			strconv.FormatBool(beforeMsgIsOwner), "", chunkSize, 0)

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
			beforeMsgIsOwner = msgs[0].Key.FromMe != nil && *msgs[0].Key.FromMe
		}

		<-time.After(pauseBetweenQueries)

	}

}

// LoadFullChatHistoryAfter loads all messages after the specified messageId
// useful to "catch up" with the message history after some specified message
func (wac *Conn) LoadFullChatHistoryAfter(jid string, messageId string, chunkSize int,
	pauseBetweenQueries time.Duration, handlers ...Handler) {

	if chunkSize <= 0 {
		return
	}

	if handlers == nil {
		handlers = wac.handler
	}

	msgOwner := true
	prevNotFound := false

	for {
		node, err := wac.query("message", jid, messageId, "after",
			strconv.FormatBool(msgOwner), "", chunkSize, 0)

		if err != nil {

			// Whatsapp will return 404 status when there is wrong owner flag on the requested message id
			if err == ErrServerRespondedWith404 {

				// this will detect two consecutive "not found" errors.
				// this is done to prevent infinite loop when wrong message id supplied
				if prevNotFound {
					log.Println("LoadFullChatHistoryAfter: could not retrieve any messages, wrong message id?")
					return
				}
				prevNotFound = true

				// try to reverse the owner flag and retry
				if msgOwner {
					// reverse initial msgOwner value and retry
					msgOwner = false

					<-time.After(time.Second)
					continue
				}

			}

			// if the error isn't a 404 error, pass it to the error handler
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
			msgOwner = msgs[0].Key.FromMe != nil && *msgs[0].Key.FromMe
		}

		// message was found
		prevNotFound = false

		<-time.After(pauseBetweenQueries)

	}

}
