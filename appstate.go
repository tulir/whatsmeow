// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"fmt"
	"time"

	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/events"
)

// EmitAppStateEventsOnFullSync can be set to true if you want to get app state events emitted even when re-syncing the whole state.
var EmitAppStateEventsOnFullSync = false

func (cli *Client) FetchAppState(name appstate.WAPatchName, fullSync bool) error {
	if fullSync {
		err := cli.Store.AppState.DeleteAppStateVersion(string(name))
		if err != nil {
			return fmt.Errorf("failed to reset app state %s version: %w", name, err)
		}
	}
	version, hash, err := cli.Store.AppState.GetAppStateVersion(string(name))
	if err != nil {
		return fmt.Errorf("failed to get app state %s version: %w", name, err)
	}
	if version == 0 {
		fullSync = true
	}
	state := appstate.HashState{Version: version, Hash: hash}
	hasMore := true
	for hasMore {
		patches, err := cli.fetchAppStatePatches(name, state.Version)
		if err != nil {
			return fmt.Errorf("failed to fetch app state %s patches: %w", name, err)
		}
		hasMore = patches.HasMorePatches

		mutations, newState, err := cli.appStateProc.DecodePatches(patches, state, true)
		if err != nil {
			return fmt.Errorf("failed to decode app state %s patches: %w", name, err)
		}
		state = newState
		fmt.Printf("%d %X\n", newState.Version, newState.Hash)
		for _, mutation := range mutations {
			fmt.Printf("%s %v %X %+v\n", mutation.Operation, mutation.Index, mutation.IndexMAC, mutation.Action)
			if (!fullSync || EmitAppStateEventsOnFullSync) && mutation.Operation == waProto.SyncdMutation_SET {
				cli.dispatchAppState(mutation)
			}
		}
	}
	return nil
}

func (cli *Client) dispatchAppState(mutation appstate.Mutation) {
	cli.dispatchEvent(&events.AppState{mutation.Index, mutation.Action})
	var jid waBinary.JID
	if len(mutation.Index) > 1 {
		jid, _ = waBinary.ParseJID(mutation.Index[1])
	}
	ts := time.Unix(mutation.Action.GetTimestamp(), 0)
	switch mutation.Index[0] {
	case "mute":
		cli.dispatchEvent(&events.Mute{jid, ts, mutation.Action.GetMuteAction()})
	case "pin_v1":
		cli.dispatchEvent(&events.Pin{jid, ts, mutation.Action.GetPinAction()})
	case "archive":
		cli.dispatchEvent(&events.Archive{jid, ts, mutation.Action.GetArchiveChatAction()})
	case "contact":
		cli.dispatchEvent(&events.Contact{jid, ts, mutation.Action.GetContactAction()})
	case "star":
		evt := events.Star{
			ChatJID:   jid,
			MessageID: mutation.Index[2],
			Timestamp: ts,
			Action:    mutation.Action.GetStarAction(),
			IsFromMe:  mutation.Index[3] == "1",
		}
		if mutation.Index[4] != "0" {
			evt.SenderJID, _ = waBinary.ParseJID(mutation.Index[4])
		}
		cli.dispatchEvent(&evt)
	case "deleteMessageForMe":
		evt := events.DeleteForMe{
			ChatJID:   jid,
			MessageID: mutation.Index[2],
			Timestamp: ts,
			Action:    mutation.Action.GetDeleteMessageForMeAction(),
			IsFromMe:  mutation.Index[3] == "1",
		}
		if mutation.Index[4] != "0" {
			evt.SenderJID, _ = waBinary.ParseJID(mutation.Index[4])
		}
		cli.dispatchEvent(&evt)
	case "setting_pushName":
		cli.dispatchEvent(&events.PushName{ts, mutation.Action.GetPushNameSetting()})
	}
}

func (cli *Client) fetchAppStatePatches(name appstate.WAPatchName, fromVersion uint64) (*appstate.PatchList, error) {
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:sync:app:state",
		Type:      "set",
		To:        waBinary.ServerJID,
		Content: []waBinary.Node{{
			Tag: "sync",
			Content: []waBinary.Node{{
				Tag: "collection",
				Attrs: waBinary.Attrs{
					"name":            string(name),
					"version":         fromVersion,
					"return_snapshot": false,
				},
			}},
		}},
	})
	if err != nil {
		return nil, err
	}
	return appstate.ParsePatchList(resp)
}
