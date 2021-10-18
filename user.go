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

type VerifiedName struct {
	Certificate *waProto.VerifiedNameCertificate
	Details     *waProto.VerifiedNameDetails
}

type UserInfo struct {
	VerifiedName *VerifiedName
	Status       string
	PictureID    string
	Devices      []waBinary.JID
}

func (cli *Client) GetUserInfo(jids []waBinary.JID) (map[waBinary.JID]UserInfo, error) {
	list, err := cli.usync(jids, "full", "background", []waBinary.Node{
		{Tag: "business", Content: []waBinary.Node{{Tag: "verified_name"}}},
		{Tag: "status"},
		{Tag: "picture"},
		{Tag: "devices", Attrs: map[string]interface{}{"version": "2"}},
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
		{Tag: "devices", Attrs: map[string]interface{}{"version": "2"}},
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
		Details: &certDetails,
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
		userList[i].Attrs = map[string]interface{}{"jid": waBinary.NewJID(jid.User, waBinary.DefaultUserServer)}
	}
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "usync",
		Type:      "get",
		To:        waBinary.ServerJID,
		Content: []waBinary.Node{{
			Tag: "usync",
			Attrs: map[string]interface{}{
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
