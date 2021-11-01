// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"errors"
	"fmt"
	"strings"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const InviteLinkPrefix = "https://chat.whatsapp.com/"

// GetGroupInviteLink requests the invite link to the group from the WhatsApp servers.
//
// If reset is true, then the old invite link will be revoked and a new one generated.
func (cli *Client) GetGroupInviteLink(jid types.JID, reset bool) (string, error) {
	iqType := "get"
	if reset {
		iqType = "set"
	}
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:g2",
		Type:      iqType,
		To:        jid,
		Content:   []waBinary.Node{{Tag: "invite"}},
	})
	if errors.Is(err, ErrIQNotAuthorized) {
		return "", wrapIQError(ErrGroupInviteLinkUnauthorized, err)
	} else if errors.Is(err, ErrIQNotFound) {
		return "", wrapIQError(ErrGroupNotFound, err)
	} else if errors.Is(err, ErrIQForbidden) {
		return "", wrapIQError(ErrNotInGroup, err)
	} else if err != nil {
		return "", err
	}
	code, ok := resp.GetChildByTag("invite").Attrs["code"].(string)
	if !ok {
		return "", fmt.Errorf("didn't find invite code in response")
	}
	return InviteLinkPrefix + code, nil
}

// GetGroupInfoFromInvite gets the group info from an invite message.
//
// Note that this is specifically for invite messages, not invite links. Use GetGroupInfoFromLink for resolving chat.whatsapp.com links.
func (cli *Client) GetGroupInfoFromInvite(jid, inviter types.JID, code string, expiration int64) (*types.GroupInfo, error) {
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:g2",
		Type:      "get",
		To:        jid,
		Content: []waBinary.Node{{
			Tag: "query",
			Content: []waBinary.Node{{
				Tag: "add_request",
				Attrs: waBinary.Attrs{
					"code":       code,
					"expiration": expiration,
					"admin":      inviter,
				},
			}},
		}},
	})
	if err != nil {
		return nil, err
	}
	groupNode, ok := resp.GetOptionalChildByTag("group")
	if !ok {
		return nil, fmt.Errorf("missing <group> element in response to invite group info query")
	}
	return cli.parseGroupNode(&groupNode)
}

// JoinGroupWithInvite joins a group using an invite message.
//
// Note that this is specifically for invite messages, not invite links. Use JoinGroupWithLink for joining with chat.whatsapp.com links.
func (cli *Client) JoinGroupWithInvite(jid, inviter types.JID, code string, expiration int64) error {
	_, err := cli.sendIQ(infoQuery{
		Namespace: "w:g2",
		Type:      "set",
		To:        jid,
		Content: []waBinary.Node{{
			Tag: "accept",
			Attrs: waBinary.Attrs{
				"code":       code,
				"expiration": expiration,
				"admin":      inviter,
			},
		}},
	})
	return err
}

// GetGroupInfoFromLink resolves the given invite link and asks the WhatsApp servers for info about the group.
// This will not cause the user to join the group.
func (cli *Client) GetGroupInfoFromLink(code string) (*types.GroupInfo, error) {
	code = strings.TrimPrefix(code, InviteLinkPrefix)
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:g2",
		Type:      "get",
		To:        types.GroupServerJID,
		Content: []waBinary.Node{{
			Tag:   "invite",
			Attrs: waBinary.Attrs{"code": code},
		}},
	})
	if errors.Is(err, ErrIQGone) {
		return nil, wrapIQError(ErrInviteLinkRevoked, err)
	} else if errors.Is(err, ErrIQNotAcceptable) {
		return nil, wrapIQError(ErrInviteLinkInvalid, err)
	} else if err != nil {
		return nil, err
	}
	groupNode, ok := resp.GetOptionalChildByTag("group")
	if !ok {
		return nil, fmt.Errorf("missing <group> element in response to invite link group info query")
	}
	return cli.parseGroupNode(&groupNode)
}

// JoinGroupWithLink joins the group using the given invite link.
func (cli *Client) JoinGroupWithLink(code string) (types.JID, error) {
	code = strings.TrimPrefix(code, InviteLinkPrefix)
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:g2",
		Type:      "set",
		To:        types.GroupServerJID,
		Content: []waBinary.Node{{
			Tag:   "invite",
			Attrs: waBinary.Attrs{"code": code}},
		},
	})
	if errors.Is(err, ErrIQGone) {
		return types.EmptyJID, wrapIQError(ErrInviteLinkRevoked, err)
	} else if errors.Is(err, ErrIQNotAcceptable) {
		return types.EmptyJID, wrapIQError(ErrInviteLinkInvalid, err)
	} else if err != nil {
		return types.EmptyJID, err
	}
	groupNode, ok := resp.GetOptionalChildByTag("group")
	if !ok {
		return types.EmptyJID, fmt.Errorf("missing <group> element in response to invite link group info query")
	}
	return groupNode.AttrGetter().JID("jid"), nil
}

// GetJoinedGroups returns the list of groups the user is participating in.
func (cli *Client) GetJoinedGroups() ([]*types.GroupInfo, error) {
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:g2",
		Type:      "get",
		To:        types.GroupServerJID,
		Content: []waBinary.Node{{
			Tag: "participating",
			Content: []waBinary.Node{
				{Tag: "participants"},
				{Tag: "description"},
			},
		}},
	})
	if err != nil {
		return nil, err
	}
	groups, ok := resp.GetOptionalChildByTag("groups")
	if !ok {
		return nil, fmt.Errorf("missing <groups> element in response to group list query")
	}
	children := groups.GetChildren()
	infos := make([]*types.GroupInfo, 0, len(children))
	for _, child := range children {
		if child.Tag != "group" {
			cli.Log.Debugf("Unexpected child in group list response: %s", child.XMLString())
			continue
		}
		parsed, parseErr := cli.parseGroupNode(&child)
		if parseErr != nil {
			cli.Log.Warnf("Error parsing group %s: %v", parsed.JID, parseErr)
		}
		infos = append(infos, parsed)
	}
	return infos, nil
}

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
	if errors.Is(err, ErrIQNotFound) {
		return nil, wrapIQError(ErrGroupNotFound, err)
	} else if errors.Is(err, ErrIQForbidden) {
		return nil, wrapIQError(ErrNotInGroup, err)
	} else if err != nil {
		return nil, err
	}

	groupNode, ok := res.GetOptionalChildByTag("group")
	if !ok {
		return nil, fmt.Errorf("missing <group> element in response to group info query")
	}
	return cli.parseGroupNode(&groupNode)
}

func (cli *Client) parseGroupNode(groupNode *waBinary.Node) (*types.GroupInfo, error) {
	var group types.GroupInfo
	ag := groupNode.AttrGetter()

	group.JID = types.NewJID(ag.String("id"), types.GroupServer)
	group.OwnerJID = ag.JID("creator")

	group.Name = ag.String("subject")
	group.NameSetAt = time.Unix(ag.Int64("s_t"), 0)
	group.NameSetBy = ag.JID("s_o")

	group.GroupCreated = time.Unix(ag.Int64("creation"), 0)

	group.AnnounceVersionID = ag.OptionalString("a_v_id")
	group.ParticipantVersionID = ag.OptionalString("p_v_id")

	for _, child := range groupNode.GetChildren() {
		childAG := child.AttrGetter()
		switch child.Tag {
		case "participant":
			pcpType := childAG.OptionalString("type")
			participant := types.GroupParticipant{
				IsAdmin:      pcpType == "admin" || pcpType == "superadmin",
				IsSuperAdmin: pcpType == "superadmin",
				JID:          childAG.JID("jid"),
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
			cli.Log.Debugf("Unknown element in group node %s: %s", group.JID.String(), child.XMLString())
		}
		if !childAG.OK() {
			cli.Log.Warnf("Possibly failed to parse %s element in group node: %+v", child.Tag, childAG.Errors)
		}
	}

	return &group, ag.Error()
}

func parseParticipantList(node *waBinary.Node) (participants []types.JID) {
	children := node.GetChildren()
	participants = make([]types.JID, 0, len(children))
	for _, child := range children {
		jid, ok := child.Attrs["jid"].(types.JID)
		if child.Tag != "participant" || !ok {
			continue
		}
		participants = append(participants, jid)
	}
	return
}

func (cli *Client) parseGroupCreate(node *waBinary.Node) (*events.JoinedGroup, error) {
	groupNode, ok := node.GetOptionalChildByTag("group")
	if !ok {
		return nil, fmt.Errorf("group create notification didn't contain group info")
	}
	var evt events.JoinedGroup
	evt.Reason = node.AttrGetter().OptionalString("reason")
	info, err := cli.parseGroupNode(&groupNode)
	if err != nil {
		return nil, fmt.Errorf("failed to parse group info in create notification: %w", err)
	}
	evt.GroupInfo = *info
	return &evt, nil
}

func (cli *Client) parseGroupChange(node *waBinary.Node) (*events.GroupInfo, error) {
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
		if child.Tag == "add" || child.Tag == "remove" || child.Tag == "promote" || child.Tag == "demote" {
			evt.PrevParticipantVersionID = cag.String("prev_v_id")
			evt.ParticipantVersionID = cag.String("v_id")
		}
		switch child.Tag {
		case "add":
			evt.JoinReason = cag.OptionalString("reason")
			evt.Join = parseParticipantList(&child)
		case "remove":
			evt.Leave = parseParticipantList(&child)
		case "promote":
			evt.Promote = parseParticipantList(&child)
		case "demote":
			evt.Demote = parseParticipantList(&child)
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
		case "invite":
			link := InviteLinkPrefix + cag.String("code")
			evt.NewInviteLink = &link
		default:
			evt.UnknownChanges = append(evt.UnknownChanges, &child)
		}
		if !cag.OK() {
			return nil, fmt.Errorf("group change %s element doesn't contain required attributes: %w", child.Tag, cag.Error())
		}
	}
	return &evt, nil
}

func (cli *Client) parseGroupNotification(node *waBinary.Node) (interface{}, error) {
	children := node.GetChildren()
	if len(children) == 1 && children[0].Tag == "create" {
		return cli.parseGroupCreate(&children[0])
	} else {
		return cli.parseGroupChange(node)
	}
}
