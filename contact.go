package whatsapp_connection

import (
	"strconv"
	"github.com/Rhymen/go-whatsapp/binary"
	"time"
	"fmt"
)

//TODO: filename? WhatsApp uses Store.Contacts for these functions
//TODO: functions probably shouldn't return a string, maybe build a struct / return json
//TODO: check for further queries
func (wac *conn) GetProfilePicThumb(jid string) (<-chan string, error) {
	data := []interface{}{"query", "ProfilePicThumb", jid}
	return wac.write(data)
}

func (wac *conn) GetStatus(jid string) (<-chan string, error) {
	data := []interface{}{"query", "Status", jid}
	return wac.write(data)
}

func (wac *conn) GetGroupMetaData(jid string) (<-chan string, error) {
	data := []interface{}{"query", "GroupMetadata", jid}
	return wac.write(data)
}

func (wac *conn) SubscribePresence(jid string) (<-chan string, error) {
	data := []interface{}{"action", "presence", "subscribe", jid}
	return wac.write(data)
}

func (wac *conn) CreateGroup(subject string, participants []string) (<-chan string, error) {
	ts := time.Now().Unix()
	id := fmt.Sprintf("%d.--%d", ts, wac.msgCount)

	p := make([]binary.Node, len(participants))
	for i, participant := range participants {
		p[i] = binary.Node{
			Description: "participant",
			Attributes: map[string]string{
				"jid": participant,
			},
		}
	}

	//TODO: get proto or improve encoder to handle []interface{}
	n := binary.Node{
		Description: "action",
		Attributes: map[string]string{
			"type":  "set",
			"epoch": strconv.Itoa(wac.msgCount),
		},
		Content: []interface{}{binary.Node{
			Description: "group",
			Attributes: map[string]string{
				"author": wac.session.Wid,
				"id": id,
				"subject": subject,
				"type": "create",
			},
			Content: p,
		}},
	}

	wac.msgCount++
	return wac.writeBinary(n, GROUP, IGNORE, id)
}
