package whatsapp

import (
	"github.com/Rhymen/go-whatsapp/binary"
	"strings"
)

type Store struct {
	Contacts map[string]Contact
	Chats    map[string]Chat
}

type Contact struct {
	Jid    string
	Notify string
	Name   string
	Short  string
}

type Chat struct {
	Jid             string
	Name            string
	Unread          string
	LastMessageTime string
	IsMuted         string
	IsMarkedSpam    string
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

		jid := strings.Replace(chatNode.Attributes["jid"], "@c.us", "@s.whatsapp.net", 1)
		wac.Store.Chats[jid] = Chat{
			jid,
			chatNode.Attributes["name"],
			chatNode.Attributes["count"],
			chatNode.Attributes["t"],
			chatNode.Attributes["mute"],
			chatNode.Attributes["spam"],
		}
	}
}
