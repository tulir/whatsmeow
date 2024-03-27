// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package types

import (
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
)

// VerifiedName contains verified WhatsApp business details.
type VerifiedName struct {
	Certificate *waProto.VerifiedNameCertificate
	Details     *waProto.VerifiedNameCertificate_Details
}

// UserInfo contains info about a WhatsApp user.
type UserInfo struct {
	VerifiedName *VerifiedName
	Status       string
	PictureID    string
	Devices      []JID
}

// ProfilePictureInfo contains the ID and URL for a WhatsApp user's profile picture or group's photo.
type ProfilePictureInfo struct {
	URL  string `json:"url"`  // The full URL for the image, can be downloaded with a simple HTTP request.
	ID   string `json:"id"`   // The ID of the image. This is the same as UserInfo.PictureID.
	Type string `json:"type"` // The type of image. Known types include "image" (full res) and "preview" (thumbnail).

	DirectPath string `json:"direct_path"` // The path to the image, probably not very useful
}

// ContactInfo contains the cached names of a WhatsApp user.
type ContactInfo struct {
	Found bool

	FirstName    string
	FullName     string
	PushName     string
	BusinessName string
}

// LocalChatSettings contains the cached local settings for a chat.
type LocalChatSettings struct {
	Found bool

	MutedUntil time.Time
	Pinned     bool
	Archived   bool
}

// IsOnWhatsAppResponse contains information received in response to checking if a phone number is on WhatsApp.
type IsOnWhatsAppResponse struct {
	Query string // The query string used
	JID   JID    // The canonical user ID
	IsIn  bool   // Whether the phone is registered or not.

	VerifiedName *VerifiedName // If the phone is a business, the verified business details.
}

// BusinessMessageLinkTarget contains the info that is found using a business message link (see Client.ResolveBusinessMessageLink)
type BusinessMessageLinkTarget struct {
	JID JID // The JID of the business.

	PushName      string // The notify / push name of the business.
	VerifiedName  string // The verified business name.
	IsSigned      bool   // Some boolean, seems to be true?
	VerifiedLevel string // I guess the level of verification, starting from "unknown".

	Message string // The message that WhatsApp clients will pre-fill in the input box when clicking the link.
}

// ContactQRLinkTarget contains the info that is found using a contact QR link (see Client.ResolveContactQRLink)
type ContactQRLinkTarget struct {
	JID      JID    // The JID of the user.
	Type     string // Might always be "contact".
	PushName string // The notify / push name of the user.
}

// PrivacySetting is an individual setting value in the user's privacy settings.
type PrivacySetting string

// Possible privacy setting values.
const (
	PrivacySettingUndefined        PrivacySetting = ""
	PrivacySettingAll              PrivacySetting = "all"
	PrivacySettingContacts         PrivacySetting = "contacts"
	PrivacySettingContactBlacklist PrivacySetting = "contact_blacklist"
	PrivacySettingMatchLastSeen    PrivacySetting = "match_last_seen"
	PrivacySettingKnown            PrivacySetting = "known"
	PrivacySettingNone             PrivacySetting = "none"
)

// PrivacySettingType is the type of privacy setting.
type PrivacySettingType string

const (
	PrivacySettingTypeGroupAdd     PrivacySettingType = "groupadd"     // Valid values: PrivacySettingAll, PrivacySettingContacts, PrivacySettingContactBlacklist, PrivacySettingNone
	PrivacySettingTypeLastSeen     PrivacySettingType = "last"         // Valid values: PrivacySettingAll, PrivacySettingContacts, PrivacySettingContactBlacklist, PrivacySettingNone
	PrivacySettingTypeStatus       PrivacySettingType = "status"       // Valid values: PrivacySettingAll, PrivacySettingContacts, PrivacySettingContactBlacklist, PrivacySettingNone
	PrivacySettingTypeProfile      PrivacySettingType = "profile"      // Valid values: PrivacySettingAll, PrivacySettingContacts, PrivacySettingContactBlacklist, PrivacySettingNone
	PrivacySettingTypeReadReceipts PrivacySettingType = "readreceipts" // Valid values: PrivacySettingAll, PrivacySettingNone
	PrivacySettingTypeOnline       PrivacySettingType = "online"       // Valid values: PrivacySettingAll, PrivacySettingMatchLastSeen
	PrivacySettingTypeCallAdd      PrivacySettingType = "calladd"      // Valid values: PrivacySettingAll, PrivacySettingKnown
)

// PrivacySettings contains the user's privacy settings.
type PrivacySettings struct {
	GroupAdd     PrivacySetting // Valid values: PrivacySettingAll, PrivacySettingContacts, PrivacySettingContactBlacklist, PrivacySettingNone
	LastSeen     PrivacySetting // Valid values: PrivacySettingAll, PrivacySettingContacts, PrivacySettingContactBlacklist, PrivacySettingNone
	Status       PrivacySetting // Valid values: PrivacySettingAll, PrivacySettingContacts, PrivacySettingContactBlacklist, PrivacySettingNone
	Profile      PrivacySetting // Valid values: PrivacySettingAll, PrivacySettingContacts, PrivacySettingContactBlacklist, PrivacySettingNone
	ReadReceipts PrivacySetting // Valid values: PrivacySettingAll, PrivacySettingNone
	CallAdd      PrivacySetting // Valid values: PrivacySettingAll, PrivacySettingKnown
	Online       PrivacySetting // Valid values: PrivacySettingAll, PrivacySettingMatchLastSeen
}

// StatusPrivacyType is the type of list in StatusPrivacy.
type StatusPrivacyType string

const (
	// StatusPrivacyTypeContacts means statuses are sent to all contacts.
	StatusPrivacyTypeContacts StatusPrivacyType = "contacts"
	// StatusPrivacyTypeBlacklist means statuses are sent to all contacts, except the ones on the list.
	StatusPrivacyTypeBlacklist StatusPrivacyType = "blacklist"
	// StatusPrivacyTypeWhitelist means statuses are only sent to users on the list.
	StatusPrivacyTypeWhitelist StatusPrivacyType = "whitelist"
)

// StatusPrivacy contains the settings for who to send status messages to by default.
type StatusPrivacy struct {
	Type StatusPrivacyType
	List []JID

	IsDefault bool
}

// Blocklist contains the user's current list of blocked users.
type Blocklist struct {
	DHash string // TODO is this just a timestamp?
	JIDs  []JID
}

// BusinessHoursConfig contains business operating hours of a WhatsApp business.
type BusinessHoursConfig struct {
	DayOfWeek string
	Mode      string
	OpenTime  string
	CloseTime string
}

// Category contains a WhatsApp business category.
type Category struct {
	ID   string
	Name string
}

// BusinessProfile contains the profile information of a WhatsApp business.
type BusinessProfile struct {
	JID                   JID
	Address               string
	Email                 string
	Categories            []Category
	ProfileOptions        map[string]string
	BusinessHoursTimeZone string
	BusinessHours         []BusinessHoursConfig
}
