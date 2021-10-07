// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	"crypto/md5"
	"encoding/binary"
	"strconv"
	"strings"

	"github.com/RadicalApp/libsignal-protocol-go/ecc"
	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
)

// waVersion is the WhatsApp web client version
var waVersion = []int{2, 2138, 10}

// waVersionHash is the md5 hash of a dot-separated waVersion
var waVersionHash [16]byte

func init() {
	waVersionParts := make([]string, len(waVersion))
	for i, part := range waVersion {
		waVersionParts[i] = strconv.Itoa(part)
	}
	waVersionString := strings.Join(waVersionParts, ".")
	waVersionHash = md5.Sum([]byte(waVersionString))
}

type Session struct {
	NoiseKey          *KeyPair
	SignedIdentityKey *KeyPair
	SignedPreKey      *SignedKeyPair
	RegistrationID    uint16
	AdvSecretKey      []byte
	ID                *waBinary.FullJID
}

var BaseClientPayload = &waProto.ClientPayload{
	UserAgent: &waProto.UserAgent{
		Platform: waProto.UserAgent_WEB.Enum(),
		AppVersion: &waProto.AppVersion{
			Primary:   proto.Uint32(uint32(waVersion[0])),
			Secondary: proto.Uint32(uint32(waVersion[1])),
			Tertiary:  proto.Uint32(uint32(waVersion[2])),
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
	PlatformType:    waProto.CompanionProps_FIREFOX.Enum(),
	RequireFullSync: proto.Bool(false),
}

func (sess *Session) getRegistrationPayload() *waProto.ClientPayload {
	payload := proto.Clone(BaseClientPayload).(*waProto.ClientPayload)
	regID := make([]byte, 4)
	binary.BigEndian.PutUint32(regID, uint32(sess.RegistrationID))
	preKeyID := make([]byte, 4)
	binary.BigEndian.PutUint32(preKeyID, uint32(sess.SignedPreKey.KeyID))
	companionProps, _ := proto.Marshal(CompanionProps)
	payload.RegData = &waProto.CompanionRegData{
		ERegid:         regID,
		EKeytype:       []byte{ecc.DjbType},
		EIdent:         sess.SignedIdentityKey.Pub[:],
		ESkeyId:        preKeyID[1:],
		ESkeyVal:       sess.SignedPreKey.Pub[:],
		ESkeySig:       sess.SignedPreKey.Signature,
		BuildHash:      waVersionHash[:],
		CompanionProps: companionProps,
	}
	payload.Passive = proto.Bool(false)
	return payload
}

func (sess *Session) getLoginPayload() *waProto.ClientPayload {
	payload := proto.Clone(BaseClientPayload).(*waProto.ClientPayload)
	payload.Username = proto.Uint64(sess.ID.UserInt())
	payload.Device = proto.Uint32(uint32(sess.ID.Device))
	payload.Passive = proto.Bool(true)
	return payload
}

func (sess *Session) getClientPayload() *waProto.ClientPayload {
	if sess.ID != nil {
		return sess.getLoginPayload()
	} else {
		return sess.getRegistrationPayload()
	}
}
