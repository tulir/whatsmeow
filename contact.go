package whatsapp

import (
	"fmt"
	"github.com/Rhymen/go-whatsapp/binary"
	"strconv"
	"time"
)

type Presence string

const (
	PresenceAvailable   = "available"
	PresenceUnavailable = "unavailable"
	PresenceComposing   = "composing"
)

//TODO: filename? WhatsApp uses Store.Contacts for these functions
//TODO: functions probably shouldn't return a string, maybe build a struct / return json
//TODO: check for further queries
func (wac *Conn) GetProfilePicThumb(jid string) (<-chan string, error) {
	data := []interface{}{"query", "ProfilePicThumb", jid}
	return wac.write(data)
}

func (wac *Conn) GetStatus(jid string) (<-chan string, error) {
	data := []interface{}{"query", "Status", jid}
	return wac.write(data)
}

func (wac *Conn) GetGroupMetaData(jid string) (<-chan string, error) {
	data := []interface{}{"query", "GroupMetadata", jid}
	return wac.write(data)
}

func (wac *Conn) SubscribePresence(jid string) (<-chan string, error) {
	data := []interface{}{"action", "presence", "subscribe", jid}
	return wac.write(data)
}

func (wac *Conn) CreateGroup(subject string, participants []string) (<-chan string, error) {
	return wac.setGroup("create", "", subject, participants)
}

func (wac *Conn) UpdateGroupSubject(subject string, jid string) (<-chan string, error) {
	return wac.setGroup("subject", jid, subject, nil)
}

func (wac *Conn) SetAdmin(jid string, participants []string) (<-chan string, error) {
	return wac.setGroup("promote", jid, "", participants)
}

func (wac *Conn) RemoveAdmin(jid string, participants []string) (<-chan string, error) {
	return wac.setGroup("demote", jid, "", participants)
}

func (wac *Conn) AddMember(jid string, participants []string) (<-chan string, error) {
	return wac.setGroup("add", jid, "", participants)
}

func (wac *Conn) RemoveMember(jid string, participants []string) (<-chan string, error) {
	return wac.setGroup("remove", jid, "", participants)
}

func (wac *Conn) LeaveGroup(jid string) (<-chan string, error) {
	return wac.setGroup("leave", jid, "", nil)
}

func (wac *Conn) Search(search string, count, page int) (*binary.Node, error) {
	return wac.query("search", "", "", "", "", search, count, page)
}

func (wac *Conn) LoadMessages(jid, messageId string, count int) (*binary.Node, error) {
	return wac.query("message", jid, "", "before", "true", "", count, 0)
}

func (wac *Conn) LoadMessagesBefore(jid, messageId string, count int) (*binary.Node, error) {
	return wac.query("message", jid, messageId, "before", "true", "", count, 0)
}

func (wac *Conn) LoadMessagesAfter(jid, messageId string, count int) (*binary.Node, error) {
	return wac.query("message", jid, messageId, "after", "true", "", count, 0)
}

func (wac *Conn) Presence(jid string, presence Presence) (<-chan string, error) {
	ts := time.Now().Unix()
	tag := fmt.Sprintf("%d.--%d", ts, wac.msgCount)

	n := binary.Node{
		Description: "action",
		Attributes: map[string]string{
			"type":  "set",
			"epoch": strconv.Itoa(wac.msgCount),
		},
		Content: []interface{}{binary.Node{
			Description: "presence",
			Attributes: map[string]string{
				"type": string(presence),
			},
		}},
	}

	return wac.writeBinary(n, group, ignore, tag)
}

func (wac *Conn) Emoji() (*binary.Node, error) {
	return wac.query("emoji", "", "", "", "", "", 0, 0)
}

func (wac *Conn) Contacts() (*binary.Node, error) {
	return wac.query("contacts", "", "", "", "", "", 0, 0)
}

func (wac *Conn) Chats() (*binary.Node, error) {
	return wac.query("chat", "", "", "", "", "", 0, 0)
}

func (wac *Conn) Read(jid, id string) (<-chan string, error) {
	ts := time.Now().Unix()
	tag := fmt.Sprintf("%d.--%d", ts, wac.msgCount)

	n := binary.Node{
		Description: "action",
		Attributes: map[string]string{
			"type":  "set",
			"epoch": strconv.Itoa(wac.msgCount),
		},
		Content: []interface{}{binary.Node{
			Description: "read",
			Attributes: map[string]string{
				"count": "1",
				"index": id,
				"jid":   jid,
				"owner": "false",
			},
		}},
	}

	return wac.writeBinary(n, group, ignore, tag)
}

func (wac *Conn) query(t, jid, messageId, kind, owner, search string, count, page int) (*binary.Node, error) {
	ts := time.Now().Unix()
	tag := fmt.Sprintf("%d.--%d", ts, wac.msgCount)

	n := binary.Node{
		Description: "query",
		Attributes: map[string]string{
			"type":  t,
			"epoch": strconv.Itoa(wac.msgCount),
		},
	}

	if jid != "" {
		n.Attributes["jid"] = jid
	}

	if messageId != "" {
		n.Attributes["index"] = messageId
	}

	if kind != "" {
		n.Attributes["kind"] = kind
	}

	if owner != "" {
		n.Attributes["owner"] = owner
	}

	if search != "" {
		n.Attributes["search"] = search
	}

	if count != 0 {
		n.Attributes["count"] = strconv.Itoa(count)
	}

	if page != 0 {
		n.Attributes["page"] = strconv.Itoa(page)
	}

	ch, err := wac.writeBinary(n, group, ignore, tag)
	if err != nil {
		return nil, err
	}

	msg, err := wac.decryptBinaryMessage([]byte(<-ch))
	if err != nil {
		return nil, err
	}

	//TODO: use parseProtoMessage
	return msg, nil
}

func (wac *Conn) setGroup(t, jid, subject string, participants []string) (<-chan string, error) {
	ts := time.Now().Unix()
	tag := fmt.Sprintf("%d.--%d", ts, wac.msgCount)

	//TODO: get proto or improve encoder to handle []interface{}

	p := buildParticipantNodes(participants)

	g := binary.Node{
		Description: "group",
		Attributes: map[string]string{
			"author": wac.session.Wid,
			"id":     tag,
			"type":   t,
		},
		Content: p,
	}

	if jid != "" {
		g.Attributes["jid"] = jid
	}

	if subject != "" {
		g.Attributes["subject"] = subject
	}

	n := binary.Node{
		Description: "action",
		Attributes: map[string]string{
			"type":  "set",
			"epoch": strconv.Itoa(wac.msgCount),
		},
		Content: []interface{}{g},
	}

	return wac.writeBinary(n, group, ignore, tag)
}

func buildParticipantNodes(participants []string) []binary.Node {
	l := len(participants)
	if participants == nil || l == 0 {
		return nil
	}

	p := make([]binary.Node, len(participants))
	for i, participant := range participants {
		p[i] = binary.Node{
			Description: "participant",
			Attributes: map[string]string{
				"jid": participant,
			},
		}
	}
	return p
}
