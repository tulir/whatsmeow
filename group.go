// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsapp

import (
	"fmt"

	waBinary "go.mau.fi/whatsmeow/binary"
)

type GroupParticipant struct {
	JID          waBinary.FullJID `json:"id"`
	IsAdmin      bool             `json:"isAdmin"`
	IsSuperAdmin bool             `json:"isSuperAdmin"`
}

type GroupInfo struct {
	JID      waBinary.FullJID `json:"jid"`
	OwnerJID waBinary.FullJID `json:"owner"`

	Name        string           `json:"subject"`
	NameSetTime int64            `json:"subjectTime"`
	NameSetBy   waBinary.FullJID `json:"subjectOwner"`

	Announce bool `json:"announce"` // Can only admins send messages?
	Locked   bool `json:"locked"`   // Can only admins edit group info?

	Topic      string           `json:"desc"`
	TopicID    string           `json:"descId"`
	TopicSetAt int64            `json:"descTime"`
	TopicSetBy waBinary.FullJID `json:"descOwner"`

	GroupCreated int64 `json:"creation"`

	Status int16 `json:"status"`

	Participants []GroupParticipant `json:"participants"`
}

type BroadcastListInfo struct {
	Status int16 `json:"status"`

	Name string `json:"name"`

	Recipients []struct {
		JID waBinary.FullJID `json:"id"`
	} `json:"recipients"`
}

func (cli *Client) GetGroupInfo(jid waBinary.FullJID) (*GroupInfo, error) {
	res, err := cli.sendIQ(InfoQuery{
		Namespace: "w:g2",
		Type:      "get",
		To:        jid,
		Content: []waBinary.Node{{
			Tag:   "query",
			Attrs: map[string]interface{}{"request": "interactive"},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to request group info: %w", err)
	}

	errorNode, ok := res.GetOptionalChildByTag("error")
	if ok {
		return nil, fmt.Errorf("group info request returned error: %s", errorNode.XMLString())
	}

	groupNode, ok := res.GetOptionalChildByTag("group")
	if !ok {
		return nil, fmt.Errorf("group info request didn't return group info")
	}

	var group GroupInfo
	ag := groupNode.AttrGetter()

	group.JID = waBinary.NewJID(ag.String("id"), waBinary.GroupServer)
	group.OwnerJID = ag.JID("creator")

	group.Name = ag.String("subject")
	group.NameSetTime = ag.Int64("s_t")
	group.NameSetBy = ag.JID("s_o")

	group.GroupCreated = ag.Int64("creation")

	for _, child := range groupNode.GetChildren() {
		childAG := child.AttrGetter()
		switch child.Tag {
		case "participant":
			participant := GroupParticipant{
				IsAdmin: childAG.OptionalString("type") == "admin",
				JID:     childAG.JID("jid"),
			}
			group.Participants = append(group.Participants, participant)
		case "description":
			body, bodyOK := child.GetOptionalChildByTag("body")
			if bodyOK {
				group.Topic, _ = body.Content.(string)
				group.TopicID = childAG.String("id")
				group.TopicSetBy = childAG.JID("participant")
				group.TopicSetAt = childAG.Int64("t")
			}
		case "announcement":
			group.Announce = true
		case "locked":
			group.Locked = true
		default:
			cli.Log.Debugfln("Unknown element in group node %s: %s", jid.String(), child.XMLString())
		}
		if !childAG.OK() {
			cli.Log.Warnfln("Possibly failed to parse %s element in group node: %+v", child.Tag, childAG.Errors)
		}
	}

	return &group, nil
}
