// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"strconv"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// TryFetchPrivacySettings will fetch the user's privacy settings, either from the in-memory cache or from the server.
func (cli *Client) TryFetchPrivacySettings(ignoreCache bool) (*types.PrivacySettings, error) {
	if val := cli.privacySettingsCache.Load(); val != nil && !ignoreCache {
		return val.(*types.PrivacySettings), nil
	}
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "privacy",
		Type:      iqGet,
		To:        types.ServerJID,
		Content:   []waBinary.Node{{Tag: "privacy"}},
	})
	if err != nil {
		return nil, err
	}
	privacyNode, ok := resp.GetOptionalChildByTag("privacy")
	if !ok {
		return nil, &ElementMissingError{Tag: "privacy", In: "response to privacy settings query"}
	}
	var settings types.PrivacySettings
	cli.parsePrivacySettings(&privacyNode, &settings)
	cli.privacySettingsCache.Store(&settings)
	return &settings, nil
}

// GetPrivacySettings will get the user's privacy settings. If an error occurs while fetching them, the error will be
// logged, but the method will just return an empty struct.
func (cli *Client) GetPrivacySettings() (settings types.PrivacySettings) {
	if cli.MessengerConfig != nil {
		return
	}
	settingsPtr, err := cli.TryFetchPrivacySettings(false)
	if err != nil {
		cli.Log.Errorf("Failed to fetch privacy settings: %v", err)
	} else {
		settings = *settingsPtr
	}
	return
}

// SetPrivacySetting will set the given privacy setting to the given value.
// The privacy settings will be fetched from the server after the change and the new settings will be returned.
// If an error occurs while fetching the new settings, will return an empty struct.
func (cli *Client) SetPrivacySetting(name types.PrivacySettingType, value types.PrivacySetting) (settings types.PrivacySettings, err error) {
	settingsPtr, err := cli.TryFetchPrivacySettings(false)
	if err != nil {
		return settings, err
	}
	_, err = cli.sendIQ(infoQuery{
		Namespace: "privacy",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "privacy",
			Content: []waBinary.Node{{
				Tag: "category",
				Attrs: waBinary.Attrs{
					"name":  string(name),
					"value": string(value),
				},
			}},
		}},
	})
	if err != nil {
		return settings, err
	}
	settings = *settingsPtr
	switch name {
	case types.PrivacySettingTypeGroupAdd:
		settings.GroupAdd = value
	case types.PrivacySettingTypeLastSeen:
		settings.LastSeen = value
	case types.PrivacySettingTypeStatus:
		settings.Status = value
	case types.PrivacySettingTypeProfile:
		settings.Profile = value
	case types.PrivacySettingTypeReadReceipts:
		settings.ReadReceipts = value
	case types.PrivacySettingTypeOnline:
		settings.Online = value
	case types.PrivacySettingTypeCallAdd:
		settings.CallAdd = value
	}
	cli.privacySettingsCache.Store(&settings)
	return
}

// SetDefaultDisappearingTimer will set the default disappearing message timer.
func (cli *Client) SetDefaultDisappearingTimer(timer time.Duration) (err error) {
	_, err = cli.sendIQ(infoQuery{
		Namespace: "disappearing_mode",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "disappearing_mode",
			Attrs: waBinary.Attrs{
				"duration": strconv.Itoa(int(timer.Seconds())),
			},
		}},
	})
	return
}

func (cli *Client) parsePrivacySettings(privacyNode *waBinary.Node, settings *types.PrivacySettings) *events.PrivacySettings {
	var evt events.PrivacySettings
	for _, child := range privacyNode.GetChildren() {
		if child.Tag != "category" {
			continue
		}
		ag := child.AttrGetter()
		name := types.PrivacySettingType(ag.String("name"))
		value := types.PrivacySetting(ag.String("value"))
		switch name {
		case types.PrivacySettingTypeGroupAdd:
			settings.GroupAdd = value
			evt.GroupAddChanged = true
		case types.PrivacySettingTypeLastSeen:
			settings.LastSeen = value
			evt.LastSeenChanged = true
		case types.PrivacySettingTypeStatus:
			settings.Status = value
			evt.StatusChanged = true
		case types.PrivacySettingTypeProfile:
			settings.Profile = value
			evt.ProfileChanged = true
		case types.PrivacySettingTypeReadReceipts:
			settings.ReadReceipts = value
			evt.ReadReceiptsChanged = true
		case types.PrivacySettingTypeOnline:
			settings.Online = value
			evt.OnlineChanged = true
		case types.PrivacySettingTypeCallAdd:
			settings.CallAdd = value
			evt.CallAddChanged = true
		}
	}
	return &evt
}

func (cli *Client) handlePrivacySettingsNotification(privacyNode *waBinary.Node) {
	cli.Log.Debugf("Parsing privacy settings change notification")
	settings, err := cli.TryFetchPrivacySettings(false)
	if err != nil {
		cli.Log.Errorf("Failed to fetch privacy settings when handling change: %v", err)
		return
	}
	evt := cli.parsePrivacySettings(privacyNode, settings)
	// The data isn't be reliable if the fetch failed, so only cache if it didn't fail
	if err == nil {
		cli.privacySettingsCache.Store(settings)
	}
	cli.dispatchEvent(evt)
}
