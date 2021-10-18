// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
)

// VerifiedName contains verified WhatsApp business details.
type VerifiedName struct {
	Certificate *waProto.VerifiedNameCertificate
	Details     *waProto.VerifiedNameDetails
}

// UserInfo contains info about a WhatsApp user.
type UserInfo struct {
	VerifiedName *VerifiedName
	Status       string
	PictureID    string
	Devices      []waBinary.JID
}

// ProfilePictureInfo contains the ID and URL for a WhatsApp user's profile picture.
type ProfilePictureInfo struct {
	URL  string // The full URL for the image, can be downloaded with a simple HTTP request.
	ID   string // The ID of the image. This is the same as UserInfo.PictureID.
	Type string // The type of image. Known types include "image" (full res) and "preview" (thumbnail).

	DirectPath string // The path to the image, probably not very useful
}

// IsOnWhatsAppResponse contains information received in response to checking if a phone number is on WhatsApp.
type IsOnWhatsAppResponse struct {
	Query string       // The query string used, plus @c.us at the end
	JID   waBinary.JID // The canonical user ID
	IsIn  bool         // Whether or not the phone is registered.

	VerifiedName *VerifiedName // If the phone is a business, the verified business details.
}

// IsOnWhatsApp checks if the given phone numbers are registered on WhatsApp.
// The phone numbers should be in international format, including the `+` prefix.
func (cli *Client) IsOnWhatsApp(phones []string) ([]IsOnWhatsAppResponse, error) {
	jids := make([]waBinary.JID, len(phones))
	for i := range jids {
		jids[i] = waBinary.NewJID(phones[i], waBinary.LegacyUserServer)
	}
	list, err := cli.usync(jids, "query", "interactive", []waBinary.Node{
		{Tag: "business", Content: []waBinary.Node{{Tag: "verified_name"}}},
		{Tag: "contact"},
	})
	if err != nil {
		return nil, err
	}
	output := make([]IsOnWhatsAppResponse, 0, len(jids))
	for _, child := range list.GetChildren() {
		jid, jidOK := child.Attrs["jid"].(waBinary.JID)
		if child.Tag != "user" || !jidOK {
			continue
		}
		var info IsOnWhatsAppResponse
		info.JID = jid
		info.VerifiedName, err = parseVerifiedName(child.GetChildByTag("business"))
		if err != nil {
			cli.Log.Warnf("Failed to parse %s's verified name details: %v", jid, err)
		}
		contactNode := child.GetChildByTag("contact")
		info.IsIn = contactNode.AttrGetter().String("type") == "in"
		contactQuery, _ := contactNode.Content.([]byte)
		info.Query = string(contactQuery)
		output = append(output, info)
	}
	return output, nil
}

// GetUserInfo gets basic user info (avatar, status, verified business name, device list).
func (cli *Client) GetUserInfo(jids []waBinary.JID) (map[waBinary.JID]UserInfo, error) {
	list, err := cli.usync(jids, "full", "background", []waBinary.Node{
		{Tag: "business", Content: []waBinary.Node{{Tag: "verified_name"}}},
		{Tag: "status"},
		{Tag: "picture"},
		{Tag: "devices", Attrs: waBinary.Attrs{"version": "2"}},
	})
	if err != nil {
		return nil, err
	}
	respData := make(map[waBinary.JID]UserInfo, len(jids))
	for _, child := range list.GetChildren() {
		jid, jidOK := child.Attrs["jid"].(waBinary.JID)
		if child.Tag != "user" || !jidOK {
			continue
		}
		verifiedName, err := parseVerifiedName(child.GetChildByTag("business"))
		if err != nil {
			cli.Log.Warnf("Failed to parse %s's verified name details: %v", jid, err)
		}
		status, _ := child.GetChildByTag("status").Content.([]byte)
		pictureID, _ := child.GetChildByTag("picture").Attrs["id"].(string)
		devices := parseDeviceList(jid.User, child.GetChildByTag("devices"), nil, nil)
		respData[jid] = UserInfo{
			VerifiedName: verifiedName,
			Status:       string(status),
			PictureID:    pictureID,
			Devices:      devices,
		}
	}
	return respData, nil
}

// GetUserDevices gets the list of devices that the given user has.
func (cli *Client) GetUserDevices(jids []waBinary.JID) ([]waBinary.JID, error) {
	list, err := cli.usync(jids, "query", "message", []waBinary.Node{
		{Tag: "devices", Attrs: waBinary.Attrs{"version": "2"}},
	})
	if err != nil {
		return nil, err
	}

	var devices []waBinary.JID
	for _, user := range list.GetChildren() {
		jid, jidOK := user.Attrs["jid"].(waBinary.JID)
		if user.Tag != "user" || !jidOK {
			continue
		}
		parseDeviceList(jid.User, user.GetChildByTag("devices"), &devices, cli.Store.ID)
	}

	return devices, nil
}

// GetProfilePicture gets the URL where you can download a WhatsApp user's profile picture.
func (cli *Client) GetProfilePicture(jid waBinary.JID, preview bool) (*ProfilePictureInfo, error) {
	attrs := waBinary.Attrs{
		"query": "url",
	}
	if preview {
		attrs["type"] = "preview"
	} else {
		attrs["type"] = "image"
	}
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:profile:picture",
		Type:      "get",
		To:        jid,
		Content: []waBinary.Node{{
			Tag:   "picture",
			Attrs: attrs,
		}},
	})
	if err != nil {
		return nil, err
	}
	picture, ok := resp.GetOptionalChildByTag("picture")
	if !ok {
		return nil, fmt.Errorf("missing <picture> element in response to profile picture query")
	}
	var info ProfilePictureInfo
	ag := picture.AttrGetter()
	info.ID = ag.String("id")
	info.URL = ag.String("url")
	info.Type = ag.String("type")
	info.DirectPath = ag.String("direct_path")
	if !ag.OK() {
		return &info, ag.Error()
	}
	return &info, nil
}

func parseVerifiedName(businessNode waBinary.Node) (*VerifiedName, error) {
	if businessNode.Tag != "business" {
		return nil, nil
	}
	verifiedNameNode, ok := businessNode.GetOptionalChildByTag("verified_name")
	if !ok {
		return nil, nil
	}
	rawCert, ok := verifiedNameNode.Content.([]byte)
	if !ok {
		return nil, nil
	}

	var cert waProto.VerifiedNameCertificate
	err := proto.Unmarshal(rawCert, &cert)
	if err != nil {
		return nil, err
	}
	fmt.Printf("%+v\n", &cert)
	var certDetails waProto.VerifiedNameDetails
	err = proto.Unmarshal(cert.GetDetails(), &certDetails)
	if err != nil {
		return nil, err
	}
	fmt.Printf("%+v\n", &certDetails)
	return &VerifiedName{
		Certificate: &cert,
		Details:     &certDetails,
	}, nil
}

func parseDeviceList(user string, deviceNode waBinary.Node, appendTo *[]waBinary.JID, ignore *waBinary.JID) []waBinary.JID {
	deviceList := deviceNode.GetChildByTag("device-list")
	if deviceNode.Tag != "devices" || deviceList.Tag != "device-list" {
		return nil
	}
	children := deviceList.GetChildren()
	if appendTo == nil {
		arr := make([]waBinary.JID, 0, len(children))
		appendTo = &arr
	}
	for _, device := range children {
		deviceID, ok := device.AttrGetter().GetInt64("id", true)
		if device.Tag != "device" || !ok {
			continue
		}
		deviceJID := waBinary.NewADJID(user, 0, byte(deviceID))
		if ignore == nil || deviceJID != *ignore {
			*appendTo = append(*appendTo, deviceJID)
		}
	}
	return *appendTo
}

func (cli *Client) usync(jids []waBinary.JID, mode, context string, query []waBinary.Node) (*waBinary.Node, error) {
	userList := make([]waBinary.Node, len(jids))
	for i, jid := range jids {
		userList[i].Tag = "user"
		if jid.AD {
			jid.AD = false
		}
		switch jid.Server {
		case waBinary.LegacyUserServer:
			userList[i].Content = []waBinary.Node{{
				Tag:     "contact",
				Content: jid.String(),
			}}
		case waBinary.DefaultUserServer:
			userList[i].Attrs = waBinary.Attrs{"jid": jid}
		default:
			return nil, fmt.Errorf("unknown user server '%s'", jid.Server)
		}
	}
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "usync",
		Type:      "get",
		To:        waBinary.ServerJID,
		Content: []waBinary.Node{{
			Tag: "usync",
			Attrs: waBinary.Attrs{
				"sid":     cli.generateRequestID(),
				"mode":    mode,
				"last":    "true",
				"index":   "0",
				"context": context,
			},
			Content: []waBinary.Node{
				{Tag: "query", Content: query},
				{Tag: "list", Content: userList},
			},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send usync query: %w", err)
	} else if usync, ok := resp.GetOptionalChildByTag("usync"); !ok {
		return nil, fmt.Errorf("missing <usync> element in response to usync query")
	} else if list, ok := usync.GetOptionalChildByTag("list"); !ok {
		return nil, fmt.Errorf("missing <list> element in response to usync query")
	} else {
		return list, err
	}
}
