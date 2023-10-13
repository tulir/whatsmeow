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

func (vc WAVersionContainer) ProtoAppVersion() *waProto.ClientPayload_UserAgent_AppVersion {
	return &waProto.ClientPayload_UserAgent_AppVersion{
		Primary:   &vc[0],
		Secondary: &vc[1],
		Tertiary:  &vc[2],
	}
}

// waVersion is the WhatsApp web client version
var waVersion = WAVersionContainer{2, 2344, 53}

// waVersionHash is the md5 hash of a dot-separated waVersion
var waVersionHash [16]byte

func init() {
	waVersionHash = waVersion.Hash()
}

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
}

var BaseClientPayload = &waProto.ClientPayload{
	UserAgent: &waProto.ClientPayload_UserAgent{
		Platform:       waProto.ClientPayload_UserAgent_WEB.Enum(),
		ReleaseChannel: waProto.ClientPayload_UserAgent_RELEASE.Enum(),
		AppVersion:     waVersion.ProtoAppVersion(),
		Mcc:            proto.String("000"),
		Mnc:            proto.String("000"),
		OsVersion:      proto.String("0.1.0"),
		Manufacturer:   proto.String(""),
		Device:         proto.String("Desktop"),
		OsBuildNumber:  proto.String("0.1.0"),

		LocaleLanguageIso6391:       proto.String("en"),
		LocaleCountryIso31661Alpha2: proto.String("en"),
	},
	WebInfo: &waProto.ClientPayload_WebInfo{
		WebSubPlatform: waProto.ClientPayload_WebInfo_WEB_BROWSER.Enum(),
	},
	ConnectType:   waProto.ClientPayload_WIFI_UNKNOWN.Enum(),
	ConnectReason: waProto.ClientPayload_USER_ACTIVATED.Enum(),
}

var DeviceProps = &waProto.DeviceProps{
	Os: proto.String("whatsmeow"),
	Version: &waProto.DeviceProps_AppVersion{
		Primary:   proto.Uint32(0),
		Secondary: proto.Uint32(1),
		Tertiary:  proto.Uint32(0),
	},
	PlatformType:    waProto.DeviceProps_UNKNOWN.Enum(),
	RequireFullSync: proto.Bool(false),
}

func SetOSInfo(name string, version [3]uint32) {
	DeviceProps.Os = &name
	DeviceProps.Version.Primary = &version[0]
	DeviceProps.Version.Secondary = &version[1]
	DeviceProps.Version.Tertiary = &version[2]
	BaseClientPayload.UserAgent.OsVersion = proto.String(fmt.Sprintf("%d.%d.%d", version[0], version[1], version[2]))
	BaseClientPayload.UserAgent.OsBuildNumber = BaseClientPayload.UserAgent.OsVersion
}

func (device *Device) getRegistrationPayload() *waProto.ClientPayload {
	payload := proto.Clone(BaseClientPayload).(*waProto.ClientPayload)
	regID := make([]byte, 4)
	binary.BigEndian.PutUint32(regID, device.RegistrationID)
	preKeyID := make([]byte, 4)
	binary.BigEndian.PutUint32(preKeyID, device.SignedPreKey.KeyID)
	deviceProps, _ := proto.Marshal(DeviceProps)
	payload.DevicePairingData = &waProto.ClientPayload_DevicePairingRegistrationData{
		ERegid:      regID,
		EKeytype:    []byte{ecc.DjbType},
		EIdent:      device.IdentityKey.Pub[:],
		ESkeyId:     preKeyID[1:],
		ESkeyVal:    device.SignedPreKey.Pub[:],
		ESkeySig:    device.SignedPreKey.Signature[:],
		BuildHash:   waVersionHash[:],
		DeviceProps: deviceProps,
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
