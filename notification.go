// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"errors"

	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
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
	cachedParticipantHash := participantListHashV2(cached)
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
			cached = append(cached, changedDeviceJID)
		case "remove":
			for i, jid := range cached {
				if jid == changedDeviceJID {
					cached = append(cached[:i], cached[i+1:]...)
				}
			}
		case "update":
			// ???
		}
		newParticipantHash := participantListHashV2(cached)
		if newParticipantHash == deviceHash {
			cli.Log.Debugf("%s's device list hash changed from %s to %s (%s). New hash matches", from, cachedParticipantHash, deviceHash, child.Tag)
			cli.userDevicesCache[from] = cached
		} else {
			cli.Log.Warnf("%s's device list hash changed from %s to %s (%s). New hash doesn't match (%s)", from, cachedParticipantHash, deviceHash, child.Tag, newParticipantHash)
			delete(cli.userDevicesCache, from)
		}
	}
}

func (cli *Client) handleOwnDevicesNotification(node *waBinary.Node) {
	cli.userDevicesCacheLock.Lock()
	defer cli.userDevicesCacheLock.Unlock()
	cached, ok := cli.userDevicesCache[cli.Store.ID.ToNonAD()]
	if !ok {
		cli.Log.Debugf("Ignoring own device change notification, device list not cached")
		return
	}
	oldHash := participantListHashV2(cached)
	expectedNewHash := node.AttrGetter().String("dhash")
	var newDeviceList []types.JID
	for _, child := range node.GetChildren() {
		jid := child.AttrGetter().JID("jid")
		if child.Tag == "device" && !jid.IsEmpty() {
			jid.AD = true
			newDeviceList = append(newDeviceList, jid)
		}
	}
	newHash := participantListHashV2(newDeviceList)
	if newHash != expectedNewHash {
		cli.Log.Debugf("Received own device list change notification %s -> %s, but expected hash was %s", oldHash, newHash, expectedNewHash)
		delete(cli.userDevicesCache, cli.Store.ID.ToNonAD())
	} else {
		cli.Log.Debugf("Received own device list change notification %s -> %s", oldHash, newHash)
		cli.userDevicesCache[cli.Store.ID.ToNonAD()] = newDeviceList
	}
}

func (cli *Client) handleAccountSyncNotification(node *waBinary.Node) {
	for _, child := range node.GetChildren() {
		switch child.Tag {
		case "privacy":
			cli.handlePrivacySettingsNotification(&child)
		case "devices":
			cli.handleOwnDevicesNotification(&child)
		default:
			cli.Log.Debugf("Unhandled account sync item %s", child.Tag)
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
	// Other types: business, disappearing_mode, server, status, pay, psa, privacy_token
	default:
		cli.Log.Debugf("Unhandled notification with type %s", notifType)
	}
}
