package whatsapp

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Rhymen/go-whatsapp/binary"
)

type Presence string

const (
	PresenceAvailable   Presence = "available"
	PresenceUnavailable Presence = "unavailable"
	PresenceComposing   Presence = "composing"
	PresenceRecording   Presence = "recording"
	PresencePaused      Presence = "paused"
)

type ProfilePicInfo struct {
	URL string `json:"eurl"`
	Tag string `json:"tag"`

	Status int `json:"status"`
}

func (ppi *ProfilePicInfo) Download() (io.ReadCloser, error) {
	resp, err := http.Get(ppi.URL)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (ppi *ProfilePicInfo) DownloadBytes() ([]byte, error) {
	body, err := ppi.Download()
	if err != nil {
		return nil, err
	}
	defer body.Close()
	data, err := ioutil.ReadAll(body)
	return data, err
}

func (wac *Conn) GetProfilePicThumb(jid string) (*ProfilePicInfo, error) {
	data := []interface{}{"query", "ProfilePicThumb", jid}
	resp, err := wac.writeJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to get avatar: %v", err)
	}
	content := <-resp
	info := &ProfilePicInfo{}
	err = json.Unmarshal([]byte(content), info)
	if err != nil {
		return info, fmt.Errorf("failed to unmarshal avatar info: %v", err)
	}
	return info, nil
}

func (wac *Conn) GetStatus(jid string) (<-chan string, error) {
	data := []interface{}{"query", "Status", jid}
	return wac.writeJSON(data)
}

func (wac *Conn) SubscribePresence(jid string) (<-chan string, error) {
	data := []interface{}{"action", "presence", "subscribe", jid}
	return wac.writeJSON(data)
}

func (wac *Conn) Search(search string, count, page int) (*binary.Node, error) {
	return wac.query("search", "", "", "", "", search, count, page)
}

func (wac *Conn) LoadMessages(jid string, count int) (*binary.Node, error) {
	return wac.query("message", jid, "", "before", "true", "", count, 0)
}

func (wac *Conn) LoadMessagesBefore(jid, messageId string, fromMe bool, count int) (*binary.Node, error) {
	return wac.query("message", jid, messageId, "before", strconv.FormatBool(fromMe), "", count, 0)
}

func (wac *Conn) LoadMessagesAfter(jid, messageId string, fromMe bool, count int) (*binary.Node, error) {
	return wac.query("message", jid, messageId, "after", strconv.FormatBool(fromMe), "", count, 0)
}

func (wac *Conn) LoadMediaInfo(jid, messageId string, fromMe bool) (*binary.Node, error) {
	return wac.query("media", jid, messageId, "", strconv.FormatBool(fromMe), "", 0, 0)
}

func (wac *Conn) Presence(jid string, presence Presence) (<-chan string, error) {
	ts := time.Now().Unix()
	tag := fmt.Sprintf("%d.--%d", ts, wac.msgCount)

	content := binary.Node{
		Description: "presence",
		Attributes: map[string]string{
			"type": string(presence),
		},
	}
	switch presence {
	case PresenceComposing:
		fallthrough
	case PresenceRecording:
		fallthrough
	case PresencePaused:
		content.Attributes["to"] = jid
	}

	n := binary.Node{
		Description: "action",
		Attributes: map[string]string{
			"type":  "set",
			"epoch": strconv.Itoa(wac.msgCount),
		},
		Content: []interface{}{content},
	}

	return wac.writeBinary(n, group, ignore, tag)
}

func (wac *Conn) Exist(jid string) (<-chan string, error) {
	data := []interface{}{"query", "exist", jid}
	return wac.writeJSON(data)
}

func (wac *Conn) Emoji() (*binary.Node, error) {
	return wac.query("emoji", "", "", "", "", "", 0, 0)
}

func (wac *Conn) Contacts() (*binary.Node, error) {
	node, err := wac.query("contacts", "", "", "", "", "", 0, 0)
	if node != nil && node.Description == "response" && node.Attributes["type"] == "contacts" {
		wac.updateContacts(node.Content)
	}
	return node, err
}

func (wac *Conn) Chats() (*binary.Node, error) {
	node, err := wac.query("chat", "", "", "", "", "", 0, 0)
	if node != nil && node.Description == "response" && node.Attributes["type"] == "chat" {
		wac.updateChats(node.Content)
	}
	return node, err
}

func (wac *Conn) Read(jid JID, id MessageID) (<-chan string, error) {
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

func (wac *Conn) query(t string, jid JID, messageId MessageID, kind, owner, search string, count, page int) (*binary.Node, error) {
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

	metric := group
	if t == "media" {
		metric = queryMedia
	}

	ch, err := wac.writeBinary(n, metric, ignore, tag)
	if err != nil {
		return nil, err
	}

	select {
	case response := <-ch:
		msg, err := wac.decryptBinaryMessage([]byte(response))
		if err != nil {
			return nil, err
		}

		//TODO: use parseProtoMessage
		return msg, nil
	case <-time.After(3 * time.Minute):
		return nil, ErrQueryTimeout
	}
}

func (wac *Conn) setGroup(t string, jid JID, subject string, participants []string) (<-chan string, error) {
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

func buildParticipantNodes(participants []JID) []binary.Node {
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

func (wac *Conn) BlockContact(jid JID) (<-chan string, error) {
	return wac.handleBlockContact("add", jid)
}

func (wac *Conn) UnblockContact(jid JID) (<-chan string, error) {
	return wac.handleBlockContact("remove", jid)
}

func (wac *Conn) handleBlockContact(action string, jid JID) (<-chan string, error) {
	ts := time.Now().Unix()
	tag := fmt.Sprintf("%d.--%d", ts, wac.msgCount)

	netsplit := strings.Split(jid, "@")
	cusjid := netsplit[0] + "@c.us"

	n := binary.Node{
		Description: "action",
		Attributes: map[string]string{
			"type":  "set",
			"epoch": strconv.Itoa(wac.msgCount),
		},
		Content: []interface{}{
			binary.Node{
				Description: "block",
				Attributes: map[string]string{
					"type": action,
				},
				Content: []binary.Node{
					{
						Description: "user",
						Attributes: map[string]string{
							"jid": cusjid,
						},
						Content: nil,
					},
				},
			},
		},
	}

	return wac.writeBinary(n, contact, ignore, tag)
}
