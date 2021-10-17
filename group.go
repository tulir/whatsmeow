// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsapp

import (
	"fmt"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
)

// GroupParticipant contains info about a participant of a WhatsApp group chat.
type GroupParticipant struct {
	JID     waBinary.JID
	IsAdmin bool
}

// GroupInfo contains basic information about a group chat on WhatsApp.
type GroupInfo struct {
	JID      waBinary.JID
	OwnerJID waBinary.JID

	Name      string
	NameSetAt time.Time
	NameSetBy waBinary.JID

	Announce bool // Can only admins send messages?
	Locked   bool // Can only admins edit group info?

	Topic      string
	TopicID    string
	TopicSetAt time.Time
	TopicSetBy waBinary.JID

	GroupCreated time.Time

	Participants []GroupParticipant
}

// BroadcastListInfo contains basic information about a broadcast list on WhatsApp.
type BroadcastListInfo struct {
	Name       string
	Recipients []waBinary.JID
}

// GetGroupInfo requests basic info about a group chat from the WhatsApp servers.
func (cli *Client) GetGroupInfo(jid waBinary.JID) (*GroupInfo, error) {
	res, err := cli.sendIQ(infoQuery{
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
	group.NameSetAt = time.Unix(ag.Int64("s_t"), 0)
	group.NameSetBy = ag.JID("s_o")

	group.GroupCreated = time.Unix(ag.Int64("creation"), 0)

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
				group.TopicSetAt = time.Unix(childAG.Int64("t"), 0)
			}
		case "announcement":
			group.Announce = true
		case "locked":
			group.Locked = true
		default:
			cli.Log.Debugf("Unknown element in group node %s: %s", jid.String(), child.XMLString())
		}
		if !childAG.OK() {
			cli.Log.Warnf("Possibly failed to parse %s element in group node: %+v", child.Tag, childAG.Errors)
		}
	}

	return &group, nil
}
