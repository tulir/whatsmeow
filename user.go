// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"fmt"

	waBinary "go.mau.fi/whatsmeow/binary"
)

// GetUserDevices gets the list of devices that the given user has.
func (cli *Client) GetUserDevices(jids []waBinary.JID, ignorePrimary bool) ([]waBinary.JID, error) {
	userList := make([]waBinary.Node, len(jids))
	for i, jid := range jids {
		userList[i].Tag = "user"
		userList[i].Attrs = map[string]interface{}{"jid": waBinary.NewJID(jid.User, waBinary.DefaultUserServer)}
	}
	res, err := cli.sendIQ(infoQuery{
		Namespace: "usync",
		Type:      "get",
		To:        waBinary.ServerJID,
		Content: []waBinary.Node{{
			Tag: "usync",
			Attrs: map[string]interface{}{
				"sid":     cli.generateRequestID(),
				"mode":    "query",
				"last":    "true",
				"index":   "0",
				"context": "message",
			},
			Content: []waBinary.Node{
				{Tag: "query", Content: []waBinary.Node{{
					Tag: "devices",
					Attrs: map[string]interface{}{
						"version": "2",
					},
				}}},
				{Tag: "list", Content: userList},
			},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send usync query: %w", err)
	}
	usync := res.GetChildByTag("usync")
	if usync.Tag != "usync" {
		return nil, fmt.Errorf("unexpected children in response to usync query")
	}
	list := usync.GetChildByTag("list")
	if list.Tag != "list" {
		return nil, fmt.Errorf("missing list inside usync tag")
	}

	var devices []waBinary.JID
	for _, user := range list.GetChildren() {
		jid, jidOK := user.Attrs["jid"].(waBinary.JID)
		if user.Tag != "user" || !jidOK {
			continue
		}
		deviceNode := user.GetChildByTag("devices")
		deviceList := deviceNode.GetChildByTag("device-list")
		if deviceNode.Tag != "devices" || deviceList.Tag != "device-list" {
			continue
		}
		for _, device := range deviceList.GetChildren() {
			deviceID, ok := device.AttrGetter().GetInt64("id", true)
			if device.Tag != "device" || !ok {
				continue
			}
			deviceJID := waBinary.NewADJID(jid.User, 0, byte(deviceID))
			if (deviceJID.Device > 0 || !ignorePrimary) && deviceJID != *cli.Store.ID {
				devices = append(devices, deviceJID)
			}
		}
	}

	return devices, nil
}
