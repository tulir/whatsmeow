// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"encoding/json"
	"errors"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func (cli *Client) handleEncryptNotification(node *waBinary.Node) {
	from := node.AttrGetter().JID("from")
	if from == types.ServerJID {
		count := node.GetChildByTag("count")
		ag := count.AttrGetter()
		otksLeft := ag.Int("value")
		if !ag.OK() {
			cli.Log.Warnf("Didn't get number of OTKs left in encryption notification %s", node.XMLString())
			return
		}
		cli.Log.Infof("Got prekey count from server: %s", node.XMLString())
		if otksLeft < MinPreKeyCount {
			cli.uploadPreKeys()
		}
	} else if _, ok := node.GetOptionalChildByTag("identity"); ok {
		cli.Log.Debugf("Got identity change for %s: %s, deleting all identities/sessions for that number", from, node.XMLString())
		err := cli.Store.Identities.DeleteAllIdentities(from.User)
		if err != nil {
			cli.Log.Warnf("Failed to delete all identities of %s from store after identity change: %v", from, err)
		}
		err = cli.Store.Sessions.DeleteAllSessions(from.User)
		if err != nil {
			cli.Log.Warnf("Failed to delete all sessions of %s from store after identity change: %v", from, err)
		}
		ts := node.AttrGetter().UnixTime("t")
		cli.dispatchEvent(&events.IdentityChange{JID: from, Timestamp: ts})
	} else {
		cli.Log.Debugf("Got unknown encryption notification from server: %s", node.XMLString())
	}
}

func (cli *Client) handleAppStateNotification(node *waBinary.Node) {
	for _, collection := range node.GetChildrenByTag("collection") {
		ag := collection.AttrGetter()
		name := appstate.WAPatchName(ag.String("name"))
		version := ag.Uint64("version")
		cli.Log.Debugf("Got server sync notification that app state %s has updated to version %d", name, version)
		err := cli.FetchAppState(name, false, false)
		if errors.Is(err, ErrIQDisconnected) || errors.Is(err, ErrNotConnected) {
			// There are some app state changes right before a remote logout, so stop syncing if we're disconnected.
			cli.Log.Debugf("Failed to sync app state after notification: %v, not trying to sync other states", err)
			return
		} else if err != nil {
			cli.Log.Errorf("Failed to sync app state after notification: %v", err)
		}
	}
}

func (cli *Client) handlePictureNotification(node *waBinary.Node) {
	ts := node.AttrGetter().UnixTime("t")
	for _, child := range node.GetChildren() {
		ag := child.AttrGetter()
		var evt events.Picture
		evt.Timestamp = ts
		evt.JID = ag.JID("jid")
		evt.Author = ag.OptionalJIDOrEmpty("author")
		if child.Tag == "delete" {
			evt.Remove = true
		} else if child.Tag == "add" {
			evt.PictureID = ag.String("id")
		} else if child.Tag == "set" {
			// TODO sometimes there's a hash and no ID?
			evt.PictureID = ag.String("id")
		} else {
			continue
		}
		if !ag.OK() {
			cli.Log.Debugf("Ignoring picture change notification with unexpected attributes: %v", ag.Error())
			continue
		}
		cli.dispatchEvent(&evt)
	}
}

func (cli *Client) handleDeviceNotification(node *waBinary.Node) {
	cli.userDevicesCacheLock.Lock()
	defer cli.userDevicesCacheLock.Unlock()
	ag := node.AttrGetter()
	from := ag.JID("from")
	cached, ok := cli.userDevicesCache[from]
	if !ok {
		cli.Log.Debugf("No device list cached for %s, ignoring device list notification", from)
		return
	}
	cachedParticipantHash := participantListHashV2(cached.devices)
	for _, child := range node.GetChildren() {
		if child.Tag != "add" && child.Tag != "remove" {
			cli.Log.Debugf("Unknown device list change tag %s", child.Tag)
			continue
		}
		cag := child.AttrGetter()
		deviceHash := cag.String("device_hash")
		deviceChild, _ := child.GetOptionalChildByTag("device")
		changedDeviceJID := deviceChild.AttrGetter().JID("jid")
		switch child.Tag {
		case "add":
			cached.devices = append(cached.devices, changedDeviceJID)
		case "remove":
			for i, jid := range cached.devices {
				if jid == changedDeviceJID {
					cached.devices = append(cached.devices[:i], cached.devices[i+1:]...)
				}
			}
		case "update":
			// ???
		}
		newParticipantHash := participantListHashV2(cached.devices)
		if newParticipantHash == deviceHash {
			cli.Log.Debugf("%s's device list hash changed from %s to %s (%s). New hash matches", from, cachedParticipantHash, deviceHash, child.Tag)
			cli.userDevicesCache[from] = cached
		} else {
			cli.Log.Warnf("%s's device list hash changed from %s to %s (%s). New hash doesn't match (%s)", from, cachedParticipantHash, deviceHash, child.Tag, newParticipantHash)
			delete(cli.userDevicesCache, from)
		}
	}
}

func (cli *Client) handleFBDeviceNotification(node *waBinary.Node) {
	cli.userDevicesCacheLock.Lock()
	defer cli.userDevicesCacheLock.Unlock()
	jid := node.AttrGetter().JID("from")
	userDevices := parseFBDeviceList(jid, node.GetChildByTag("devices"))
	cli.userDevicesCache[jid] = userDevices
}

func (cli *Client) handleOwnDevicesNotification(node *waBinary.Node) {
	cli.userDevicesCacheLock.Lock()
	defer cli.userDevicesCacheLock.Unlock()
	ownID := cli.getOwnID().ToNonAD()
	if ownID.IsEmpty() {
		cli.Log.Debugf("Ignoring own device change notification, session was deleted")
		return
	}
	cached, ok := cli.userDevicesCache[ownID]
	if !ok {
		cli.Log.Debugf("Ignoring own device change notification, device list not cached")
		return
	}
	oldHash := participantListHashV2(cached.devices)
	expectedNewHash := node.AttrGetter().String("dhash")
	var newDeviceList []types.JID
	for _, child := range node.GetChildren() {
		jid := child.AttrGetter().JID("jid")
		if child.Tag == "device" && !jid.IsEmpty() {
			newDeviceList = append(newDeviceList, jid)
		}
	}
	newHash := participantListHashV2(newDeviceList)
	if newHash != expectedNewHash {
		cli.Log.Debugf("Received own device list change notification %s -> %s, but expected hash was %s", oldHash, newHash, expectedNewHash)
		delete(cli.userDevicesCache, ownID)
	} else {
		cli.Log.Debugf("Received own device list change notification %s -> %s", oldHash, newHash)
		cli.userDevicesCache[ownID] = deviceCache{devices: newDeviceList, dhash: expectedNewHash}
	}
}

func (cli *Client) handleBlocklist(node *waBinary.Node) {
	ag := node.AttrGetter()
	evt := events.Blocklist{
		Action:    events.BlocklistAction(ag.OptionalString("action")),
		DHash:     ag.String("dhash"),
		PrevDHash: ag.OptionalString("prev_dhash"),
	}
	for _, child := range node.GetChildren() {
		ag := child.AttrGetter()
		change := events.BlocklistChange{
			JID:    ag.JID("jid"),
			Action: events.BlocklistChangeAction(ag.String("action")),
		}
		if !ag.OK() {
			cli.Log.Warnf("Unexpected data in blocklist event child %v: %v", child.XMLString(), ag.Error())
			continue
		}
		evt.Changes = append(evt.Changes, change)
	}
	cli.dispatchEvent(&evt)
}

func (cli *Client) handleAccountSyncNotification(node *waBinary.Node) {
	for _, child := range node.GetChildren() {
		switch child.Tag {
		case "privacy":
			cli.handlePrivacySettingsNotification(&child)
		case "devices":
			cli.handleOwnDevicesNotification(&child)
		case "picture":
			cli.dispatchEvent(&events.Picture{
				Timestamp: node.AttrGetter().UnixTime("t"),
				JID:       cli.getOwnID().ToNonAD(),
			})
		case "blocklist":
			cli.handleBlocklist(&child)
		default:
			cli.Log.Debugf("Unhandled account sync item %s", child.Tag)
		}
	}
}

func (cli *Client) handlePrivacyTokenNotification(node *waBinary.Node) {
	ownID := cli.getOwnID().ToNonAD()
	if ownID.IsEmpty() {
		cli.Log.Debugf("Ignoring privacy token notification, session was deleted")
		return
	}
	tokens := node.GetChildByTag("tokens")
	if tokens.Tag != "tokens" {
		cli.Log.Warnf("privacy_token notification didn't contain <tokens> tag")
		return
	}
	parentAG := node.AttrGetter()
	sender := parentAG.JID("from")
	if !parentAG.OK() {
		cli.Log.Warnf("privacy_token notification didn't have a sender (%v)", parentAG.Error())
		return
	}
	for _, child := range tokens.GetChildren() {
		ag := child.AttrGetter()
		if child.Tag != "token" {
			cli.Log.Warnf("privacy_token notification contained unexpected <%s> tag", child.Tag)
		} else if targetUser := ag.JID("jid"); targetUser != ownID {
			cli.Log.Warnf("privacy_token notification contained token for different user %s", targetUser)
		} else if tokenType := ag.String("type"); tokenType != "trusted_contact" {
			cli.Log.Warnf("privacy_token notification contained unexpected token type %s", tokenType)
		} else if token, ok := child.Content.([]byte); !ok {
			cli.Log.Warnf("privacy_token notification contained non-binary token")
		} else {
			timestamp := ag.UnixTime("t")
			if !ag.OK() {
				cli.Log.Warnf("privacy_token notification is missing some fields: %v", ag.Error())
			}
			err := cli.Store.PrivacyTokens.PutPrivacyTokens(store.PrivacyToken{
				User:      sender,
				Token:     token,
				Timestamp: timestamp,
			})
			if err != nil {
				cli.Log.Errorf("Failed to save privacy token from %s: %v", sender, err)
			} else {
				cli.Log.Debugf("Stored privacy token from %s (ts: %v)", sender, timestamp)
			}
		}
	}
}

func (cli *Client) parseNewsletterMessages(node *waBinary.Node) []*types.NewsletterMessage {
	children := node.GetChildren()
	output := make([]*types.NewsletterMessage, 0, len(children))
	for _, child := range children {
		if child.Tag != "message" {
			continue
		}
		msg := types.NewsletterMessage{
			MessageServerID: child.AttrGetter().Int("server_id"),
			ViewsCount:      0,
			ReactionCounts:  nil,
		}
		for _, subchild := range child.GetChildren() {
			switch subchild.Tag {
			case "plaintext":
				byteContent, ok := subchild.Content.([]byte)
				if ok {
					msg.Message = new(waProto.Message)
					err := proto.Unmarshal(byteContent, msg.Message)
					if err != nil {
						cli.Log.Warnf("Failed to unmarshal newsletter message: %v", err)
						msg.Message = nil
					}
				}
			case "views_count":
				msg.ViewsCount = subchild.AttrGetter().Int("count")
			case "reactions":
				msg.ReactionCounts = make(map[string]int)
				for _, reaction := range subchild.GetChildren() {
					rag := reaction.AttrGetter()
					msg.ReactionCounts[rag.String("code")] = rag.Int("count")
				}
			}
		}
		output = append(output, &msg)
	}
	return output
}

func (cli *Client) handleNewsletterNotification(node *waBinary.Node) {
	ag := node.AttrGetter()
	liveUpdates := node.GetChildByTag("live_updates")
	cli.dispatchEvent(&events.NewsletterLiveUpdate{
		JID:      ag.JID("from"),
		Time:     ag.UnixTime("t"),
		Messages: cli.parseNewsletterMessages(&liveUpdates),
	})
}

type newsLetterEventWrapper struct {
	Data newsletterEvent `json:"data"`
}

type newsletterEvent struct {
	Join       *events.NewsletterJoin       `json:"xwa2_notify_newsletter_on_join"`
	Leave      *events.NewsletterLeave      `json:"xwa2_notify_newsletter_on_leave"`
	MuteChange *events.NewsletterMuteChange `json:"xwa2_notify_newsletter_on_mute_change"`
	// _on_admin_metadata_update -> id, thread_metadata, messages
	// _on_metadata_update
	// _on_state_change -> id, is_requestor, state
}

func (cli *Client) handleMexNotification(node *waBinary.Node) {
	for _, child := range node.GetChildren() {
		if child.Tag != "update" {
			continue
		}
		childData, ok := child.Content.([]byte)
		if !ok {
			continue
		}
		var wrapper newsLetterEventWrapper
		err := json.Unmarshal(childData, &wrapper)
		if err != nil {
			cli.Log.Errorf("Failed to unmarshal JSON in mex event: %v", err)
			continue
		}
		if wrapper.Data.Join != nil {
			cli.dispatchEvent(wrapper.Data.Join)
		} else if wrapper.Data.Leave != nil {
			cli.dispatchEvent(wrapper.Data.Leave)
		} else if wrapper.Data.MuteChange != nil {
			cli.dispatchEvent(wrapper.Data.MuteChange)
		}
	}
}

func (cli *Client) handleNotification(node *waBinary.Node) {
	ag := node.AttrGetter()
	notifType := ag.String("type")
	if !ag.OK() {
		return
	}
	go cli.sendAck(node)
	switch notifType {
	case "encrypt":
		go cli.handleEncryptNotification(node)
	case "server_sync":
		go cli.handleAppStateNotification(node)
	case "account_sync":
		go cli.handleAccountSyncNotification(node)
	case "devices":
		go cli.handleDeviceNotification(node)
	case "fbid:devices":
		go cli.handleFBDeviceNotification(node)
	case "w:gp2":
		evt, err := cli.parseGroupNotification(node)
		if err != nil {
			cli.Log.Errorf("Failed to parse group notification: %v", err)
		} else {
			go cli.dispatchEvent(evt)
		}
	case "picture":
		go cli.handlePictureNotification(node)
	case "mediaretry":
		go cli.handleMediaRetryNotification(node)
	case "privacy_token":
		go cli.handlePrivacyTokenNotification(node)
	case "link_code_companion_reg":
		go cli.tryHandleCodePairNotification(node)
	case "newsletter":
		go cli.handleNewsletterNotification(node)
	case "mex":
		go cli.handleMexNotification(node)
	// Other types: business, disappearing_mode, server, status, pay, psa
	default:
		cli.Log.Debugf("Unhandled notification with type %s", notifType)
	}
}
