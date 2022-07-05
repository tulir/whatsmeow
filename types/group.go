// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types

import (
	"time"
)

type GroupMemberAddMode string

const (
	GroupMemberAddModeAdmin GroupMemberAddMode = "admin_add"
)

// GroupInfo contains basic information about a group chat on WhatsApp.
type GroupInfo struct {
	JID      JID
	OwnerJID JID

	GroupName
	GroupTopic
	GroupLocked
	GroupAnnounce
	GroupEphemeral

	GroupCreated time.Time

	ParticipantVersionID string
	Participants         []GroupParticipant

	MemberAddMode GroupMemberAddMode
}

// GroupName contains the name of a group along with metadata of who set it and when.
type GroupName struct {
	Name      string
	NameSetAt time.Time
	NameSetBy JID
}

// GroupTopic contains the topic (description) of a group along with metadata of who set it and when.
type GroupTopic struct {
	Topic      string
	TopicID    string
	TopicSetAt time.Time
	TopicSetBy JID
}

// GroupLocked specifies whether the group info can only be edited by admins.
type GroupLocked struct {
	IsLocked bool
}

// GroupAnnounce specifies whether only admins can send messages in the group.
type GroupAnnounce struct {
	IsAnnounce        bool
	AnnounceVersionID string
}

// GroupParticipant contains info about a participant of a WhatsApp group chat.
type GroupParticipant struct {
	JID          JID
	IsAdmin      bool
	IsSuperAdmin bool
}

// GroupEphemeral contains the group's disappearing messages settings.
type GroupEphemeral struct {
	IsEphemeral       bool
	DisappearingTimer uint32
}
