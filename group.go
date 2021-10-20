// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"fmt"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/events"
	"go.mau.fi/whatsmeow/types"
)

// GetGroupInfo requests basic info about a group chat from the WhatsApp servers.
func (cli *Client) GetGroupInfo(jid types.JID) (*types.GroupInfo, error) {
	res, err := cli.sendIQ(infoQuery{
		Namespace: "w:g2",
		Type:      "get",
		To:        jid,
		Content: []waBinary.Node{{
			Tag:   "query",
			Attrs: waBinary.Attrs{"request": "interactive"},
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

	var group types.GroupInfo
	ag := groupNode.AttrGetter()

	group.JID = types.NewJID(ag.String("id"), types.GroupServer)
	group.OwnerJID = ag.JID("creator")

	group.Name = ag.String("subject")
	group.NameSetAt = time.Unix(ag.Int64("s_t"), 0)
	group.NameSetBy = ag.JID("s_o")

	group.GroupCreated = time.Unix(ag.Int64("creation"), 0)

	group.AnnounceVersionID = ag.OptionalString("a_v_id")
	group.ParticipantVersionID = ag.String("p_v_id")

	for _, child := range groupNode.GetChildren() {
		childAG := child.AttrGetter()
		switch child.Tag {
		case "participant":
			participant := types.GroupParticipant{
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
			group.IsAnnounce = true
		case "locked":
			group.IsLocked = true
		default:
			cli.Log.Debugf("Unknown element in group node %s: %s", jid.String(), child.XMLString())
		}
		if !childAG.OK() {
			cli.Log.Warnf("Possibly failed to parse %s element in group node: %+v", child.Tag, childAG.Errors)
		}
	}

	return &group, nil
}

func parseParticipantList(node *waBinary.Node) (participants []types.GroupParticipant) {
	children := node.GetChildren()
	participants = make([]types.GroupParticipant, 0, len(children))
	for _, child := range children {
		jid, ok := child.Attrs["jid"].(types.JID)
		if child.Tag != "participant" || !ok {
			continue
		}
		pType, _ := child.Attrs["type"].(string)
		participants = append(participants, types.GroupParticipant{JID: jid, IsAdmin: pType == "admin"})
	}
	return
}

func parseGroupChange(node *waBinary.Node) (*events.GroupInfo, error) {
	var evt events.GroupInfo
	ag := node.AttrGetter()
	evt.JID = ag.JID("from")
	evt.Notify = ag.OptionalString("notify")
	evt.Sender = ag.OptionalJID("participant")
	evt.Timestamp = time.Unix(ag.Int64("t"), 0)
	if !ag.OK() {
		return nil, fmt.Errorf("group change doesn't contain required attributes: %w", ag.Error())
	}

	for _, child := range node.GetChildren() {
		cag := child.AttrGetter()
		switch child.Tag {
		case "add":
			evt.PrevParticipantVersionID = cag.String("prev_v_id")
			evt.ParticipantVersionID = cag.String("v_id")
			evt.JoinReason = cag.OptionalString("reason")
			evt.Join = parseParticipantList(&child)
		case "remove":
			evt.PrevParticipantVersionID = cag.String("prev_v_id")
			evt.ParticipantVersionID = cag.String("v_id")
			evt.Leave = parseParticipantList(&child)
		case "locked":
			evt.Locked = &types.GroupLocked{IsLocked: true}
		case "unlocked":
			evt.Locked = &types.GroupLocked{IsLocked: false}
		case "announcement":
			evt.Announce = &types.GroupAnnounce{
				IsAnnounce:        true,
				AnnounceVersionID: cag.String("v_id"),
			}
		case "not_announcement":
			evt.Announce = &types.GroupAnnounce{
				IsAnnounce:        false,
				AnnounceVersionID: cag.String("v_id"),
			}
		default:
			evt.UnknownChanges = append(evt.UnknownChanges, &child)
		}
		if !cag.OK() {
			return nil, fmt.Errorf("group change %s element doesn't contain required attributes: %w", child.Tag, cag.Error())
		}
	}
	return &evt, nil
}
