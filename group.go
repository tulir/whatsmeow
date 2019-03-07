package whatsapp

import (
	"encoding/json"
	"fmt"
	"time"
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

	if int(response["status"].(float64)) != 200 {
		return "", fmt.Errorf("request responded with %d", response["status"])
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

	if int(response["status"].(float64)) != 200 {
		return "", fmt.Errorf("request responded with %d", response["status"])
	}

	return response["gid"].(string), nil
}
