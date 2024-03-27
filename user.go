// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
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
const ContactQRLinkPrefix = "https://wa.me/qr/"
const BusinessMessageLinkDirectPrefix = "https://api.whatsapp.com/message/"
const ContactQRLinkDirectPrefix = "https://api.whatsapp.com/qr/"
const NewsletterLinkPrefix = "https://whatsapp.com/channel/"

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
		Type:      iqGet,
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

// ResolveContactQRLink resolves a link from a contact share QR code and returns the target JID and push name.
//
// The links look like https://wa.me/qr/<code> or https://api.whatsapp.com/qr/<code>. You can either provide
// the full link, or just the <code> part.
func (cli *Client) ResolveContactQRLink(code string) (*types.ContactQRLinkTarget, error) {
	code = strings.TrimPrefix(code, ContactQRLinkPrefix)
	code = strings.TrimPrefix(code, ContactQRLinkDirectPrefix)

	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:qr",
		Type:      iqGet,
		Content: []waBinary.Node{{
			Tag: "qr",
			Attrs: waBinary.Attrs{
				"code": code,
			},
		}},
	})
	if errors.Is(err, ErrIQNotFound) {
		return nil, wrapIQError(ErrContactQRLinkNotFound, err)
	} else if err != nil {
		return nil, err
	}
	qrChild, ok := resp.GetOptionalChildByTag("qr")
	if !ok {
		return nil, &ElementMissingError{Tag: "qr", In: "response to contact link query"}
	}
	var target types.ContactQRLinkTarget
	ag := qrChild.AttrGetter()
	target.JID = ag.JID("jid")
	target.PushName = ag.OptionalString("notify")
	target.Type = ag.String("type")
	return &target, ag.Error()
}

// GetContactQRLink gets your own contact share QR link that can be resolved using ResolveContactQRLink
// (or scanned with the official apps when encoded as a QR code).
//
// If the revoke parameter is set to true, it will ask the server to revoke the previous link and generate a new one.
func (cli *Client) GetContactQRLink(revoke bool) (string, error) {
	action := "get"
	if revoke {
		action = "revoke"
	}
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:qr",
		Type:      iqSet,
		Content: []waBinary.Node{{
			Tag: "qr",
			Attrs: waBinary.Attrs{
				"type":   "contact",
				"action": action,
			},
		}},
	})
	if err != nil {
		return "", err
	}
	qrChild, ok := resp.GetOptionalChildByTag("qr")
	if !ok {
		return "", &ElementMissingError{Tag: "qr", In: "response to own contact link fetch"}
	}
	ag := qrChild.AttrGetter()
	return ag.String("code"), ag.Error()
}

// SetStatusMessage updates the current user's status text, which is shown in the "About" section in the user profile.
//
// This is different from the ephemeral status broadcast messages. Use SendMessage to types.StatusBroadcastJID to send
// such messages.
func (cli *Client) SetStatusMessage(msg string) error {
	_, err := cli.sendIQ(infoQuery{
		Namespace: "status",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag:     "status",
			Content: msg,
		}},
	})
	return err
}

// IsOnWhatsApp checks if the given phone numbers are registered on WhatsApp.
// The phone numbers should be in international format, including the `+` prefix.
func (cli *Client) IsOnWhatsApp(phones []string) ([]types.IsOnWhatsAppResponse, error) {
	jids := make([]types.JID, len(phones))
	for i := range jids {
		jids[i] = types.NewJID(phones[i], types.LegacyUserServer)
	}
	list, err := cli.usync(context.TODO(), jids, "query", "interactive", []waBinary.Node{
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
	list, err := cli.usync(context.TODO(), jids, "full", "background", []waBinary.Node{
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

func (cli *Client) parseBusinessProfile(node *waBinary.Node) (*types.BusinessProfile, error) {
	profileNode := node.GetChildByTag("profile")
	jid, ok := profileNode.AttrGetter().GetJID("jid", true)
	if !ok {
		return nil, errors.New("missing jid in business profile")
	}
	address := string(profileNode.GetChildByTag("address").Content.([]byte))
	email := string(profileNode.GetChildByTag("email").Content.([]byte))
	businessHour := profileNode.GetChildByTag("business_hours")
	businessHourTimezone := businessHour.AttrGetter().String("timezone")
	businessHoursConfigs := businessHour.GetChildren()
	businessHours := make([]types.BusinessHoursConfig, 0)
	for _, config := range businessHoursConfigs {
		if config.Tag != "business_hours_config" {
			continue
		}
		dow := config.AttrGetter().String("dow")
		mode := config.AttrGetter().String("mode")
		openTime := config.AttrGetter().String("open_time")
		closeTime := config.AttrGetter().String("close_time")
		businessHours = append(businessHours, types.BusinessHoursConfig{
			DayOfWeek: dow,
			Mode:      mode,
			OpenTime:  openTime,
			CloseTime: closeTime,
		})
	}
	categoriesNode := profileNode.GetChildByTag("categories")
	categories := make([]types.Category, 0)
	for _, category := range categoriesNode.GetChildren() {
		if category.Tag != "category" {
			continue
		}
		id := category.AttrGetter().String("id")
		name := string(category.Content.([]byte))
		categories = append(categories, types.Category{
			ID:   id,
			Name: name,
		})
	}
	profileOptionsNode := profileNode.GetChildByTag("profile_options")
	profileOptions := make(map[string]string)
	for _, option := range profileOptionsNode.GetChildren() {
		profileOptions[option.Tag] = string(option.Content.([]byte))
	}
	return &types.BusinessProfile{
		JID:                   jid,
		Email:                 email,
		Address:               address,
		Categories:            categories,
		ProfileOptions:        profileOptions,
		BusinessHoursTimeZone: businessHourTimezone,
		BusinessHours:         businessHours,
	}, nil
}

// GetBusinessProfile gets the profile info of a WhatsApp business account
func (cli *Client) GetBusinessProfile(jid types.JID) (*types.BusinessProfile, error) {
	resp, err := cli.sendIQ(infoQuery{
		Type:      iqGet,
		To:        types.ServerJID,
		Namespace: "w:biz",
		Content: []waBinary.Node{{
			Tag: "business_profile",
			Attrs: waBinary.Attrs{
				"v": "244",
			},
			Content: []waBinary.Node{{
				Tag: "profile",
				Attrs: waBinary.Attrs{
					"jid": jid,
				},
			}},
		}},
	})
	if err != nil {
		return nil, err
	}
	node, ok := resp.GetOptionalChildByTag("business_profile")
	if !ok {
		return nil, &ElementMissingError{Tag: "business_profile", In: "response to business profile query"}
	}
	return cli.parseBusinessProfile(&node)
}

// GetUserDevices gets the list of devices that the given user has. The input should be a list of
// regular JIDs, and the output will be a list of AD JIDs. The local device will not be included in
// the output even if the user's JID is included in the input. All other devices will be included.
func (cli *Client) GetUserDevices(jids []types.JID) ([]types.JID, error) {
	return cli.GetUserDevicesContext(context.Background(), jids)
}

func (cli *Client) GetUserDevicesContext(ctx context.Context, jids []types.JID) ([]types.JID, error) {
	cli.userDevicesCacheLock.Lock()
	defer cli.userDevicesCacheLock.Unlock()

	var devices, jidsToSync, fbJIDsToSync []types.JID
	for _, jid := range jids {
		cached, ok := cli.userDevicesCache[jid]
		if ok && len(cached.devices) > 0 {
			devices = append(devices, cached.devices...)
		} else if jid.Server == types.MessengerServer {
			fbJIDsToSync = append(fbJIDsToSync, jid)
		} else {
			jidsToSync = append(jidsToSync, jid)
		}
	}
	if len(jidsToSync) > 0 {
		list, err := cli.usync(ctx, jidsToSync, "query", "message", []waBinary.Node{
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
			cli.userDevicesCache[jid] = deviceCache{devices: userDevices, dhash: participantListHashV2(userDevices)}
			devices = append(devices, userDevices...)
		}
	}

	if len(fbJIDsToSync) > 0 {
		list, err := cli.getFBIDDevices(ctx, fbJIDsToSync)
		if err != nil {
			return nil, err
		}
		for _, user := range list.GetChildren() {
			jid, jidOK := user.Attrs["jid"].(types.JID)
			if user.Tag != "user" || !jidOK {
				continue
			}
			userDevices := parseFBDeviceList(jid, user.GetChildByTag("devices"))
			cli.userDevicesCache[jid] = userDevices
			devices = append(devices, userDevices.devices...)
		}
	}

	return devices, nil
}

type GetProfilePictureParams struct {
	Preview     bool
	ExistingID  string
	IsCommunity bool
}

// GetProfilePictureInfo gets the URL where you can download a WhatsApp user's profile picture or group's photo.
//
// Optionally, you can pass the last known profile picture ID.
// If the profile picture hasn't changed, this will return nil with no error.
//
// To get a community photo, you should pass `IsCommunity: true`, as otherwise you may get a 401 error.
func (cli *Client) GetProfilePictureInfo(jid types.JID, params *GetProfilePictureParams) (*types.ProfilePictureInfo, error) {
	attrs := waBinary.Attrs{
		"query": "url",
	}
	var target, to types.JID
	if params == nil {
		params = &GetProfilePictureParams{}
	}
	if params.Preview {
		attrs["type"] = "preview"
	} else {
		attrs["type"] = "image"
	}
	if params.ExistingID != "" {
		attrs["id"] = params.ExistingID
	}
	var expectWrapped bool
	var content []waBinary.Node
	namespace := "w:profile:picture"
	if params.IsCommunity {
		target = types.EmptyJID
		namespace = "w:g2"
		to = jid
		attrs["parent_group_jid"] = jid
		expectWrapped = true
		content = []waBinary.Node{{
			Tag: "pictures",
			Content: []waBinary.Node{{
				Tag:   "picture",
				Attrs: attrs,
			}},
		}}
	} else {
		to = types.ServerJID
		target = jid
		content = []waBinary.Node{{
			Tag:   "picture",
			Attrs: attrs,
		}}
	}
	resp, err := cli.sendIQ(infoQuery{
		Namespace: namespace,
		Type:      "get",
		To:        to,
		Target:    target,
		Content:   content,
	})
	if errors.Is(err, ErrIQNotAuthorized) {
		return nil, wrapIQError(ErrProfilePictureUnauthorized, err)
	} else if errors.Is(err, ErrIQNotFound) {
		return nil, wrapIQError(ErrProfilePictureNotSet, err)
	} else if err != nil {
		return nil, err
	}
	if expectWrapped {
		pics, ok := resp.GetOptionalChildByTag("pictures")
		if !ok {
			return nil, &ElementMissingError{Tag: "pictures", In: "response to profile picture query"}
		}
		resp = &pics
	}
	picture, ok := resp.GetOptionalChildByTag("picture")
	if !ok {
		if params.ExistingID != "" {
			return nil, nil
		}
		return nil, &ElementMissingError{Tag: "picture", In: "response to profile picture query"}
	}
	var info types.ProfilePictureInfo
	ag := picture.AttrGetter()
	if ag.OptionalInt("status") == 304 {
		return nil, nil
	}
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
	var certDetails waProto.VerifiedNameCertificate_Details
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

func parseFBDeviceList(user types.JID, deviceList waBinary.Node) deviceCache {
	children := deviceList.GetChildren()
	devices := make([]types.JID, 0, len(children))
	for _, device := range children {
		deviceID, ok := device.AttrGetter().GetInt64("id", true)
		if device.Tag != "device" || !ok {
			continue
		}
		user.Device = uint16(deviceID)
		devices = append(devices, user)
		// TODO take identities here too?
	}
	// TODO do something with the icdc blob?
	return deviceCache{
		devices: devices,
		dhash:   deviceList.AttrGetter().String("dhash"),
	}
}

func (cli *Client) getFBIDDevices(ctx context.Context, jids []types.JID) (*waBinary.Node, error) {
	users := make([]waBinary.Node, len(jids))
	for i, jid := range jids {
		users[i].Tag = "user"
		users[i].Attrs = waBinary.Attrs{"jid": jid}
		// TODO include dhash for users
	}
	resp, err := cli.sendIQ(infoQuery{
		Context:   ctx,
		Namespace: "fbid:devices",
		Type:      iqGet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag:     "users",
			Content: users,
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send usync query: %w", err)
	} else if list, ok := resp.GetOptionalChildByTag("users"); !ok {
		return nil, &ElementMissingError{Tag: "users", In: "response to fbid devices query"}
	} else {
		return &list, err
	}
}

func (cli *Client) usync(ctx context.Context, jids []types.JID, mode, context string, query []waBinary.Node) (*waBinary.Node, error) {
	userList := make([]waBinary.Node, len(jids))
	for i, jid := range jids {
		userList[i].Tag = "user"
		jid = jid.ToNonAD()
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
		Context:   ctx,
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

func (cli *Client) parseBlocklist(node *waBinary.Node) *types.Blocklist {
	output := &types.Blocklist{
		DHash: node.AttrGetter().String("dhash"),
	}
	for _, child := range node.GetChildren() {
		ag := child.AttrGetter()
		blockedJID := ag.JID("jid")
		if !ag.OK() {
			cli.Log.Debugf("Ignoring contact blocked data with unexpected attributes: %v", ag.Error())
			continue
		}

		output.JIDs = append(output.JIDs, blockedJID)
	}
	return output
}

// GetBlocklist gets the list of users that this user has blocked.
func (cli *Client) GetBlocklist() (*types.Blocklist, error) {
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "blocklist",
		Type:      iqGet,
		To:        types.ServerJID,
	})
	if err != nil {
		return nil, err
	}
	list, ok := resp.GetOptionalChildByTag("list")
	if !ok {
		return nil, &ElementMissingError{Tag: "list", In: "response to blocklist query"}
	}
	return cli.parseBlocklist(&list), nil
}

// UpdateBlocklist updates the user's block list and returns the updated list.
func (cli *Client) UpdateBlocklist(jid types.JID, action events.BlocklistChangeAction) (*types.Blocklist, error) {
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "blocklist",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "item",
			Attrs: waBinary.Attrs{
				"jid":    jid,
				"action": string(action),
			},
		}},
	})
	list, ok := resp.GetOptionalChildByTag("list")
	if !ok {
		return nil, &ElementMissingError{Tag: "list", In: "response to blocklist update"}
	}
	return cli.parseBlocklist(&list), err
}
