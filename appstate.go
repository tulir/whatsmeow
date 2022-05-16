// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// FetchAppState fetches updates to the given type of app state. If fullSync is true, the current
// cached state will be removed and all app state patches will be re-fetched from the server.
func (cli *Client) FetchAppState(name appstate.WAPatchName, fullSync, onlyIfNotSynced bool) error {
	cli.appStateSyncLock.Lock()
	defer cli.appStateSyncLock.Unlock()
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
	} else if onlyIfNotSynced {
		return nil
	}

	state := appstate.HashState{Version: version, Hash: hash}

	hasMore := true
	wantSnapshot := fullSync
	for hasMore {
		patches, err := cli.fetchAppStatePatches(name, state.Version, wantSnapshot)
		wantSnapshot = false
		if err != nil {
			return fmt.Errorf("failed to fetch app state %s patches: %w", name, err)
		}
		hasMore = patches.HasMorePatches

		mutations, newState, err := cli.appStateProc.DecodePatches(patches, state, true)
		if err != nil {
			if errors.Is(err, appstate.ErrKeyNotFound) {
				go cli.requestMissingAppStateKeys(patches)
			}
			return fmt.Errorf("failed to decode app state %s patches: %w", name, err)
		}
		wasFullSync := state.Version == 0 && patches.Snapshot != nil
		state = newState
		if name == appstate.WAPatchCriticalUnblockLow && wasFullSync && !cli.EmitAppStateEventsOnFullSync {
			var contacts []store.ContactEntry
			mutations, contacts = cli.filterContacts(mutations)
			cli.Log.Debugf("Mass inserting app state snapshot with %d contacts into the store", len(contacts))
			err = cli.Store.Contacts.PutAllContactNames(contacts)
			if err != nil {
				// This is a fairly serious failure, so just abort the whole thing
				return fmt.Errorf("failed to update contact store with data from snapshot: %v", err)
			}
		}
		for _, mutation := range mutations {
			cli.dispatchAppState(mutation, !fullSync || cli.EmitAppStateEventsOnFullSync)
		}
	}
	if fullSync {
		cli.Log.Debugf("Full sync of app state %s completed. Current version: %d", name, state.Version)
		cli.dispatchEvent(&events.AppStateSyncComplete{Name: name})
	} else {
		cli.Log.Debugf("Synced app state %s from version %d to %d", name, version, state.Version)
	}
	return nil
}

func (cli *Client) filterContacts(mutations []appstate.Mutation) ([]appstate.Mutation, []store.ContactEntry) {
	filteredMutations := mutations[:0]
	contacts := make([]store.ContactEntry, 0, len(mutations))
	for _, mutation := range mutations {
		if mutation.Index[0] == "contact" && len(mutation.Index) > 1 {
			jid, _ := types.ParseJID(mutation.Index[1])
			act := mutation.Action.GetContactAction()
			contacts = append(contacts, store.ContactEntry{
				JID:       jid,
				FirstName: act.GetFirstName(),
				FullName:  act.GetFullName(),
			})
		} else {
			filteredMutations = append(filteredMutations, mutation)
		}
	}
	return filteredMutations, contacts
}

func (cli *Client) dispatchAppState(mutation appstate.Mutation, dispatchEvts bool) {
	if mutation.Operation != waProto.SyncdMutation_SET {
		return
	}

	if dispatchEvts {
		cli.dispatchEvent(&events.AppState{Index: mutation.Index, SyncActionValue: mutation.Action})
	}

	var jid types.JID
	if len(mutation.Index) > 1 {
		jid, _ = types.ParseJID(mutation.Index[1])
	}
	ts := time.Unix(mutation.Action.GetTimestamp(), 0)

	var storeUpdateError error
	var eventToDispatch interface{}
	switch mutation.Index[0] {
	case "mute":
		act := mutation.Action.GetMuteAction()
		eventToDispatch = &events.Mute{JID: jid, Timestamp: ts, Action: act}
		var mutedUntil time.Time
		if act.GetMuted() {
			mutedUntil = time.Unix(act.GetMuteEndTimestamp(), 0)
		}
		if cli.Store.ChatSettings != nil {
			storeUpdateError = cli.Store.ChatSettings.PutMutedUntil(jid, mutedUntil)
		}
	case "pin_v1":
		act := mutation.Action.GetPinAction()
		eventToDispatch = &events.Pin{JID: jid, Timestamp: ts, Action: act}
		if cli.Store.ChatSettings != nil {
			storeUpdateError = cli.Store.ChatSettings.PutPinned(jid, act.GetPinned())
		}
	case "archive":
		act := mutation.Action.GetArchiveChatAction()
		eventToDispatch = &events.Archive{JID: jid, Timestamp: ts, Action: act}
		if cli.Store.ChatSettings != nil {
			storeUpdateError = cli.Store.ChatSettings.PutArchived(jid, act.GetArchived())
		}
	case "contact":
		act := mutation.Action.GetContactAction()
		eventToDispatch = &events.Contact{JID: jid, Timestamp: ts, Action: act}
		if cli.Store.Contacts != nil {
			storeUpdateError = cli.Store.Contacts.PutContactName(jid, act.GetFirstName(), act.GetFullName())
		}
	case "star":
		if len(mutation.Index) < 5 {
			return
		}
		evt := events.Star{
			ChatJID:   jid,
			MessageID: mutation.Index[2],
			Timestamp: ts,
			Action:    mutation.Action.GetStarAction(),
			IsFromMe:  mutation.Index[3] == "1",
		}
		if mutation.Index[4] != "0" {
			evt.SenderJID, _ = types.ParseJID(mutation.Index[4])
		}
		eventToDispatch = &evt
	case "deleteMessageForMe":
		if len(mutation.Index) < 5 {
			return
		}
		evt := events.DeleteForMe{
			ChatJID:   jid,
			MessageID: mutation.Index[2],
			Timestamp: ts,
			Action:    mutation.Action.GetDeleteMessageForMeAction(),
			IsFromMe:  mutation.Index[3] == "1",
		}
		if mutation.Index[4] != "0" {
			evt.SenderJID, _ = types.ParseJID(mutation.Index[4])
		}
		eventToDispatch = &evt
	case "markChatAsRead":
		eventToDispatch = &events.MarkChatAsRead{
			JID:       jid,
			Timestamp: ts,
			Action:    mutation.Action.GetMarkChatAsReadAction(),
		}
	case "setting_pushName":
		eventToDispatch = &events.PushNameSetting{Timestamp: ts, Action: mutation.Action.GetPushNameSetting()}
		cli.Store.PushName = mutation.Action.GetPushNameSetting().GetName()
		err := cli.Store.Save()
		if err != nil {
			cli.Log.Errorf("Failed to save device store after updating push name: %v", err)
		}
	case "setting_unarchiveChats":
		eventToDispatch = &events.UnarchiveChatsSetting{Timestamp: ts, Action: mutation.Action.GetUnarchiveChatsSetting()}
	}
	if storeUpdateError != nil {
		cli.Log.Errorf("Failed to update device store after app state mutation: %v", storeUpdateError)
	}
	if dispatchEvts && eventToDispatch != nil {
		cli.dispatchEvent(eventToDispatch)
	}
}

func (cli *Client) downloadExternalAppStateBlob(ref *waProto.ExternalBlobReference) ([]byte, error) {
	return cli.Download(ref)
}

func (cli *Client) fetchAppStatePatches(name appstate.WAPatchName, fromVersion uint64, snapshot bool) (*appstate.PatchList, error) {
	attrs := waBinary.Attrs{
		"name":            string(name),
		"return_snapshot": snapshot,
	}
	if !snapshot {
		attrs["version"] = fromVersion
	}
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:sync:app:state",
		Type:      "set",
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "sync",
			Content: []waBinary.Node{{
				Tag:   "collection",
				Attrs: attrs,
			}},
		}},
	})
	if err != nil {
		return nil, err
	}
	return appstate.ParsePatchList(resp, cli.downloadExternalAppStateBlob)
}

func (cli *Client) requestMissingAppStateKeys(patches *appstate.PatchList) {
	cli.appStateKeyRequestsLock.Lock()
	rawKeyIDs := cli.appStateProc.GetMissingKeyIDs(patches)
	filteredKeyIDs := make([][]byte, 0, len(rawKeyIDs))
	now := time.Now()
	for _, keyID := range rawKeyIDs {
		stringKeyID := hex.EncodeToString(keyID)
		lastRequestTime := cli.appStateKeyRequests[stringKeyID]
		if lastRequestTime.IsZero() || lastRequestTime.Add(24*time.Hour).Before(now) {
			cli.appStateKeyRequests[stringKeyID] = now
			filteredKeyIDs = append(filteredKeyIDs, keyID)
		}
	}
	cli.appStateKeyRequestsLock.Unlock()
	cli.requestAppStateKeys(filteredKeyIDs)
}

func (cli *Client) requestAppStateKeys(rawKeyIDs [][]byte) {
	keyIDs := make([]*waProto.AppStateSyncKeyId, len(rawKeyIDs))
	debugKeyIDs := make([]string, len(rawKeyIDs))
	for i, keyID := range rawKeyIDs {
		keyIDs[i] = &waProto.AppStateSyncKeyId{KeyId: keyID}
		debugKeyIDs[i] = hex.EncodeToString(keyID)
	}
	msg := &waProto.Message{
		ProtocolMessage: &waProto.ProtocolMessage{
			Type: waProto.ProtocolMessage_APP_STATE_SYNC_KEY_REQUEST.Enum(),
			AppStateSyncKeyRequest: &waProto.AppStateSyncKeyRequest{
				KeyIds: keyIDs,
			},
		},
	}
	cli.Log.Infof("Sending key request for app state keys %+v", debugKeyIDs)
	_, err := cli.SendMessage(cli.Store.ID.ToNonAD(), "", msg)
	if err != nil {
		cli.Log.Warnf("Failed to send app state key request: %v", err)
	}
}
