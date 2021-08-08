package whatsapp

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Rhymen/go-whatsapp/binary"
)

type Store struct {
	Contacts     map[JID]Contact
	ContactsLock sync.RWMutex
	Chats        map[JID]Chat
	ChatsLock    sync.RWMutex
}

type Contact struct {
	JID    JID
	Notify string
	Name   string
	Short  string
	Source map[string]string
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
	ReceivedAt      time.Time
}

func parseChat(attributes map[string]string) (out Chat) {
	var err error
	out.JID = strings.Replace(attributes["jid"], OldUserSuffix, NewUserSuffix, 1)
	out.Name = attributes["name"]
	out.ModifyTag = attributes["modify_tag"]
	out.UnreadCount, err = strconv.Atoi(attributes["count"])
	if err != nil {
		out.UnreadCount = -1
	}
	out.LastMessageTime, _ = strconv.ParseInt(attributes["t"], 10, 64)
	out.MutedUntil, _ = strconv.ParseInt(attributes["mute"], 10, 64)
	out.IsMarkedSpam, _ = strconv.ParseBool(attributes["spam"])
	out.IsArchived, _ = strconv.ParseBool(attributes["archive"])
	_, out.IsPinned = attributes["pin"]
	out.Source = attributes
	out.ReceivedAt = time.Now()
	return
}

func newStore() *Store {
	return &Store{
		Contacts: make(map[string]Contact),
		Chats:    make(map[string]Chat),
	}
}

func (wac *Conn) updateContacts(contacts interface{}) {
	c, ok := contacts.([]interface{})
	if !ok {
		return
	}

	wac.Store.ContactsLock.Lock()
	for _, contact := range c {
		contactNode, ok := contact.(binary.Node)
		if !ok {
			continue
		}

		jid := strings.Replace(contactNode.Attributes["jid"], "@c.us", "@s.whatsapp.net", 1)
		wac.Store.Contacts[jid] = Contact{
			JID:    jid,
			Notify: contactNode.Attributes["notify"],
			Name:   contactNode.Attributes["name"],
			Short:  contactNode.Attributes["short"],
			Source: contactNode.Attributes,
		}
	}
	wac.Store.ContactsLock.Unlock()
}

func (wac *Conn) updateChats(chats interface{}) {
	c, ok := chats.([]interface{})
	if !ok {
		return
	}

	wac.Store.ChatsLock.Lock()
	for _, chat := range c {
		chatNode, ok := chat.(binary.Node)
		if !ok {
			continue
		}
		parsedChat := parseChat(chatNode.Attributes)
		wac.Store.Chats[parsedChat.JID] = parsedChat
	}
	wac.Store.ChatsLock.Unlock()
}
