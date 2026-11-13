// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"errors"
	"fmt"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func (cli *Client) getBroadcastListParticipants(ctx context.Context, jid types.JID) ([]types.JID, error) {
	var list []types.JID
	var err error
	if jid == types.StatusBroadcastJID {
		list, err = cli.getStatusBroadcastRecipients(ctx)
	} else {
		return nil, ErrBroadcastListUnsupported
	}
	if err != nil {
		return nil, err
	}
	ownID := cli.getOwnLID().ToNonAD()
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
	if selfIndex < 0 {
		list = append(list, ownID)
	}
	return list, nil
}

func (cli *Client) getStatusBroadcastRecipients(ctx context.Context) ([]types.JID, error) {
	statusPrivacyOptions, err := cli.GetStatusPrivacy(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get status privacy: %w", err)
	}
	statusPrivacy := statusPrivacyOptions[0]

	var candidates []types.JID
	if statusPrivacy.Type == types.StatusPrivacyTypeWhitelist {
		candidates = statusPrivacy.List
	} else {
		// Blacklist or all contacts mode. Find all contacts from database, then filter them appropriately.
		contacts, err := cli.Store.Contacts.GetAllContacts(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get contact list from db: %w", err)
		}
		for jid, contact := range contacts {
			// TODO should there be a better way to separate contacts and found push names in the db?
			if len(contact.FullName) > 0 {
				candidates = append(candidates, jid)
			}
		}
	}

	// Collect all PNs from candidates and blacklist to query their LIDs in one batch
	var pnList []types.JID
	for _, jid := range candidates {
		if jid.Server == types.DefaultUserServer {
			pnList = append(pnList, jid.ToNonAD())
		}
	}
	if statusPrivacy.Type == types.StatusPrivacyTypeBlacklist {
		for _, jid := range statusPrivacy.List {
			if jid.Server == types.DefaultUserServer {
				pnList = append(pnList, jid.ToNonAD())
			}
		}
	}

	lidMap, err := cli.Store.LIDs.GetManyLIDsForPNs(ctx, pnList)
	if err != nil {
		return nil, fmt.Errorf("failed to query LID mappings: %w", err)
	}

	// Populate blacklist matching both PN and LID
	blacklist := make(map[types.JID]struct{})
	if statusPrivacy.Type == types.StatusPrivacyTypeBlacklist {
		for _, jid := range statusPrivacy.List {
			baseJID := jid.ToNonAD()
			blacklist[baseJID] = struct{}{}
			if baseJID.Server == types.DefaultUserServer {
				if lid, ok := lidMap[baseJID]; ok && !lid.IsEmpty() {
					blacklist[lid.ToNonAD()] = struct{}{}
				}
			} else if baseJID.Server == types.HiddenUserServer {
				if pn, err := cli.Store.LIDs.GetPNForLID(ctx, baseJID); err == nil && !pn.IsEmpty() {
					blacklist[pn.ToNonAD()] = struct{}{}
				}
			}
		}
	}

	recipients := make([]types.JID, 0, len(candidates))
	for _, jid := range candidates {
		baseJID := jid.ToNonAD()
		target := baseJID
		if baseJID.Server == types.DefaultUserServer {
			target = lidMap[baseJID].ToNonAD()
		}
		if target.IsEmpty() || target.Server != types.HiddenUserServer {
			continue
		}
		if _, isBlacklisted := blacklist[baseJID]; isBlacklisted {
			continue
		}
		if _, isBlacklisted := blacklist[target]; isBlacklisted {
			continue
		}
		recipients = append(recipients, target)
	}
	return recipients, nil
}

var DefaultStatusPrivacy = []types.StatusPrivacy{{
	Type:      types.StatusPrivacyTypeContacts,
	IsDefault: true,
}}

// GetStatusPrivacy gets the user's status privacy settings (who to send status broadcasts to).
//
// There can be multiple different stored settings, the first one is always the default.
func (cli *Client) GetStatusPrivacy(ctx context.Context) ([]types.StatusPrivacy, error) {
	resp, err := cli.sendIQ(ctx, infoQuery{
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
