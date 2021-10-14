package whatsapp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/binary"
)

type GroupParticipant struct {
	JID          JID  `json:"id"`
	IsAdmin      bool `json:"isAdmin"`
	IsSuperAdmin bool `json:"isSuperAdmin"`

	FullJID binary.FullJID `json:"-"`
}

type GroupInfo struct {
	JID      JID `json:"jid"`
	OwnerJID JID `json:"owner"`

	Name        string `json:"subject"`
	NameSetTime int64  `json:"subjectTime"`
	NameSetBy   JID    `json:"subjectOwner"`

	Announce bool `json:"announce"` // Can only admins send messages?
	Locked bool `json:"locked"` // Can only admins edit group info?

	Topic      string `json:"desc"`
	TopicID    string `json:"descId"`
	TopicSetAt int64  `json:"descTime"`
	TopicSetBy JID    `json:"descOwner"`

	GroupCreated int64 `json:"creation"`

	Status int16 `json:"status"`

	Participants []GroupParticipant `json:"participants"`
}

type BroadcastListInfo struct {
	Status int16 `json:"status"`

	Name string `json:"name"`

	Recipients []struct {
		JID JID `json:"id"`
	} `json:"recipients"`
}

func (wac *Conn) GetBroadcastMetadata(jid JID) (*BroadcastListInfo, error) {
	data := []interface{}{"query", "contact", jid}
	resp, err := wac.writeJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to get broadcast list metadata: %w", err)
	}
	content := <-resp
	var info BroadcastListInfo
	err = json.Unmarshal([]byte(content), &info)
	if err != nil {
		return &info, fmt.Errorf("failed to unmarshal group metadata: %w", err)
	}
	for index, recipient := range info.Recipients {
		info.Recipients[index].JID = strings.Replace(recipient.JID, OldUserSuffix, NewUserSuffix, 1)
	}
	return &info, nil
}

func (wac *Conn) GetGroupMetaData(jid JID) (*GroupInfo, error) {
	data := []interface{}{"query", "GroupMetadata", jid}
	resp, err := wac.writeJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to get group metadata: %w", err)
	}
	content := <-resp

	var info GroupInfo
	err = json.Unmarshal([]byte(content), &info)
	if err != nil {
		return &info, fmt.Errorf("failed to unmarshal group metadata: %w", err)
	}

	for index, participant := range info.Participants {
		info.Participants[index].JID = strings.Replace(participant.JID, OldUserSuffix, NewUserSuffix, 1)
	}
	info.NameSetBy = strings.Replace(info.NameSetBy, OldUserSuffix, NewUserSuffix, 1)
	info.TopicSetBy = strings.Replace(info.TopicSetBy, OldUserSuffix, NewUserSuffix, 1)

	return &info, nil
}

type CreateGroupResponse struct {
	Status       int `json:"status"`
	GroupID      JID `json:"gid"`
	Participants map[JID]struct {
		Code string `json:"code"`
	} `json:"participants"`

	Source string `json:"-"`
}

type actualCreateGroupResponse struct {
	Status       int `json:"status"`
	GroupID      JID `json:"gid"`
	Participants []map[JID]struct {
		Code string `json:"code"`
	} `json:"participants"`
}

func (wac *Conn) CreateGroup(subject string, participants []JID) (*CreateGroupResponse, error) {
	respChan, err := wac.setGroup("create", "", subject, participants)
	if err != nil {
		return nil, err
	}
	var resp CreateGroupResponse
	var actualResp actualCreateGroupResponse
	resp.Source = <-respChan
	err = json.Unmarshal([]byte(resp.Source), &actualResp)
	if err != nil {
		return nil, err
	}
	resp.Status = actualResp.Status
	resp.GroupID = actualResp.GroupID
	resp.Participants = make(map[JID]struct {
		Code string `json:"code"`
	})
	for _, participantMap := range actualResp.Participants {
		for jid, status := range participantMap {
			resp.Participants[jid] = status
		}
	}
	return &resp, nil
}

func (wac *Conn) UpdateGroupSubject(subject string, jid JID) (<-chan string, error) {
	return wac.setGroup("subject", jid, subject, nil)
}

func (wac *Conn) SetAdmin(jid JID, participants []string) (<-chan string, error) {
	return wac.setGroup("promote", jid, "", participants)
}

func (wac *Conn) RemoveAdmin(jid JID, participants []string) (<-chan string, error) {
	return wac.setGroup("demote", jid, "", participants)
}

func (wac *Conn) AddMember(jid JID, participants []string) (<-chan string, error) {
	return wac.setGroup("add", jid, "", participants)
}

func (wac *Conn) RemoveMember(jid JID, participants []string) (<-chan string, error) {
	return wac.setGroup("remove", jid, "", participants)
}

func (wac *Conn) LeaveGroup(jid JID) (<-chan string, error) {
	return wac.setGroup("leave", jid, "", nil)
}

func (wac *Conn) GroupInviteLink(jid string) (string, error) {
	request := []interface{}{"query", "inviteCode", jid}
	ch, err := wac.writeJSON(request)
	if err != nil {
		return "", err
	}

	var response map[string]interface{}

	select {
	case r := <-ch:
		if err := json.Unmarshal([]byte(r), &response); err != nil {
			return "", fmt.Errorf("error decoding response message: %w", err)
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
	ch, err := wac.writeJSON(request)
	if err != nil {
		return "", err
	}

	var response map[string]interface{}

	select {
	case r := <-ch:
		if err := json.Unmarshal([]byte(r), &response); err != nil {
			return "", fmt.Errorf("error decoding response message: %w", err)
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

func (wac *Conn) UpdateGroupDescription(ownJID, groupJID JID, description string) (<-chan string, error) {
	prevMeta, err := wac.GetGroupMetaData(groupJID)
	if err != nil {
		return nil, err
	}
	newData := map[string]string{
		"prev": prevMeta.TopicID,
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
		Tag: "action",
		LegacyAttributes: map[string]string{
			"type":  "set",
			"epoch": strconv.Itoa(wac.msgCount),
		},
		Content: []interface{}{
			binary.Node{
				Tag: "group",
				LegacyAttributes: map[string]string{
					"id":     tag,
					"jid":    groupJID,
					"type":   "description",
					"author": ownJID,
				},
				Content: []binary.Node{
					{
						Tag:              "description",
						LegacyAttributes: newData,
						Content:          desc,
					},
				},
			},
		},
	}
	return wac.writeBinary(n, group, 136, tag)
}
