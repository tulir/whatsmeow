package whatsapp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Rhymen/go-whatsapp/binary"
)

func (wac *Conn) GetGroupMetaData(jid string) (<-chan string, error) {
	data := []interface{}{"query", "GroupMetadata", jid}
	return wac.writeJson(data)
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

func (wac *Conn) GroupInviteLink(jid string) (string, error) {
	request := []interface{}{"query", "inviteCode", jid}
	ch, err := wac.writeJson(request)
	if err != nil {
		return "", err
	}

	var response map[string]interface{}

	select {
	case r := <-ch:
		if err := json.Unmarshal([]byte(r), &response); err != nil {
			return "", fmt.Errorf("error decoding response message: %v\n", err)
		}
	case <-time.After(wac.msgTimeout):
		return "", fmt.Errorf("request timed out")
	}

	status := int(response["status"].(float64))
	if status == 401 {
		return "", ErrCantGetInviteLink
	} else if status != 200 {
		return "", fmt.Errorf("request responded with %d", status)
	}

	return response["code"].(string), nil
}

func (wac *Conn) GroupAcceptInviteCode(code string) (jid string, err error) {
	request := []interface{}{"action", "invite", code}
	ch, err := wac.writeJson(request)
	if err != nil {
		return "", err
	}

	var response map[string]interface{}

	select {
	case r := <-ch:
		if err := json.Unmarshal([]byte(r), &response); err != nil {
			return "", fmt.Errorf("error decoding response message: %v\n", err)
		}
	case <-time.After(wac.msgTimeout):
		return "", fmt.Errorf("request timed out")
	}

	status := int(response["status"].(float64))

	if status == 401 {
		return "", ErrJoinUnauthorized
	} else if status != 200 {
		return "", fmt.Errorf("request responded with %d", status)
	}

	return response["gid"].(string), nil
}

type descriptionID struct {
	DescID string `json:"descId"`
}

func (wac *Conn) getDescriptionID(jid string) (string, error) {
	data, err := wac.GetGroupMetaData(jid)
	if err != nil {
		return "none", err
	}
	var oldData descriptionID
	err = json.Unmarshal([]byte(<-data), &oldData)
	if err != nil {
		return "none", err
	}
	if oldData.DescID == "" {
		return "none", nil
	}
	return oldData.DescID, nil
}

func (wac *Conn) UpdateGroupDescription(jid, description string) (<-chan string, error) {
	prevID, err := wac.getDescriptionID(jid)
	if err != nil {
		return nil, err
	}
	newData := map[string]string{
		"prev": prevID,
	}
	var desc interface{} = description
	if description == "" {
		newData["delete"] = "true"
		desc = nil
	} else {
		newData["id"] = fmt.Sprintf("%d-%d", time.Now().Unix(), wac.msgCount*19)
	}
	tag := fmt.Sprintf("%d.--%d", time.Now().Unix(), wac.msgCount*19)
	n := binary.Node{
		Description: "action",
		Attributes: map[string]string{
			"type":  "set",
			"epoch": strconv.Itoa(wac.msgCount),
		},
		Content: []interface{}{
			binary.Node{
				Description: "group",
				Attributes: map[string]string{
					"id":     tag,
					"jid":    jid,
					"type":   "description",
					"author": wac.Info.Wid,
				},
				Content: []binary.Node{
					{
						Description: "description",
						Attributes:  newData,
						Content:     desc,
					},
				},
			},
		},
	}
	return wac.writeBinary(n, group, 136, tag)
}
