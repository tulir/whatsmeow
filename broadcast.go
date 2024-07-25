// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"errors"
	"fmt"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func (cli *Client) getBroadcastListParticipants(jid types.JID) ([]types.JID, error) {
	var list []types.JID
	var err error
	if jid == types.StatusBroadcastJID {
		list, err = cli.getStatusBroadcastRecipients()
	} else {
		return nil, ErrBroadcastListUnsupported
	}
	if err != nil {
		return nil, err
	}
	ownID := cli.getOwnID().ToNonAD()
	if ownID.IsEmpty() {
		return nil, ErrNotLoggedIn
	}

	selfIndex := -1
	for i, participant := range list {
		if participant.User == ownID.User {
			selfIndex = i
			break
		}
	}
	if selfIndex >= 0 {
		if cli.DontSendSelfBroadcast {
			list[selfIndex] = list[len(list)-1]
			list = list[:len(list)-1]
		}
	} else if !cli.DontSendSelfBroadcast {
		list = append(list, ownID)
	}
	return list, nil
}

func (cli *Client) getStatusBroadcastRecipients() ([]types.JID, error) {
	statusPrivacyOptions, err := cli.GetStatusPrivacy()
	if err != nil {
		return nil, fmt.Errorf("failed to get status privacy: %w", err)
	}
	statusPrivacy := statusPrivacyOptions[0]
	if statusPrivacy.Type == types.StatusPrivacyTypeWhitelist {
		// Whitelist mode, just return the list
		return statusPrivacy.List, nil
	}

	// Blacklist or all contacts mode. Find all contacts from database, then filter them appropriately.
	contacts, err := cli.Store.Contacts.GetAllContacts()
	if err != nil {
		return nil, fmt.Errorf("failed to get contact list from db: %w", err)
	}

	blacklist := make(map[types.JID]struct{})
	if statusPrivacy.Type == types.StatusPrivacyTypeBlacklist {
		for _, jid := range statusPrivacy.List {
			blacklist[jid] = struct{}{}
		}
	}

	var contactsArray []types.JID
	for jid, contact := range contacts {
		_, isBlacklisted := blacklist[jid]
		if isBlacklisted {
			continue
		}
		// TODO should there be a better way to separate contacts and found push names in the db?
		if len(contact.FullName) > 0 {
			contactsArray = append(contactsArray, jid)
		}
	}
	return contactsArray, nil
}

var DefaultStatusPrivacy = []types.StatusPrivacy{{
	Type:      types.StatusPrivacyTypeContacts,
	IsDefault: true,
}}

// GetStatusPrivacy gets the user's status privacy settings (who to send status broadcasts to).
//
// There can be multiple different stored settings, the first one is always the default.
func (cli *Client) GetStatusPrivacy() ([]types.StatusPrivacy, error) {
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "status",
		Type:      iqGet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "privacy",
		}},
	})
	if err != nil {
		if errors.Is(err, ErrIQNotFound) {
			return DefaultStatusPrivacy, nil
		}
		return nil, err
	}
	privacyLists := resp.GetChildByTag("privacy")
	var outputs []types.StatusPrivacy
	for _, list := range privacyLists.GetChildren() {
		if list.Tag != "list" {
			continue
		}

		ag := list.AttrGetter()
		var out types.StatusPrivacy
		out.IsDefault = ag.OptionalBool("default")
		out.Type = types.StatusPrivacyType(ag.String("type"))
		children := list.GetChildren()
		if len(children) > 0 {
			out.List = make([]types.JID, 0, len(children))
			for _, child := range children {
				jid, ok := child.Attrs["jid"].(types.JID)
				if child.Tag == "user" && ok {
					out.List = append(out.List, jid)
				}
			}
		}
		outputs = append(outputs, out)
		if out.IsDefault {
			// Move default to always be first in the list
			outputs[len(outputs)-1] = outputs[0]
			outputs[0] = out
		}
		if len(ag.Errors) > 0 {
			return nil, ag.Error()
		}
	}
	if len(outputs) == 0 {
		return DefaultStatusPrivacy, nil
	}
	return outputs, nil
}
