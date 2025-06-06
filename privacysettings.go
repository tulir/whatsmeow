// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"strconv"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// TryFetchPrivacySettings will fetch the user's privacy settings, either from the in-memory cache or from the server.
func (cli *Client) TryFetchPrivacySettings(ctx context.Context, ignoreCache bool) (*types.PrivacySettings, error) {
	if cli == nil {
		return nil, ErrClientIsNil
	} else if val := cli.privacySettingsCache.Load(); val != nil && !ignoreCache {
		return val.(*types.PrivacySettings), nil
	}
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "privacy",
		Context:   ctx,
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
func (cli *Client) GetPrivacySettings(ctx context.Context) (settings types.PrivacySettings) {
	if cli == nil || cli.MessengerConfig != nil {
		return
	}
	settingsPtr, err := cli.TryFetchPrivacySettings(ctx, false)
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
func (cli *Client) SetPrivacySetting(ctx context.Context, name types.PrivacySettingType, value types.PrivacySetting, userJID []types.PrivacyJID, DHash int64) (settings types.PrivacySettings, err error) {
	settingsPtr, err := cli.TryFetchPrivacySettings(ctx, false)
	if err != nil {
		return settings, err
	}

	attr := waBinary.Attrs{
		"name":  string(name),
		"value": string(value),
	}

	userNodes := make([]waBinary.Node, len(userJID))
	if value == types.PrivacySettingContactBlacklist && len(userJID) > 0 {
		for i, item := range userJID {
			userNodes[i].Tag = "user"
			userNodes[i].Attrs = waBinary.Attrs{"action": string(item.Action), "jid": item.JID.ToNonAD().String()}
		}
	}

	if DHash > 0 {
		attr["dhash"] = DHash
	}

	_, err = cli.sendIQ(infoQuery{
		Namespace: "privacy",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "privacy",
			Content: []waBinary.Node{{
				Tag:     "category",
				Attrs:   attr,
				Content: userNodes,
			}},
		}},
	})
	if err != nil {
		return settings, err
	}

	var (
		dhash int64
		jids  []types.JID
	)
	if value == types.PrivacySettingContactBlacklist {
		jids, dhash = cli.GetPrivacyContactBlacklist(ctx, name)
		//<iq from="s.whatsapp.net" id="221.99-345" type="result"><privacy><category dhash="1749023801911" name="profile" value="contact_blacklist"/></privacy></iq>
	}

	settings = *settingsPtr
	switch name {
	case types.PrivacySettingTypeGroupAdd:
		settings.GroupAdd = types.PrivacySettingItem{
			Value: value,
			JID:   jids,
			DHash: dhash,
		}
	case types.PrivacySettingTypeLastSeen:
		settings.LastSeen = types.PrivacySettingItem{
			Value: value,
			JID:   jids,
			DHash: dhash,
		}

	case types.PrivacySettingTypeStatus:
		settings.Status = types.PrivacySettingItem{
			Value: value,
			JID:   jids,
			DHash: dhash,
		}
	case types.PrivacySettingTypeProfile:
		settings.Profile = types.PrivacySettingItem{
			Value: value,
			JID:   jids,
			DHash: dhash,
		}
	case types.PrivacySettingTypeReadReceipts:
		settings.ReadReceipts = types.PrivacySettingItem{
			Value: value,
			JID:   jids,
			DHash: dhash,
		}
	case types.PrivacySettingTypeOnline:
		settings.Online = types.PrivacySettingItem{
			Value: value,
			JID:   jids,
			DHash: dhash,
		}
	case types.PrivacySettingTypeCallAdd:
		settings.CallAdd = types.PrivacySettingItem{
			Value: value,
			JID:   jids,
			DHash: dhash,
		}
	}
	cli.privacySettingsCache.Store(&settings)
	return
}

func (cli *Client) GetPrivacyContactBlacklist(ctx context.Context, tag types.PrivacySettingType) (list []types.JID, DHash int64) {
	attrs := waBinary.Attrs{"name": string(tag), "value": string(types.PrivacySettingContactBlacklist)}

	nodes := []waBinary.Node{{
		Tag: "privacy",
		Content: []waBinary.Node{
			{
				Tag:   "list",
				Attrs: attrs,
			},
		},
	}}

	resp, err := cli.sendIQ(infoQuery{
		Namespace: "privacy",
		Type:      iqGet,
		Context:   ctx,
		To:        types.ServerJID,
		Content:   nodes,
	})
	if err != nil {
		return nil, 0
	}

	node, ok := resp.GetOptionalChildByTag("list")
	if !ok {
		return nil, 0
	}

	dhashValue, ok := node.AttrGetter().GetInt64("dhash", true)
	if ok {
		DHash = dhashValue
	}

	userList := node.GetChildrenByTag("user")
	list = make([]types.JID, len(userList))
	for v, i := range userList {
		jid, ok := i.AttrGetter().GetJID("jid", true)
		if ok {
			list[v] = jid
		}
	}

	return list, DHash
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

		var (
			list  []types.JID
			dhash int64
		)
		if value == types.PrivacySettingContactBlacklist {
			list, dhash = cli.GetPrivacyContactBlacklist(context.Background(), name)
		}

		switch name {
		case types.PrivacySettingTypeGroupAdd:
			settings.GroupAdd = types.PrivacySettingItem{
				Value: value,
				DHash: dhash,
				JID:   list,
			}
			evt.GroupAddChanged = true
		case types.PrivacySettingTypeLastSeen:
			settings.LastSeen = types.PrivacySettingItem{
				Value: value,
				DHash: dhash,
				JID:   list,
			}
			evt.LastSeenChanged = true
		case types.PrivacySettingTypeStatus:
			settings.Status = types.PrivacySettingItem{
				Value: value,
				DHash: dhash,
				JID:   list,
			}
			evt.StatusChanged = true
		case types.PrivacySettingTypeProfile:
			settings.Profile = types.PrivacySettingItem{
				Value: value,
				DHash: dhash,
				JID:   list,
			}
			evt.ProfileChanged = true
		case types.PrivacySettingTypeReadReceipts:
			settings.ReadReceipts = types.PrivacySettingItem{
				Value: value,
				DHash: dhash,
				JID:   list,
			}
			evt.ReadReceiptsChanged = true
		case types.PrivacySettingTypeOnline:
			settings.Online = types.PrivacySettingItem{
				Value: value,
				DHash: dhash,
				JID:   list,
			}
			evt.OnlineChanged = true
		case types.PrivacySettingTypeCallAdd:
			settings.CallAdd = types.PrivacySettingItem{
				Value: value,
				DHash: dhash,
				JID:   list,
			}
			evt.CallAddChanged = true
		}
	}
	return &evt
}

func (cli *Client) handlePrivacySettingsNotification(ctx context.Context, privacyNode *waBinary.Node) {
	cli.Log.Debugf("Parsing privacy settings change notification")
	settings, err := cli.TryFetchPrivacySettings(ctx, false)
	if err != nil {
		cli.Log.Errorf("Failed to fetch privacy settings when handling change: %v", err)
		return
	}
	evt := cli.parsePrivacySettings(privacyNode, settings)
	cli.privacySettingsCache.Store(settings)
	cli.dispatchEvent(evt)
}
