package whatsapp

import (
	"github.com/Rhymen/go-whatsapp/binary"
	"strings"
)

type Store struct {
	Contacts map[string]Contact
}

type Contact struct {
	Jid    string
	Notify string
	Name   string
	Short  string
}

func newStore() *Store {
	return &Store{
		make(map[string]Contact),
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
