// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const InviteLinkPrefix = "https://chat.whatsapp.com/"

func (cli *Client) sendGroupIQ(ctx context.Context, iqType infoQueryType, jid types.JID, content waBinary.Node) (*waBinary.Node, error) {
	return cli.sendIQ(ctx, infoQuery{
		Namespace: "w:g2",
		Type:      iqType,
		To:        jid,
		Content:   []waBinary.Node{content},
	})
}

// ReqCreateGroup contains the request data for CreateGroup.
type ReqCreateGroup struct {
	// Group names are limited to 25 characters. A longer group name will cause a 406 not acceptable error.
	Name string
	// You don't need to include your own JID in the participants array, the WhatsApp servers will add it implicitly.
	Participants []types.JID
	// A create key can be provided to deduplicate the group create notification that will be triggered
	// when the group is created. If provided, the JoinedGroup event will contain the same key.
	CreateKey types.MessageID

	types.GroupEphemeral
	types.GroupAnnounce
	types.GroupLocked
	types.GroupMembershipApprovalMode
	// Set IsParent to true to create a community instead of a normal group.
	// When creating a community, the linked announcement group will be created automatically by the server.
	types.GroupParent
	// Set LinkedParentJID to create a group inside a community.
	types.GroupLinkedParent
}

// CreateGroup creates a group on WhatsApp with the given name and participants.
//
// See ReqCreateGroup for parameters.
func (cli *Client) CreateGroup(ctx context.Context, req ReqCreateGroup) (*types.GroupInfo, error) {
	participantNodes := make([]waBinary.Node, len(req.Participants), len(req.Participants)+1)
	for i, participant := range req.Participants {
		participantNodes[i] = waBinary.Node{
			Tag:   "participant",
			Attrs: waBinary.Attrs{"jid": participant},
		}
		pt, err := cli.Store.PrivacyTokens.GetPrivacyToken(ctx, participant)
		if err != nil {
			return nil, fmt.Errorf("failed to get privacy token for participant %s: %v", participant, err)
		} else if pt != nil {
			participantNodes[i].Content = []waBinary.Node{{
				Tag:     "privacy",
				Content: pt.Token,
			}}
		}
	}
	if req.CreateKey == "" {
		req.CreateKey = cli.GenerateMessageID()
	}
	if req.IsParent {
		if req.DefaultMembershipApprovalMode == "" {
			req.DefaultMembershipApprovalMode = "request_required"
		}
		participantNodes = append(participantNodes, waBinary.Node{
			Tag: "parent",
			Attrs: waBinary.Attrs{
				"default_membership_approval_mode": req.DefaultMembershipApprovalMode,
			},
		})
	} else if !req.LinkedParentJID.IsEmpty() {
		participantNodes = append(participantNodes, waBinary.Node{
			Tag:   "linked_parent",
			Attrs: waBinary.Attrs{"jid": req.LinkedParentJID},
		})
	}
	if req.IsLocked {
		participantNodes = append(participantNodes, waBinary.Node{Tag: "locked"})
	}
	if req.IsAnnounce {
		participantNodes = append(participantNodes, waBinary.Node{Tag: "announcement"})
	}
	if req.IsEphemeral {
		participantNodes = append(participantNodes, waBinary.Node{
			Tag: "ephemeral",
			Attrs: waBinary.Attrs{
				"expiration": req.DisappearingTimer,
				"trigger":    "1", // TODO what's this?
			},
		})
	}
	if req.IsJoinApprovalRequired {
		participantNodes = append(participantNodes, waBinary.Node{
			Tag: "membership_approval_mode",
			Content: []waBinary.Node{{
				Tag:   "group_join",
				Attrs: waBinary.Attrs{"state": "on"},
			}},
		})
	}
	// WhatsApp web doesn't seem to include the static prefix for these
	key := strings.TrimPrefix(req.CreateKey, "3EB0")
	resp, err := cli.sendGroupIQ(ctx, iqSet, types.GroupServerJID, waBinary.Node{
		Tag: "create",
		Attrs: waBinary.Attrs{
			"subject": req.Name,
			"key":     key,
		},
		Content: participantNodes,
	})
	if err != nil {
		return nil, err
	}
	groupNode, ok := resp.GetOptionalChildByTag("group")
	if !ok {
		return nil, &ElementMissingError{Tag: "group", In: "response to create group query"}
	}
	return cli.parseGroupNode(&groupNode)
}

// UnlinkGroup removes a child group from a parent community.
func (cli *Client) UnlinkGroup(ctx context.Context, parent, child types.JID) error {
	_, err := cli.sendGroupIQ(ctx, iqSet, parent, waBinary.Node{
		Tag:   "unlink",
		Attrs: waBinary.Attrs{"unlink_type": string(types.GroupLinkChangeTypeSub)},
		Content: []waBinary.Node{{
			Tag:   "group",
			Attrs: waBinary.Attrs{"jid": child},
		}},
	})
	return err
}

// LinkGroup adds an existing group as a child group in a community.
//
// To create a new group within a community, set LinkedParentJID in the CreateGroup request.
func (cli *Client) LinkGroup(ctx context.Context, parent, child types.JID) error {
	_, err := cli.sendGroupIQ(ctx, iqSet, parent, waBinary.Node{
		Tag: "links",
		Content: []waBinary.Node{{
			Tag:   "link",
			Attrs: waBinary.Attrs{"link_type": string(types.GroupLinkChangeTypeSub)},
			Content: []waBinary.Node{{
				Tag:   "group",
				Attrs: waBinary.Attrs{"jid": child},
			}},
		}},
	})
	return err
}

// LeaveGroup leaves the specified group on WhatsApp.
func (cli *Client) LeaveGroup(ctx context.Context, jid types.JID) error {
	_, err := cli.sendGroupIQ(ctx, iqSet, types.GroupServerJID, waBinary.Node{
		Tag: "leave",
		Content: []waBinary.Node{{
			Tag:   "group",
			Attrs: waBinary.Attrs{"id": jid},
		}},
	})
	return err
}

type ParticipantChange string

const (
	ParticipantChangeAdd     ParticipantChange = "add"
	ParticipantChangeRemove  ParticipantChange = "remove"
	ParticipantChangePromote ParticipantChange = "promote"
	ParticipantChangeDemote  ParticipantChange = "demote"
)

// UpdateGroupParticipants can be used to add, remove, promote and demote members in a WhatsApp group.
func (cli *Client) UpdateGroupParticipants(ctx context.Context, jid types.JID, participantChanges []types.JID, action ParticipantChange) ([]types.GroupParticipant, error) {
	content := make([]waBinary.Node, len(participantChanges))
	for i, participantJID := range participantChanges {
		content[i] = waBinary.Node{
			Tag:   "participant",
			Attrs: waBinary.Attrs{"jid": participantJID},
		}
		if participantJID.Server == types.HiddenUserServer && action == ParticipantChangeAdd {
			pn, err := cli.Store.LIDs.GetPNForLID(ctx, participantJID)
			if err != nil {
				return nil, fmt.Errorf("failed to get phone number for LID %s: %v", participantJID, err)
			} else if !pn.IsEmpty() {
				content[i].Attrs["phone_number"] = pn
			}
		}
	}
	resp, err := cli.sendGroupIQ(ctx, iqSet, jid, waBinary.Node{
		Tag:     string(action),
		Content: content,
	})
	if err != nil {
		return nil, err
	}
	requestAction, ok := resp.GetOptionalChildByTag(string(action))
	if !ok {
		return nil, &ElementMissingError{Tag: string(action), In: "response to group participants update"}
	}
	requestParticipants := requestAction.GetChildrenByTag("participant")
	participants := make([]types.GroupParticipant, len(requestParticipants))
	for i, child := range requestParticipants {
		participants[i] = parseParticipant(child.AttrGetter(), &child)
	}
	return participants, nil
}

// GetGroupRequestParticipants gets the list of participants that have requested to join the group.
func (cli *Client) GetGroupRequestParticipants(ctx context.Context, jid types.JID) ([]types.GroupParticipantRequest, error) {
	resp, err := cli.sendGroupIQ(ctx, iqGet, jid, waBinary.Node{
		Tag: "membership_approval_requests",
	})
	if err != nil {
		return nil, err
	}
	request, ok := resp.GetOptionalChildByTag("membership_approval_requests")
	if !ok {
		return nil, &ElementMissingError{Tag: "membership_approval_requests", In: "response to group request participants query"}
	}
	requestParticipants := request.GetChildrenByTag("membership_approval_request")
	participants := make([]types.GroupParticipantRequest, len(requestParticipants))
	for i, req := range requestParticipants {
		participants[i] = types.GroupParticipantRequest{
			JID:         req.AttrGetter().JID("jid"),
			RequestedAt: req.AttrGetter().UnixTime("request_time"),
		}
	}
	return participants, nil
}

type ParticipantRequestChange string

const (
	ParticipantChangeApprove ParticipantRequestChange = "approve"
	ParticipantChangeReject  ParticipantRequestChange = "reject"
)

// UpdateGroupRequestParticipants can be used to approve or reject requests to join the group.
func (cli *Client) UpdateGroupRequestParticipants(ctx context.Context, jid types.JID, participantChanges []types.JID, action ParticipantRequestChange) ([]types.GroupParticipant, error) {
	content := make([]waBinary.Node, len(participantChanges))
	for i, participantJID := range participantChanges {
		content[i] = waBinary.Node{
			Tag:   "participant",
			Attrs: waBinary.Attrs{"jid": participantJID},
		}
	}
	resp, err := cli.sendGroupIQ(ctx, iqSet, jid, waBinary.Node{
		Tag: "membership_requests_action",
		Content: []waBinary.Node{{
			Tag:     string(action),
			Content: content,
		}},
	})
	if err != nil {
		return nil, err
	}
	request, ok := resp.GetOptionalChildByTag("membership_requests_action")
	if !ok {
		return nil, &ElementMissingError{Tag: "membership_requests_action", In: "response to group request participants update"}
	}
	requestAction, ok := request.GetOptionalChildByTag(string(action))
	if !ok {
		return nil, &ElementMissingError{Tag: string(action), In: "response to group request participants update"}
	}
	requestParticipants := requestAction.GetChildrenByTag("participant")
	participants := make([]types.GroupParticipant, len(requestParticipants))
	for i, child := range requestParticipants {
		participants[i] = parseParticipant(child.AttrGetter(), &child)
	}
	return participants, nil
}

// SetGroupPhoto updates the group picture/icon of the given group on WhatsApp.
// The avatar should be a JPEG photo, other formats may be rejected with ErrInvalidImageFormat.
// The bytes can be nil to remove the photo. Returns the new picture ID.
func (cli *Client) SetGroupPhoto(ctx context.Context, jid types.JID, avatar []byte) (string, error) {
	var content interface{}
	if avatar != nil {
		content = []waBinary.Node{{
			Tag:     "picture",
			Attrs:   waBinary.Attrs{"type": "image"},
			Content: avatar,
		}}
	}
	resp, err := cli.sendIQ(ctx, infoQuery{
		Namespace: "w:profile:picture",
		Type:      iqSet,
		To:        types.ServerJID,
		Target:    jid,
		Content:   content,
	})
	if errors.Is(err, ErrIQNotAcceptable) {
		return "", wrapIQError(ErrInvalidImageFormat, err)
	} else if err != nil {
		return "", err
	}
	if avatar == nil {
		return "remove", nil
	}
	pictureID, ok := resp.GetChildByTag("picture").Attrs["id"].(string)
	if !ok {
		return "", fmt.Errorf("didn't find picture ID in response")
	}
	return pictureID, nil
}

// SetGroupName updates the name (subject) of the given group on WhatsApp.
func (cli *Client) SetGroupName(ctx context.Context, jid types.JID, name string) error {
	_, err := cli.sendGroupIQ(ctx, iqSet, jid, waBinary.Node{
		Tag:     "subject",
		Content: []byte(name),
	})
	return err
}

// SetGroupTopic updates the topic (description) of the given group on WhatsApp.
//
// The previousID and newID fields are optional. If the previous ID is not specified, this will
// automatically fetch the current group info to find the previous topic ID. If the new ID is not
// specified, one will be generated with Client.GenerateMessageID().
func (cli *Client) SetGroupTopic(ctx context.Context, jid types.JID, previousID, newID, topic string) error {
	if previousID == "" {
		oldInfo, err := cli.GetGroupInfo(ctx, jid)
		if err != nil {
			return fmt.Errorf("failed to get old group info to update topic: %v", err)
		}
		previousID = oldInfo.TopicID
	}
	if newID == "" {
		newID = cli.GenerateMessageID()
	}
	attrs := waBinary.Attrs{
		"id": newID,
	}
	if previousID != "" {
		attrs["prev"] = previousID
	}
	content := []waBinary.Node{{
		Tag:     "body",
		Content: []byte(topic),
	}}
	if len(topic) == 0 {
		attrs["delete"] = "true"
		content = nil
	}
	_, err := cli.sendGroupIQ(ctx, iqSet, jid, waBinary.Node{
		Tag:     "description",
		Attrs:   attrs,
		Content: content,
	})
	return err
}

// SetGroupLocked changes whether the group is locked (i.e. whether only admins can modify group info).
func (cli *Client) SetGroupLocked(ctx context.Context, jid types.JID, locked bool) error {
	tag := "locked"
	if !locked {
		tag = "unlocked"
	}
	_, err := cli.sendGroupIQ(ctx, iqSet, jid, waBinary.Node{Tag: tag})
	return err
}

// SetGroupAnnounce changes whether the group is in announce mode (i.e. whether only admins can send messages).
func (cli *Client) SetGroupAnnounce(ctx context.Context, jid types.JID, announce bool) error {
	tag := "announcement"
	if !announce {
		tag = "not_announcement"
	}
	_, err := cli.sendGroupIQ(ctx, iqSet, jid, waBinary.Node{Tag: tag})
	return err
}

// GetGroupInviteLink requests the invite link to the group from the WhatsApp servers.
//
// If reset is true, then the old invite link will be revoked and a new one generated.
func (cli *Client) GetGroupInviteLink(ctx context.Context, jid types.JID, reset bool) (string, error) {
	iqType := iqGet
	if reset {
		iqType = iqSet
	}
	resp, err := cli.sendGroupIQ(ctx, iqType, jid, waBinary.Node{Tag: "invite"})
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
func (cli *Client) GetGroupInfoFromInvite(ctx context.Context, jid, inviter types.JID, code string, expiration int64) (*types.GroupInfo, error) {
	resp, err := cli.sendGroupIQ(ctx, iqGet, jid, waBinary.Node{
		Tag: "query",
		Content: []waBinary.Node{{
			Tag: "add_request",
			Attrs: waBinary.Attrs{
				"code":       code,
				"expiration": expiration,
				"admin":      inviter,
			},
		}},
	})
	if err != nil {
		return nil, err
	}
	groupNode, ok := resp.GetOptionalChildByTag("group")
	if !ok {
		return nil, &ElementMissingError{Tag: "group", In: "response to invite group info query"}
	}
	return cli.parseGroupNode(&groupNode)
}

// JoinGroupWithInvite joins a group using an invite message.
//
// Note that this is specifically for invite messages, not invite links. Use JoinGroupWithLink for joining with chat.whatsapp.com links.
func (cli *Client) JoinGroupWithInvite(ctx context.Context, jid, inviter types.JID, code string, expiration int64) error {
	_, err := cli.sendGroupIQ(ctx, iqSet, jid, waBinary.Node{
		Tag: "accept",
		Attrs: waBinary.Attrs{
			"code":       code,
			"expiration": expiration,
			"admin":      inviter,
		},
	})
	return err
}

// GetGroupInfoFromLink resolves the given invite link and asks the WhatsApp servers for info about the group.
// This will not cause the user to join the group.
func (cli *Client) GetGroupInfoFromLink(ctx context.Context, code string) (*types.GroupInfo, error) {
	code = strings.TrimPrefix(code, InviteLinkPrefix)
	resp, err := cli.sendGroupIQ(ctx, iqGet, types.GroupServerJID, waBinary.Node{
		Tag:   "invite",
		Attrs: waBinary.Attrs{"code": code},
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
		return nil, &ElementMissingError{Tag: "group", In: "response to group link info query"}
	}
	return cli.parseGroupNode(&groupNode)
}

// JoinGroupWithLink joins the group using the given invite link.
func (cli *Client) JoinGroupWithLink(ctx context.Context, code string) (types.JID, error) {
	code = strings.TrimPrefix(code, InviteLinkPrefix)
	resp, err := cli.sendGroupIQ(ctx, iqSet, types.GroupServerJID, waBinary.Node{
		Tag:   "invite",
		Attrs: waBinary.Attrs{"code": code},
	})
	if errors.Is(err, ErrIQGone) {
		return types.EmptyJID, wrapIQError(ErrInviteLinkRevoked, err)
	} else if errors.Is(err, ErrIQNotAcceptable) {
		return types.EmptyJID, wrapIQError(ErrInviteLinkInvalid, err)
	} else if err != nil {
		return types.EmptyJID, err
	}
	membershipApprovalModeNode, ok := resp.GetOptionalChildByTag("membership_approval_request")
	if ok {
		return membershipApprovalModeNode.AttrGetter().JID("jid"), nil
	}
	groupNode, ok := resp.GetOptionalChildByTag("group")
	if !ok {
		return types.EmptyJID, &ElementMissingError{Tag: "group", In: "response to group link join query"}
	}
	return groupNode.AttrGetter().JID("jid"), nil
}

// GetJoinedGroups returns the list of groups the user is participating in.
func (cli *Client) GetJoinedGroups(ctx context.Context) ([]*types.GroupInfo, error) {
	resp, err := cli.sendGroupIQ(ctx, iqGet, types.GroupServerJID, waBinary.Node{
		Tag: "participating",
		Content: []waBinary.Node{
			{Tag: "participants"},
			{Tag: "description"},
		},
	})
	if err != nil {
		return nil, err
	}
	groups, ok := resp.GetOptionalChildByTag("groups")
	if !ok {
		return nil, &ElementMissingError{Tag: "groups", In: "response to group list query"}
	}
	children := groups.GetChildren()
	infos := make([]*types.GroupInfo, 0, len(children))
	var allLIDPairs []store.LIDMapping
	var allRedactedPhones []store.RedactedPhoneEntry
	for _, child := range children {
		if child.Tag != "group" {
			cli.Log.Debugf("Unexpected child in group list response: %s", child.XMLString())
			continue
		}
		parsed, parseErr := cli.parseGroupNode(&child)
		if parseErr != nil {
			cli.Log.Warnf("Error parsing group %s: %v", parsed.JID, parseErr)
		}
		lidPairs, redactedPhones := cli.cacheGroupInfo(parsed, true)
		allLIDPairs = append(allLIDPairs, lidPairs...)
		allRedactedPhones = append(allRedactedPhones, redactedPhones...)
		infos = append(infos, parsed)
	}
	err = cli.Store.LIDs.PutManyLIDMappings(ctx, allLIDPairs)
	if err != nil {
		cli.Log.Warnf("Failed to store LID mappings from joined groups: %v", err)
	}
	err = cli.Store.Contacts.PutManyRedactedPhones(ctx, allRedactedPhones)
	if err != nil {
		cli.Log.Warnf("Failed to store redacted phones from joined groups: %v", err)
	}
	return infos, nil
}

// GetSubGroups gets the subgroups of the given community.
func (cli *Client) GetSubGroups(ctx context.Context, community types.JID) ([]*types.GroupLinkTarget, error) {
	res, err := cli.sendGroupIQ(ctx, iqGet, community, waBinary.Node{Tag: "sub_groups"})
	if err != nil {
		return nil, err
	}
	groups, ok := res.GetOptionalChildByTag("sub_groups")
	if !ok {
		return nil, &ElementMissingError{Tag: "sub_groups", In: "response to subgroups query"}
	}
	var parsedGroups []*types.GroupLinkTarget
	for _, child := range groups.GetChildren() {
		if child.Tag == "group" {
			parsedGroup, err := parseGroupLinkTargetNode(&child)
			if err != nil {
				return parsedGroups, fmt.Errorf("failed to parse group in subgroups list: %w", err)
			}
			parsedGroups = append(parsedGroups, &parsedGroup)
		}
	}
	return parsedGroups, nil
}

// GetLinkedGroupsParticipants gets all the participants in the groups of the given community.
func (cli *Client) GetLinkedGroupsParticipants(ctx context.Context, community types.JID) ([]types.JID, error) {
	res, err := cli.sendGroupIQ(ctx, iqGet, community, waBinary.Node{Tag: "linked_groups_participants"})
	if err != nil {
		return nil, err
	}
	participants, ok := res.GetOptionalChildByTag("linked_groups_participants")
	if !ok {
		return nil, &ElementMissingError{Tag: "linked_groups_participants", In: "response to community participants query"}
	}
	members, lidPairs := parseParticipantList(&participants)
	if len(lidPairs) > 0 {
		err = cli.Store.LIDs.PutManyLIDMappings(ctx, lidPairs)
		if err != nil {
			cli.Log.Warnf("Failed to store LID mappings for community participants: %v", err)
		}
	}
	return members, nil
}

// GetGroupInfo requests basic info about a group chat from the WhatsApp servers.
func (cli *Client) GetGroupInfo(ctx context.Context, jid types.JID) (*types.GroupInfo, error) {
	return cli.getGroupInfo(ctx, jid, true)
}

func (cli *Client) cacheGroupInfo(groupInfo *types.GroupInfo, lock bool) ([]store.LIDMapping, []store.RedactedPhoneEntry) {
	participants := make([]types.JID, len(groupInfo.Participants))
	lidPairs := make([]store.LIDMapping, len(groupInfo.Participants))
	redactedPhones := make([]store.RedactedPhoneEntry, 0)
	for i, part := range groupInfo.Participants {
		participants[i] = part.JID
		if !part.PhoneNumber.IsEmpty() && !part.LID.IsEmpty() {
			lidPairs[i] = store.LIDMapping{
				LID: part.LID,
				PN:  part.PhoneNumber,
			}
		}
		if part.DisplayName != "" && !part.LID.IsEmpty() {
			redactedPhones = append(redactedPhones, store.RedactedPhoneEntry{
				JID:           part.LID,
				RedactedPhone: part.DisplayName,
			})
		}
	}
	if lock {
		cli.groupCacheLock.Lock()
		defer cli.groupCacheLock.Unlock()
	}
	cli.groupCache[groupInfo.JID] = &groupMetaCache{
		AddressingMode:             groupInfo.AddressingMode,
		CommunityAnnouncementGroup: groupInfo.IsAnnounce && groupInfo.IsDefaultSubGroup,
		Members:                    participants,
	}
	return lidPairs, redactedPhones
}

func (cli *Client) getGroupInfo(ctx context.Context, jid types.JID, lockParticipantCache bool) (*types.GroupInfo, error) {
	res, err := cli.sendGroupIQ(ctx, iqGet, jid, waBinary.Node{
		Tag:   "query",
		Attrs: waBinary.Attrs{"request": "interactive"},
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
		return nil, &ElementMissingError{Tag: "groups", In: "response to group info query"}
	}
	groupInfo, err := cli.parseGroupNode(&groupNode)
	if err != nil {
		return groupInfo, err
	}
	lidPairs, redactedPhones := cli.cacheGroupInfo(groupInfo, lockParticipantCache)
	err = cli.Store.LIDs.PutManyLIDMappings(ctx, lidPairs)
	if err != nil {
		cli.Log.Warnf("Failed to store LID mappings for members of %s: %v", jid, err)
	}
	err = cli.Store.Contacts.PutManyRedactedPhones(ctx, redactedPhones)
	if err != nil {
		cli.Log.Warnf("Failed to store redacted phones for members of %s: %v", jid, err)
	}
	return groupInfo, nil
}

func (cli *Client) getCachedGroupData(ctx context.Context, jid types.JID) (*groupMetaCache, error) {
	cli.groupCacheLock.Lock()
	defer cli.groupCacheLock.Unlock()
	if val, ok := cli.groupCache[jid]; ok {
		return val, nil
	}
	_, err := cli.getGroupInfo(ctx, jid, false)
	if err != nil {
		return nil, err
	}
	return cli.groupCache[jid], nil
}

func parseParticipant(childAG *waBinary.AttrUtility, child *waBinary.Node) types.GroupParticipant {
	pcpType := childAG.OptionalString("type")
	participant := types.GroupParticipant{
		IsAdmin:      pcpType == "admin" || pcpType == "superadmin",
		IsSuperAdmin: pcpType == "superadmin",
		JID:          childAG.JID("jid"),
		DisplayName:  childAG.OptionalString("display_name"),
	}
	if participant.JID.Server == types.HiddenUserServer {
		participant.LID = participant.JID
		participant.PhoneNumber = childAG.OptionalJIDOrEmpty("phone_number")
	} else if participant.JID.Server == types.DefaultUserServer {
		participant.PhoneNumber = participant.JID
		participant.LID = childAG.OptionalJIDOrEmpty("lid")
	}
	if errorCode := childAG.OptionalInt("error"); errorCode != 0 {
		participant.Error = errorCode
		addRequest, ok := child.GetOptionalChildByTag("add_request")
		if ok {
			addAG := addRequest.AttrGetter()
			participant.AddRequest = &types.GroupParticipantAddRequest{
				Code:       addAG.String("code"),
				Expiration: addAG.UnixTime("expiration"),
			}
		}
	}
	return participant
}

func (cli *Client) parseGroupNode(groupNode *waBinary.Node) (*types.GroupInfo, error) {
	var group types.GroupInfo
	ag := groupNode.AttrGetter()

	group.JID = types.NewJID(ag.String("id"), types.GroupServer)
	group.OwnerJID = ag.OptionalJIDOrEmpty("creator")
	group.OwnerPN = ag.OptionalJIDOrEmpty("creator_pn")

	group.Name = ag.OptionalString("subject")
	group.NameSetAt = ag.OptionalUnixTime("s_t")
	group.NameSetBy = ag.OptionalJIDOrEmpty("s_o")
	group.NameSetByPN = ag.OptionalJIDOrEmpty("s_o_pn")

	group.GroupCreated = ag.UnixTime("creation")
	group.CreatorCountryCode = ag.OptionalString("creator_country_code")

	group.AnnounceVersionID = ag.OptionalString("a_v_id")
	group.ParticipantVersionID = ag.OptionalString("p_v_id")
	group.ParticipantCount = ag.OptionalInt("size")
	group.AddressingMode = types.AddressingMode(ag.OptionalString("addressing_mode"))

	for _, child := range groupNode.GetChildren() {
		childAG := child.AttrGetter()
		switch child.Tag {
		case "participant":
			group.Participants = append(group.Participants, parseParticipant(childAG, &child))
		case "description":
			body, bodyOK := child.GetOptionalChildByTag("body")
			if bodyOK {
				topicBytes, _ := body.Content.([]byte)
				group.Topic = string(topicBytes)
				group.TopicID = childAG.String("id")
				group.TopicSetBy = childAG.OptionalJIDOrEmpty("participant")
				group.TopicSetByPN = childAG.OptionalJIDOrEmpty("participant_pn") // TODO confirm field name
				group.TopicSetAt = childAG.UnixTime("t")
			}
		case "announcement":
			group.IsAnnounce = true
		case "locked":
			group.IsLocked = true
		case "ephemeral":
			group.IsEphemeral = true
			group.DisappearingTimer = uint32(childAG.Uint64("expiration"))
		case "member_add_mode":
			modeBytes, _ := child.Content.([]byte)
			group.MemberAddMode = types.GroupMemberAddMode(modeBytes)
		case "linked_parent":
			group.LinkedParentJID = childAG.JID("jid")
		case "default_sub_group":
			group.IsDefaultSubGroup = true
		case "parent":
			group.IsParent = true
			group.DefaultMembershipApprovalMode = childAG.OptionalString("default_membership_approval_mode")
		case "incognito":
			group.IsIncognito = true
		case "membership_approval_mode":
			group.IsJoinApprovalRequired = true
		case "suspended":
			group.Suspended = true
		default:
			cli.Log.Debugf("Unknown element in group node %s: %s", group.JID.String(), child.XMLString())
		}
		if !childAG.OK() {
			cli.Log.Warnf("Possibly failed to parse %s element in group node: %+v", child.Tag, childAG.Errors)
		}
	}

	return &group, ag.Error()
}

func parseGroupLinkTargetNode(groupNode *waBinary.Node) (types.GroupLinkTarget, error) {
	ag := groupNode.AttrGetter()
	jidKey := ag.OptionalJIDOrEmpty("jid")
	if jidKey.IsEmpty() {
		jidKey = types.NewJID(ag.String("id"), types.GroupServer)
	}
	return types.GroupLinkTarget{
		JID: jidKey,
		GroupName: types.GroupName{
			Name:      ag.OptionalString("subject"),
			NameSetAt: ag.OptionalUnixTime("s_t"),
		},
		GroupIsDefaultSub: types.GroupIsDefaultSub{
			IsDefaultSubGroup: groupNode.GetChildByTag("default_sub_group").Tag == "default_sub_group",
		},
	}, ag.Error()
}

func parseParticipantList(node *waBinary.Node) (participants []types.JID, lidPairs []store.LIDMapping) {
	children := node.GetChildren()
	participants = make([]types.JID, 0, len(children))
	for _, child := range children {
		jid, ok := child.Attrs["jid"].(types.JID)
		if child.Tag != "participant" || !ok {
			continue
		}
		participants = append(participants, jid)
		if jid.Server == types.HiddenUserServer {
			phoneNumber, ok := child.Attrs["phone_number"].(types.JID)
			if ok && !phoneNumber.IsEmpty() {
				lidPairs = append(lidPairs, store.LIDMapping{
					LID: jid,
					PN:  phoneNumber,
				})
			}
		} else if jid.Server == types.DefaultUserServer {
			lid, ok := child.Attrs["lid"].(types.JID)
			if ok && !lid.IsEmpty() {
				lidPairs = append(lidPairs, store.LIDMapping{
					LID: lid,
					PN:  jid,
				})
			}
		}
	}
	return
}

func (cli *Client) parseGroupCreate(parentNode, node *waBinary.Node) (*events.JoinedGroup, []store.LIDMapping, []store.RedactedPhoneEntry, error) {
	groupNode, ok := node.GetOptionalChildByTag("group")
	if !ok {
		return nil, nil, nil, fmt.Errorf("group create notification didn't contain group info")
	}
	var evt events.JoinedGroup
	pag := parentNode.AttrGetter()
	ag := node.AttrGetter()
	evt.Reason = ag.OptionalString("reason")
	evt.CreateKey = ag.OptionalString("key")
	evt.Type = ag.OptionalString("type")
	evt.Sender = pag.OptionalJID("participant")
	evt.SenderPN = pag.OptionalJID("participant_pn")
	evt.Notify = pag.OptionalString("notify")
	info, err := cli.parseGroupNode(&groupNode)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse group info in create notification: %w", err)
	}
	evt.GroupInfo = *info
	lidPairs, redactedPhones := cli.cacheGroupInfo(info, true)
	return &evt, lidPairs, redactedPhones, nil
}

func (cli *Client) parseGroupChange(node *waBinary.Node) (*events.GroupInfo, []store.LIDMapping, error) {
	var evt events.GroupInfo
	ag := node.AttrGetter()
	evt.JID = ag.JID("from")
	evt.Notify = ag.OptionalString("notify")
	evt.Sender = ag.OptionalJID("participant")
	evt.SenderPN = ag.OptionalJID("participant_pn")
	evt.Timestamp = ag.UnixTime("t")
	if !ag.OK() {
		return nil, nil, fmt.Errorf("group change doesn't contain required attributes: %w", ag.Error())
	}

	var lidPairs []store.LIDMapping
	for _, child := range node.GetChildren() {
		cag := child.AttrGetter()
		if child.Tag == "add" || child.Tag == "remove" || child.Tag == "promote" || child.Tag == "demote" {
			evt.PrevParticipantVersionID = cag.OptionalString("prev_v_id")
			evt.ParticipantVersionID = cag.OptionalString("v_id")
		}
		switch child.Tag {
		case "add":
			evt.JoinReason = cag.OptionalString("reason")
			evt.Join, lidPairs = parseParticipantList(&child)
		case "remove":
			evt.Leave, lidPairs = parseParticipantList(&child)
		case "promote":
			evt.Promote, lidPairs = parseParticipantList(&child)
		case "demote":
			evt.Demote, lidPairs = parseParticipantList(&child)
		case "locked":
			evt.Locked = &types.GroupLocked{IsLocked: true}
		case "unlocked":
			evt.Locked = &types.GroupLocked{IsLocked: false}
		case "delete":
			evt.Delete = &types.GroupDelete{Deleted: true, DeleteReason: cag.String("reason")}
		case "subject":
			evt.Name = &types.GroupName{
				Name:        cag.String("subject"),
				NameSetAt:   cag.UnixTime("s_t"),
				NameSetBy:   cag.OptionalJIDOrEmpty("s_o"),
				NameSetByPN: cag.OptionalJIDOrEmpty("s_o_pn"),
			}
		case "description":
			var topicStr string
			_, isDelete := child.GetOptionalChildByTag("delete")
			if !isDelete {
				topicChild := child.GetChildByTag("body")
				topicBytes, ok := topicChild.Content.([]byte)
				if !ok {
					return nil, nil, fmt.Errorf("group change description has unexpected body: %s", topicChild.XMLString())
				}
				topicStr = string(topicBytes)
			}
			var setBy types.JID
			if evt.Sender != nil {
				setBy = *evt.Sender
			}
			evt.Topic = &types.GroupTopic{
				Topic:        topicStr,
				TopicID:      cag.String("id"),
				TopicSetAt:   evt.Timestamp,
				TopicSetBy:   setBy,
				TopicDeleted: isDelete,
			}
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
		case "ephemeral":
			timer := uint32(cag.Uint64("expiration"))
			evt.Ephemeral = &types.GroupEphemeral{
				IsEphemeral:       true,
				DisappearingTimer: timer,
			}
		case "not_ephemeral":
			evt.Ephemeral = &types.GroupEphemeral{IsEphemeral: false}
		case "link":
			evt.Link = &types.GroupLinkChange{
				Type: types.GroupLinkChangeType(cag.String("link_type")),
			}
			groupNode, ok := child.GetOptionalChildByTag("group")
			if !ok {
				return nil, nil, &ElementMissingError{Tag: "group", In: "group link"}
			}
			var err error
			evt.Link.Group, err = parseGroupLinkTargetNode(&groupNode)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse group link node in group change: %w", err)
			}
		case "unlink":
			evt.Unlink = &types.GroupLinkChange{
				Type:         types.GroupLinkChangeType(cag.String("unlink_type")),
				UnlinkReason: types.GroupUnlinkReason(cag.String("unlink_reason")),
			}
			groupNode, ok := child.GetOptionalChildByTag("group")
			if !ok {
				return nil, nil, &ElementMissingError{Tag: "group", In: "group unlink"}
			}
			var err error
			evt.Unlink.Group, err = parseGroupLinkTargetNode(&groupNode)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse group unlink node in group change: %w", err)
			}
		case "membership_approval_mode":
			evt.MembershipApprovalMode = &types.GroupMembershipApprovalMode{
				IsJoinApprovalRequired: true,
			}
		case "suspended":
			evt.Suspended = true
		case "unsuspended":
			evt.Unsuspended = true
		default:
			evt.UnknownChanges = append(evt.UnknownChanges, &child)
		}
		if !cag.OK() {
			return nil, nil, fmt.Errorf("group change %s element doesn't contain required attributes: %w", child.Tag, cag.Error())
		}
	}
	return &evt, lidPairs, nil
}

func (cli *Client) updateGroupParticipantCache(evt *events.GroupInfo) {
	// TODO can the addressing mode change here?
	if len(evt.Join) == 0 && len(evt.Leave) == 0 {
		return
	}
	cli.groupCacheLock.Lock()
	defer cli.groupCacheLock.Unlock()
	cached, ok := cli.groupCache[evt.JID]
	if !ok {
		return
	}
Outer:
	for _, jid := range evt.Join {
		for _, existingJID := range cached.Members {
			if jid == existingJID {
				continue Outer
			}
		}
		cached.Members = append(cached.Members, jid)
	}
	for _, jid := range evt.Leave {
		for i, existingJID := range cached.Members {
			if existingJID == jid {
				cached.Members[i] = cached.Members[len(cached.Members)-1]
				cached.Members = cached.Members[:len(cached.Members)-1]
				break
			}
		}
	}
}

func (cli *Client) parseGroupNotification(node *waBinary.Node) (any, []store.LIDMapping, []store.RedactedPhoneEntry, error) {
	children := node.GetChildren()
	if len(children) == 1 && children[0].Tag == "create" {
		return cli.parseGroupCreate(node, &children[0])
	} else {
		groupChange, lidPairs, err := cli.parseGroupChange(node)
		if err != nil {
			return nil, nil, nil, err
		}
		cli.updateGroupParticipantCache(groupChange)
		return groupChange, lidPairs, nil, nil
	}
}

// SetGroupJoinApprovalMode sets the group join approval mode to 'on' or 'off'.
func (cli *Client) SetGroupJoinApprovalMode(ctx context.Context, jid types.JID, mode bool) error {
	modeStr := "off"
	if mode {
		modeStr = "on"
	}

	content := waBinary.Node{
		Tag: "membership_approval_mode",
		Content: []waBinary.Node{
			{
				Tag:   "group_join",
				Attrs: waBinary.Attrs{"state": modeStr},
			},
		},
	}

	_, err := cli.sendGroupIQ(ctx, iqSet, jid, content)
	return err
}

// SetGroupMemberAddMode sets the group member add mode to 'admin_add' or 'all_member_add'.
func (cli *Client) SetGroupMemberAddMode(ctx context.Context, jid types.JID, mode types.GroupMemberAddMode) error {
	if mode != types.GroupMemberAddModeAdmin && mode != types.GroupMemberAddModeAllMember {
		return errors.New("invalid mode, must be 'admin_add' or 'all_member_add'")
	}

	content := waBinary.Node{
		Tag:     "member_add_mode",
		Content: []byte(mode),
	}

	_, err := cli.sendGroupIQ(ctx, iqSet, jid, content)
	return err
}

// SetGroupDescription updates the group description.
func (cli *Client) SetGroupDescription(ctx context.Context, jid types.JID, description string) error {
	content := waBinary.Node{
		Tag: "description",
		Content: []waBinary.Node{
			{
				Tag:     "body",
				Content: []byte(description),
			},
		},
	}

	_, err := cli.sendGroupIQ(ctx, iqSet, jid, content)
	return err
}
