package whatsapp

import (
	"strconv"
	"strings"

	"github.com/Rhymen/go-whatsapp/binary"
)

type Store struct {
	Contacts map[JID]Contact
	Chats    map[JID]Chat
}

type Contact struct {
	JID    JID
	Notify string
	Name   string
	Short  string
}

type Chat struct {
	JID             JID
	Name            string
	ModifyTag       string
	UnreadCount     int
	LastMessageTime int64
	MutedUntil      int64
	IsMarkedSpam    bool
	IsArchived      bool
	IsPinned        bool
	Source          map[string]string
}

func parseChat(attributes map[string]string) (out Chat) {
	out.JID = strings.Replace(attributes["jid"], OldUserSuffix, NewUserSuffix, 1)
	out.Name = attributes["name"]
	out.ModifyTag = attributes["modify_tag"]
	out.UnreadCount, _ = strconv.Atoi(attributes["count"])
	out.LastMessageTime, _ = strconv.ParseInt(attributes["t"], 10, 64)
	out.MutedUntil, _ = strconv.ParseInt(attributes["mute"], 10, 64)
	out.IsMarkedSpam, _ = strconv.ParseBool(attributes["spam"])
	out.IsArchived, _ = strconv.ParseBool(attributes["archive"])
	_, out.IsPinned = attributes["pin"]
	out.Source = attributes
	return
}

func newStore() *Store {
	return &Store{
		make(map[string]Contact),
		make(map[string]Chat),
	}
}

func (wac *Conn) updateContacts(contacts interface{}) {
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
		wac.Store.Contacts[jid] = Contact{
			jid,
			contactNode.Attributes["notify"],
			contactNode.Attributes["name"],
			contactNode.Attributes["short"],
		}
	}
}

func (wac *Conn) updateChats(chats interface{}) {
	c, ok := chats.([]interface{})
	if !ok {
		return
	}

	for _, chat := range c {
		chatNode, ok := chat.(binary.Node)
		if !ok {
			continue
		}
		parsedChat := parseChat(chatNode.Attributes)
		wac.Store.Chats[parsedChat.JID] = parsedChat
	}
}
