// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const BusinessMessageLinkPrefix = "https://wa.me/message/"
const BusinessMessageLinkDirectPrefix = "https://api.whatsapp.com/message/"

// ResolveBusinessMessageLink resolves a business message short link and returns the target JID, business name and
// text to prefill in the input field (if any).
//
// The links look like https://wa.me/message/<code> or https://api.whatsapp.com/message/<code>. You can either provide
// the full link, or just the <code> part.
func (cli *Client) ResolveBusinessMessageLink(code string) (*types.BusinessMessageLinkTarget, error) {
	code = strings.TrimPrefix(code, BusinessMessageLinkPrefix)
	code = strings.TrimPrefix(code, BusinessMessageLinkDirectPrefix)

	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:qr",
		Type:      "get",
		// WhatsApp android doesn't seem to have a "to" field for this one at all, not sure why but it works
		Content: []waBinary.Node{{
			Tag: "qr",
			Attrs: waBinary.Attrs{
				"code": code,
			},
		}},
	})
	if errors.Is(err, ErrIQNotFound) {
		return nil, wrapIQError(ErrBusinessMessageLinkNotFound, err)
	} else if err != nil {
		return nil, err
	}
	qrChild, ok := resp.GetOptionalChildByTag("qr")
	if !ok {
		return nil, &ElementMissingError{Tag: "qr", In: "response to business message link query"}
	}
	var target types.BusinessMessageLinkTarget
	ag := qrChild.AttrGetter()
	target.JID = ag.JID("jid")
	target.PushName = ag.String("notify")
	messageChild, ok := qrChild.GetOptionalChildByTag("message")
	if ok {
		messageBytes, _ := messageChild.Content.([]byte)
		target.Message = string(messageBytes)
	}
	businessChild, ok := qrChild.GetOptionalChildByTag("business")
	if ok {
		bag := businessChild.AttrGetter()
		target.IsSigned = bag.OptionalBool("is_signed")
		target.VerifiedName = bag.OptionalString("verified_name")
		target.VerifiedLevel = bag.OptionalString("verified_level")
	}
	return &target, ag.Error()
}

// IsOnWhatsApp checks if the given phone numbers are registered on WhatsApp.
// The phone numbers should be in international format, including the `+` prefix.
func (cli *Client) IsOnWhatsApp(phones []string) ([]types.IsOnWhatsAppResponse, error) {
	jids := make([]types.JID, len(phones))
	for i := range jids {
		jids[i] = types.NewJID(phones[i], types.LegacyUserServer)
	}
	list, err := cli.usync(jids, "query", "interactive", []waBinary.Node{
		{Tag: "business", Content: []waBinary.Node{{Tag: "verified_name"}}},
		{Tag: "contact"},
	})
	if err != nil {
		return nil, err
	}
	output := make([]types.IsOnWhatsAppResponse, 0, len(jids))
	querySuffix := "@" + types.LegacyUserServer
	for _, child := range list.GetChildren() {
		jid, jidOK := child.Attrs["jid"].(types.JID)
		if child.Tag != "user" || !jidOK {
			continue
		}
		var info types.IsOnWhatsAppResponse
		info.JID = jid
		info.VerifiedName, err = parseVerifiedName(child.GetChildByTag("business"))
		if err != nil {
			cli.Log.Warnf("Failed to parse %s's verified name details: %v", jid, err)
		}
		contactNode := child.GetChildByTag("contact")
		info.IsIn = contactNode.AttrGetter().String("type") == "in"
		contactQuery, _ := contactNode.Content.([]byte)
		info.Query = strings.TrimSuffix(string(contactQuery), querySuffix)
		output = append(output, info)
	}
	return output, nil
}

// GetUserInfo gets basic user info (avatar, status, verified business name, device list).
func (cli *Client) GetUserInfo(jids []types.JID) (map[types.JID]types.UserInfo, error) {
	list, err := cli.usync(jids, "full", "background", []waBinary.Node{
		{Tag: "business", Content: []waBinary.Node{{Tag: "verified_name"}}},
		{Tag: "status"},
		{Tag: "picture"},
		{Tag: "devices", Attrs: waBinary.Attrs{"version": "2"}},
	})
	if err != nil {
		return nil, err
	}
	respData := make(map[types.JID]types.UserInfo, len(jids))
	for _, child := range list.GetChildren() {
		jid, jidOK := child.Attrs["jid"].(types.JID)
		if child.Tag != "user" || !jidOK {
			continue
		}
		var info types.UserInfo
		verifiedName, err := parseVerifiedName(child.GetChildByTag("business"))
		if err != nil {
			cli.Log.Warnf("Failed to parse %s's verified name details: %v", jid, err)
		}
		status, _ := child.GetChildByTag("status").Content.([]byte)
		info.Status = string(status)
		info.PictureID, _ = child.GetChildByTag("picture").Attrs["id"].(string)
		info.Devices = parseDeviceList(jid.User, child.GetChildByTag("devices"))
		if verifiedName != nil {
			cli.updateBusinessName(jid, nil, verifiedName.Details.GetVerifiedName())
		}
		respData[jid] = info
	}
	return respData, nil
}

// GetUserDevices gets the list of devices that the given user has. The input should be a list of
// regular JIDs, and the output will be a list of AD JIDs. The local device will not be included in
// the output even if the user's JID is included in the input. All other devices will be included.
func (cli *Client) GetUserDevices(jids []types.JID) ([]types.JID, error) {
	cli.userDevicesCacheLock.Lock()
	defer cli.userDevicesCacheLock.Unlock()

	var devices, jidsToSync []types.JID
	for _, jid := range jids {
		cached, ok := cli.userDevicesCache[jid]
		if ok && len(cached) > 0 {
			devices = append(devices, cached...)
		} else {
			jidsToSync = append(jidsToSync, jid)
		}
	}
	if len(jidsToSync) == 0 {
		return devices, nil
	}

	list, err := cli.usync(jidsToSync, "query", "message", []waBinary.Node{
		{Tag: "devices", Attrs: waBinary.Attrs{"version": "2"}},
	})
	if err != nil {
		return nil, err
	}

	for _, user := range list.GetChildren() {
		jid, jidOK := user.Attrs["jid"].(types.JID)
		if user.Tag != "user" || !jidOK {
			continue
		}
		userDevices := parseDeviceList(jid.User, user.GetChildByTag("devices"))
		cli.userDevicesCache[jid] = userDevices
		devices = append(devices, userDevices...)
	}

	return devices, nil
}

// GetProfilePictureInfo gets the URL where you can download a WhatsApp user's profile picture or group's photo.
// If the user or group doesn't have a profile picture, this returns nil with no error.
func (cli *Client) GetProfilePictureInfo(jid types.JID, preview bool) (*types.ProfilePictureInfo, error) {
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
	if errors.Is(err, ErrIQNotAuthorized) {
		return nil, wrapIQError(ErrProfilePictureUnauthorized, err)
	} else if errors.Is(err, ErrIQNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	picture, ok := resp.GetOptionalChildByTag("picture")
	if !ok {
		return nil, &ElementMissingError{Tag: "picture", In: "response to profile picture query"}
	}
	var info types.ProfilePictureInfo
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

func (cli *Client) handleHistoricalPushNames(names []*waProto.Pushname) {
	if cli.Store.Contacts == nil {
		return
	}
	cli.Log.Infof("Updating contact store with %d push names from history sync", len(names))
	for _, user := range names {
		if user.GetPushname() == "-" {
			continue
		}
		var changed bool
		if jid, err := types.ParseJID(user.GetId()); err != nil {
			cli.Log.Warnf("Failed to parse user ID '%s' in push name history sync: %v", user.GetId(), err)
		} else if changed, _, err = cli.Store.Contacts.PutPushName(jid, user.GetPushname()); err != nil {
			cli.Log.Warnf("Failed to store push name of %s from history sync: %v", err)
		} else if changed {
			cli.Log.Debugf("Got push name %s for %s in history sync", user.GetPushname(), jid)
		}
	}
}

func (cli *Client) updatePushName(user types.JID, messageInfo *types.MessageInfo, name string) {
	if cli.Store.Contacts == nil {
		return
	}
	user = user.ToNonAD()
	changed, previousName, err := cli.Store.Contacts.PutPushName(user, name)
	if err != nil {
		cli.Log.Errorf("Failed to save push name of %s in device store: %v", user, err)
	} else if changed {
		cli.Log.Debugf("Push name of %s changed from %s to %s, dispatching event", user, previousName, name)
		cli.dispatchEvent(&events.PushName{
			JID:         user,
			Message:     messageInfo,
			OldPushName: previousName,
			NewPushName: name,
		})
	}
}

func (cli *Client) updateBusinessName(user types.JID, messageInfo *types.MessageInfo, name string) {
	if cli.Store.Contacts == nil {
		return
	}
	changed, previousName, err := cli.Store.Contacts.PutBusinessName(user, name)
	if err != nil {
		cli.Log.Errorf("Failed to save business name of %s in device store: %v", user, err)
	} else if changed {
		cli.Log.Debugf("Business name of %s changed from %s to %s, dispatching event", user, previousName, name)
		cli.dispatchEvent(&events.BusinessName{
			JID:             user,
			Message:         messageInfo,
			OldBusinessName: previousName,
			NewBusinessName: name,
		})
	}
}

func parseVerifiedName(businessNode waBinary.Node) (*types.VerifiedName, error) {
	if businessNode.Tag != "business" {
		return nil, nil
	}
	verifiedNameNode, ok := businessNode.GetOptionalChildByTag("verified_name")
	if !ok {
		return nil, nil
	}
	return parseVerifiedNameContent(verifiedNameNode)
}

func parseVerifiedNameContent(verifiedNameNode waBinary.Node) (*types.VerifiedName, error) {
	rawCert, ok := verifiedNameNode.Content.([]byte)
	if !ok {
		return nil, nil
	}

	var cert waProto.VerifiedNameCertificate
	err := proto.Unmarshal(rawCert, &cert)
	if err != nil {
		return nil, err
	}
	var certDetails waProto.VerifiedNameDetails
	err = proto.Unmarshal(cert.GetDetails(), &certDetails)
	if err != nil {
		return nil, err
	}
	return &types.VerifiedName{
		Certificate: &cert,
		Details:     &certDetails,
	}, nil
}

func parseDeviceList(user string, deviceNode waBinary.Node) []types.JID {
	deviceList := deviceNode.GetChildByTag("device-list")
	if deviceNode.Tag != "devices" || deviceList.Tag != "device-list" {
		return nil
	}
	children := deviceList.GetChildren()
	devices := make([]types.JID, 0, len(children))
	for _, device := range children {
		deviceID, ok := device.AttrGetter().GetInt64("id", true)
		if device.Tag != "device" || !ok {
			continue
		}
		devices = append(devices, types.NewADJID(user, 0, byte(deviceID)))
	}
	return devices
}

func (cli *Client) usync(jids []types.JID, mode, context string, query []waBinary.Node) (*waBinary.Node, error) {
	userList := make([]waBinary.Node, len(jids))
	for i, jid := range jids {
		userList[i].Tag = "user"
		if jid.AD {
			jid.AD = false
		}
		switch jid.Server {
		case types.LegacyUserServer:
			userList[i].Content = []waBinary.Node{{
				Tag:     "contact",
				Content: jid.String(),
			}}
		case types.DefaultUserServer:
			userList[i].Attrs = waBinary.Attrs{"jid": jid}
		default:
			return nil, fmt.Errorf("unknown user server '%s'", jid.Server)
		}
	}
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "usync",
		Type:      "get",
		To:        types.ServerJID,
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
	} else if list, ok := resp.GetOptionalChildByTag("usync", "list"); !ok {
		return nil, &ElementMissingError{Tag: "list", In: "response to usync query"}
	} else {
		return &list, err
	}
}
