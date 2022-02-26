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

	waProto "go.mau.fi/whatsmeow/binary/proto"
)

// waVersion is the WhatsApp web client version
var waVersion = []uint32{2, 2206, 5}

// waVersionHash is the md5 hash of a dot-separated waVersion
var waVersionHash [16]byte

func init() {
	waVersionParts := make([]string, len(waVersion))
	for i, part := range waVersion {
		waVersionParts[i] = strconv.Itoa(int(part))
	}
	waVersionString := strings.Join(waVersionParts, ".")
	waVersionHash = md5.Sum([]byte(waVersionString))
}

var BaseClientPayload = &waProto.ClientPayload{
	UserAgent: &waProto.UserAgent{
		Platform:       waProto.UserAgent_WEB.Enum(),
		ReleaseChannel: waProto.UserAgent_RELEASE.Enum(),
		AppVersion: &waProto.AppVersion{
			Primary:   &waVersion[0],
			Secondary: &waVersion[1],
			Tertiary:  &waVersion[2],
		},
		Mcc:                         proto.String("000"),
		Mnc:                         proto.String("000"),
		OsVersion:                   proto.String("0.1.0"),
		Manufacturer:                proto.String(""),
		Device:                      proto.String("Desktop"),
		OsBuildNumber:               proto.String("0.1.0"),
		LocaleLanguageIso6391:       proto.String("en"),
		LocaleCountryIso31661Alpha2: proto.String("en"),
	},
	WebInfo: &waProto.WebInfo{
		WebSubPlatform: waProto.WebInfo_WEB_BROWSER.Enum(),
	},
	ConnectType:   waProto.ClientPayload_WIFI_UNKNOWN.Enum(),
	ConnectReason: waProto.ClientPayload_USER_ACTIVATED.Enum(),
}

var CompanionProps = &waProto.CompanionProps{
	Os: proto.String("whatsmeow"),
	Version: &waProto.AppVersion{
		Primary:   proto.Uint32(0),
		Secondary: proto.Uint32(1),
		Tertiary:  proto.Uint32(0),
	},
	PlatformType:    waProto.CompanionProps_UNKNOWN.Enum(),
	RequireFullSync: proto.Bool(false),
}

func SetOSInfo(name string, version [3]uint32) {
	CompanionProps.Os = &name
	CompanionProps.Version.Primary = &version[0]
	CompanionProps.Version.Secondary = &version[1]
	CompanionProps.Version.Tertiary = &version[2]
	BaseClientPayload.UserAgent.OsVersion = proto.String(fmt.Sprintf("%d.%d.%d", version[0], version[1], version[2]))
	BaseClientPayload.UserAgent.OsBuildNumber = BaseClientPayload.UserAgent.OsVersion
}

func (device *Device) getRegistrationPayload() *waProto.ClientPayload {
	payload := proto.Clone(BaseClientPayload).(*waProto.ClientPayload)
	regID := make([]byte, 4)
	binary.BigEndian.PutUint32(regID, device.RegistrationID)
	preKeyID := make([]byte, 4)
	binary.BigEndian.PutUint32(preKeyID, device.SignedPreKey.KeyID)
	companionProps, _ := proto.Marshal(CompanionProps)
	payload.RegData = &waProto.CompanionRegData{
		ERegid:         regID,
		EKeytype:       []byte{ecc.DjbType},
		EIdent:         device.IdentityKey.Pub[:],
		ESkeyId:        preKeyID[1:],
		ESkeyVal:       device.SignedPreKey.Pub[:],
		ESkeySig:       device.SignedPreKey.Signature[:],
		BuildHash:      waVersionHash[:],
		CompanionProps: companionProps,
	}
	payload.Passive = proto.Bool(false)
	return payload
}

func (device *Device) getLoginPayload() *waProto.ClientPayload {
	payload := proto.Clone(BaseClientPayload).(*waProto.ClientPayload)
	payload.Username = proto.Uint64(device.ID.UserInt())
	payload.Device = proto.Uint32(uint32(device.ID.Device))
	payload.Passive = proto.Bool(true)
	return payload
}

func (device *Device) GetClientPayload() *waProto.ClientPayload {
	if device.ID != nil {
		return device.getLoginPayload()
	} else {
		return device.getRegistrationPayload()
	}
}
