// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/libsignal/ecc"

	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/proto/waWa6"
	"go.mau.fi/whatsmeow/types"
)

// WAVersionContainer is a container for a WhatsApp web version number.
type WAVersionContainer [3]uint32

// ParseVersion parses a version string (three dot-separated numbers) into a WAVersionContainer.
func ParseVersion(version string) (parsed WAVersionContainer, err error) {
	var part1, part2, part3 int
	if parts := strings.Split(version, "."); len(parts) != 3 {
		err = fmt.Errorf("'%s' doesn't contain three dot-separated parts", version)
	} else if part1, err = strconv.Atoi(parts[0]); err != nil {
		err = fmt.Errorf("first part of '%s' is not a number: %w", version, err)
	} else if part2, err = strconv.Atoi(parts[1]); err != nil {
		err = fmt.Errorf("second part of '%s' is not a number: %w", version, err)
	} else if part3, err = strconv.Atoi(parts[2]); err != nil {
		err = fmt.Errorf("third part of '%s' is not a number: %w", version, err)
	} else {
		parsed = WAVersionContainer{uint32(part1), uint32(part2), uint32(part3)}
	}
	return
}

func (vc WAVersionContainer) LessThan(other WAVersionContainer) bool {
	return vc[0] < other[0] ||
		(vc[0] == other[0] && vc[1] < other[1]) ||
		(vc[0] == other[0] && vc[1] == other[1] && vc[2] < other[2])
}

// IsZero returns true if the version is zero.
func (vc WAVersionContainer) IsZero() bool {
	return vc == [3]uint32{0, 0, 0}
}

// String returns the version number as a dot-separated string.
func (vc WAVersionContainer) String() string {
	parts := make([]string, len(vc))
	for i, part := range vc {
		parts[i] = strconv.Itoa(int(part))
	}
	return strings.Join(parts, ".")
}

// Hash returns the md5 hash of the String representation of this version.
func (vc WAVersionContainer) Hash() [16]byte {
	return md5.Sum([]byte(vc.String()))
}

func (vc WAVersionContainer) ProtoAppVersion() *waWa6.ClientPayload_UserAgent_AppVersion {
	return &waWa6.ClientPayload_UserAgent_AppVersion{
		Primary:   &vc[0],
		Secondary: &vc[1],
		Tertiary:  &vc[2],
	}
}

// waVersion is the WhatsApp web client version
var waVersion = WAVersionContainer{2, 3000, 1050588369}

// waVersionHash is the md5 hash of a dot-separated waVersion
var waVersionHash = waVersion.Hash()

// GetWAVersion gets the current WhatsApp web client version.
func GetWAVersion() WAVersionContainer {
	return waVersion
}

// SetWAVersion sets the current WhatsApp web client version.
//
// In general, you should keep the library up-to-date instead of using this,
// as there may be code changes that are necessary too (like protobuf schema changes).
func SetWAVersion(version WAVersionContainer) {
	if version.IsZero() {
		return
	}
	waVersion = version
	waVersionHash = version.Hash()
	BaseClientPayload.UserAgent.AppVersion = waVersion.ProtoAppVersion()
}

var BaseClientPayload = &waWa6.ClientPayload{
	UserAgent: &waWa6.ClientPayload_UserAgent{
		Platform:       waWa6.ClientPayload_UserAgent_WEB.Enum(),
		ReleaseChannel: waWa6.ClientPayload_UserAgent_RELEASE.Enum(),
		AppVersion:     waVersion.ProtoAppVersion(),
		Mcc:            new("000"),
		Mnc:            new("000"),
		OsVersion:      new("0.1"),
		Manufacturer:   new(""),
		Device:         new("Desktop"),
		OsBuildNumber:  new("0.1"),

		LocaleLanguageIso6391:       new("en"),
		LocaleCountryIso31661Alpha2: new("US"),
	},
	WebInfo: &waWa6.ClientPayload_WebInfo{
		WebSubPlatform: waWa6.ClientPayload_WebInfo_WEB_BROWSER.Enum(),
	},
	ConnectType:   waWa6.ClientPayload_WIFI_UNKNOWN.Enum(),
	ConnectReason: waWa6.ClientPayload_USER_ACTIVATED.Enum(),
}

var DeviceProps = &waCompanionReg.DeviceProps{
	Os: new("whatsmeow"),
	Version: &waCompanionReg.DeviceProps_AppVersion{
		Primary:   proto.Uint32(0),
		Secondary: proto.Uint32(1),
		Tertiary:  proto.Uint32(0),
	},
	HistorySyncConfig: &waCompanionReg.DeviceProps_HistorySyncConfig{
		FullSyncDaysLimit:                        nil,
		FullSyncSizeMbLimit:                      nil,
		StorageQuotaMb:                           proto.Uint32(10240),
		InlineInitialPayloadInE2EeMsg:            new(true),
		RecentSyncDaysLimit:                      nil,
		SupportCallLogHistory:                    new(true),
		SupportBotUserAgentChatHistory:           new(true),
		SupportCagReactionsAndPolls:              new(true),
		SupportBizHostedMsg:                      new(true),
		SupportRecentSyncChunkMessageCountTuning: new(true),
		SupportHostedGroupMsg:                    new(true),
		SupportFbidBotChatHistory:                new(true),
		SupportAddOnHistorySyncMigration:         nil,
		SupportMessageAssociation:                new(true),
		SupportGroupHistory:                      new(true),
		OnDemandReady:                            nil,
		SupportGuestChat:                         nil,
		CompleteOnDemandReady:                    nil,
		ThumbnailSyncDaysLimit:                   proto.Uint32(60),
		InitialSyncMaxMessagesPerChat:            nil,
		SupportManusHistory:                      new(true),
		SupportHatchHistory:                      new(true),
	},
	PlatformType:    waCompanionReg.DeviceProps_UNKNOWN.Enum(),
	RequireFullSync: new(false),
}

func SetOSInfo(name string, version [3]uint32) {
	DeviceProps.Os = &name
	DeviceProps.Version.Primary = &version[0]
	DeviceProps.Version.Secondary = &version[1]
	DeviceProps.Version.Tertiary = &version[2]
	BaseClientPayload.UserAgent.OsVersion = new(fmt.Sprintf("%d.%d.%d", version[0], version[1], version[2]))
	BaseClientPayload.UserAgent.OsBuildNumber = BaseClientPayload.UserAgent.OsVersion
}

func (device *Device) getRegistrationPayload() *waWa6.ClientPayload {
	payload := proto.Clone(BaseClientPayload).(*waWa6.ClientPayload)
	regID := make([]byte, 4)
	binary.BigEndian.PutUint32(regID, device.RegistrationID)
	preKeyID := make([]byte, 4)
	binary.BigEndian.PutUint32(preKeyID, device.SignedPreKey.KeyID)
	deviceProps, _ := proto.Marshal(DeviceProps)
	payload.DevicePairingData = &waWa6.ClientPayload_DevicePairingRegistrationData{
		ERegid:      regID,
		EKeytype:    []byte{ecc.DjbType},
		EIdent:      device.IdentityKey.Pub[:],
		ESkeyID:     preKeyID[1:],
		ESkeyVal:    device.SignedPreKey.Pub[:],
		ESkeySig:    device.SignedPreKey.Signature[:],
		BuildHash:   waVersionHash[:],
		DeviceProps: deviceProps,
	}
	payload.Passive = new(false)
	payload.Pull = new(false)
	return payload
}

func (device *Device) getLoginPayload() *waWa6.ClientPayload {
	payload := proto.Clone(BaseClientPayload).(*waWa6.ClientPayload)
	payload.Username = new(device.ID.UserInt())
	payload.Device = new(uint32(device.ID.Device))
	payload.Passive = new(true)
	payload.Pull = new(true)
	payload.LidDbMigrated = new(true)
	if payload.Lc == nil {
		payload.Lc = proto.Int32(1)
	}
	return payload
}

func (device *Device) GetClientPayload() *waWa6.ClientPayload {
	if device.ID != nil {
		if *device.ID == types.EmptyJID {
			panic(fmt.Errorf("GetClientPayload called with empty JID"))
		}
		return device.getLoginPayload()
	} else {
		return device.getRegistrationPayload()
	}
}
