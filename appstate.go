// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/util/exslices"
	"go.mau.fi/util/ptr"

	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waServerSync"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// FetchAppState fetches updates to the given type of app state. If fullSync is true, the current
// cached state will be removed and all app state patches will be re-fetched from the server.
func (cli *Client) FetchAppState(ctx context.Context, name appstate.WAPatchName, fullSync, onlyIfNotSynced bool) error {
	eventsToDispatch, err := cli.fetchAppState(ctx, name, fullSync, onlyIfNotSynced)
	if err != nil {
		return err
	}
	for _, evt := range eventsToDispatch {
		cli.dispatchEvent(evt)
	}
	return nil
}

func (cli *Client) fetchAppState(ctx context.Context, name appstate.WAPatchName, fullSync, onlyIfNotSynced bool) ([]any, error) {
	if cli == nil {
		return nil, ErrClientIsNil
	}
	cli.appStateSyncLock.Lock()
	defer cli.appStateSyncLock.Unlock()
	if fullSync {
		err := cli.Store.AppState.DeleteAppStateVersion(ctx, string(name))
		if err != nil {
			return nil, fmt.Errorf("failed to reset app state %s version: %w", name, err)
		}
	}
	version, hash, err := cli.Store.AppState.GetAppStateVersion(ctx, string(name))
	if err != nil {
		return nil, fmt.Errorf("failed to get app state %s version: %w", name, err)
	}
	if version == 0 {
		fullSync = true
	} else if onlyIfNotSynced {
		return nil, nil
	}

	state := appstate.HashState{Version: version, Hash: hash}

	hasMore := true
	wantSnapshot := fullSync
	var eventsToDispatch []any
	eventsToDispatchPtr := &eventsToDispatch
	if fullSync && !cli.EmitAppStateEventsOnFullSync {
		eventsToDispatchPtr = nil
	}
	for hasMore {
		patches, err := cli.fetchAppStatePatches(ctx, name, state.Version, wantSnapshot)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch app state %s patches: %w", name, err)
		} else if !wantSnapshot && patches.Snapshot != nil {
			return nil, fmt.Errorf("server unexpectedly returned snapshot for %s without asking", name)
		} else if patches.Snapshot != nil && state != (appstate.HashState{}) {
			return nil, fmt.Errorf("unexpected non-empty input state (v%d) for %s when applying snapshot", state.Version, name)
		}
		wantSnapshot = false
		hasMore = patches.HasMorePatches
		state, err = cli.applyAppStatePatches(ctx, name, state, patches, fullSync, eventsToDispatchPtr)
		if err != nil {
			cli.dispatchEvent(&events.AppStateSyncError{Name: name, FullSync: fullSync, Error: err})
			return nil, err
		}
	}
	if fullSync {
		cli.Log.Debugf("Full sync of app state %s completed. Current version: %d", name, state.Version)
		eventsToDispatch = append(eventsToDispatch, &events.AppStateSyncComplete{Name: name, Version: state.Version})
	} else {
		cli.Log.Debugf("Synced app state %s from version %d to %d", name, version, state.Version)
	}
	return eventsToDispatch, nil
}

func (cli *Client) handleAppStateRecovery(
	ctx context.Context,
	reqID types.MessageID,
	result []*waE2E.PeerDataOperationRequestResponseMessage_PeerDataOperationResult,
) bool {
	if len(result) == 0 || result[0].GetSyncdSnapshotFatalRecoveryResponse() == nil {
		cli.Log.Warnf("No app state recovery data received for %s", reqID)
		return true
	} else if len(result) > 1 {
		cli.Log.Warnf("Unexpected number of app state recovery results for %s: %d", reqID, len(result))
	}
	var eventsToDispatch []any
	eventsToDispatchPtr := &eventsToDispatch
	if !cli.EmitAppStateEventsOnFullSync {
		eventsToDispatchPtr = nil
	}
	snapshot, err := appstate.ParseRecovery(result[0].GetSyncdSnapshotFatalRecoveryResponse())
	if err != nil {
		cli.Log.Warnf("Failed to parse app state recovery blob for %s: %v", reqID, err)
		return true
	}
	name := appstate.WAPatchName(snapshot.GetCollectionName())
	version := snapshot.GetVersion().GetVersion()
	currentVersion, _, err := cli.Store.AppState.GetAppStateVersion(ctx, string(name))
	if err != nil {
		cli.Log.Errorf("Failed to get current app state %s version for %s: %v", name, reqID, err)
		return true
	} else if currentVersion >= version {
		cli.Log.Infof("Ignoring app state recovery response for %s as current version %d is newer than or equal to recovery version %d", reqID, currentVersion, snapshot.GetVersion().GetVersion())
		return true
	}
	cli.Log.Debugf("Handling app state recovery response for %s", reqID)
	mutations, err := cli.appStateProc.ProcessRecovery(ctx, snapshot)
	if err != nil {
		cli.Log.Warnf("Failed to parse app state recovery blob for %s: %v", reqID, err)
		return true
	}
	err = cli.collectEventsToDispatch(ctx, name, mutations, true, eventsToDispatchPtr)
	if err != nil {
		cli.Log.Warnf("Failed to collect app state events for %s: %v", reqID, err)
		return true
	}
	eventsToDispatch = append(eventsToDispatch, &events.AppStateSyncComplete{Name: name, Version: version, Recovery: true})
	for _, evt := range eventsToDispatch {
		handlerFailed := cli.dispatchEvent(evt)
		if handlerFailed {
			return false
		}
	}
	cli.Log.Debugf("Finished handling app state recovery response for %s (%s to v%d)", reqID, name, version)
	return true
}

func (cli *Client) applyAppStatePatches(
	ctx context.Context,
	name appstate.WAPatchName,
	state appstate.HashState,
	patches *appstate.PatchList,
	fullSync bool,
	eventsToDispatch *[]any,
) (appstate.HashState, error) {
	mutations, newState, err := cli.appStateProc.DecodePatches(ctx, patches, state, true)
	if err != nil {
		if errors.Is(err, appstate.ErrKeyNotFound) {
			go cli.requestMissingAppStateKeys(context.WithoutCancel(ctx), patches)
		}
		return state, fmt.Errorf("failed to decode app state %s patches: %w", name, err)
	}
	return newState, cli.collectEventsToDispatch(ctx, name, mutations, fullSync, eventsToDispatch)
}

func (cli *Client) collectEventsToDispatch(
	ctx context.Context,
	name appstate.WAPatchName,
	mutations []appstate.Mutation,
	fullSync bool,
	eventsToDispatch *[]any,
) error {
	if name == appstate.WAPatchCriticalUnblockLow && fullSync && !cli.EmitAppStateEventsOnFullSync {
		var contacts []store.ContactEntry
		mutations, contacts = cli.filterContacts(mutations)
		cli.Log.Debugf("Mass inserting app state snapshot with %d contacts into the store", len(contacts))
		err := cli.Store.Contacts.PutAllContactNames(ctx, contacts)
		if err != nil {
			// This is a fairly serious failure, so just abort the whole thing
			return fmt.Errorf("failed to update contact store with data from snapshot: %v", err)
		}
	}
	for _, mutation := range mutations {
		if eventsToDispatch != nil && mutation.Operation == waServerSync.SyncdMutation_SET {
			*eventsToDispatch = append(*eventsToDispatch, &events.AppState{Index: mutation.Index, SyncActionValue: mutation.Action})
		}
		evt := cli.dispatchAppState(ctx, name, mutation, fullSync)
		if eventsToDispatch != nil && evt != nil {
			*eventsToDispatch = append(*eventsToDispatch, evt)
		}
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

func (cli *Client) dispatchAppState(ctx context.Context, name appstate.WAPatchName, mutation appstate.Mutation, fullSync bool) (eventToDispatch any) {
	logLevel := zerolog.TraceLevel
	log := zerolog.Ctx(ctx)
	if cli.AppStateDebugLogs && log.GetLevel() != zerolog.TraceLevel {
		logLevel = zerolog.DebugLevel
	}
	logEvt := log.WithLevel(logLevel).
		Str("patch_name", string(name)).
		Uint64("patch_version", mutation.PatchVersion).
		Stringer("operation", mutation.Operation).
		Int32("version", mutation.Version).
		Strs("index", mutation.Index).
		Hex("index_mac", mutation.IndexMAC).
		Hex("value_mac", mutation.ValueMAC)
	if logLevel == zerolog.TraceLevel {
		logEvt.Any("action", mutation.Action)
	}
	logEvt.Msg("Received app state mutation")

	if mutation.Operation != waServerSync.SyncdMutation_SET {
		return
	}

	var jid types.JID
	if len(mutation.Index) > 1 {
		jid, _ = types.ParseJID(mutation.Index[1])
	}
	ts := time.UnixMilli(mutation.Action.GetTimestamp())

	var storeUpdateError error
	switch mutation.Index[0] {
	case appstate.IndexMute:
		act := mutation.Action.GetMuteAction()
		eventToDispatch = &events.Mute{JID: jid, Timestamp: ts, Action: act, FromFullSync: fullSync}
		var mutedUntil time.Time
		if act.GetMuted() {
			if act.GetMuteEndTimestamp() < 0 {
				mutedUntil = store.MutedForever
			} else {
				mutedUntil = time.UnixMilli(act.GetMuteEndTimestamp())
			}
		}
		if cli.Store.ChatSettings != nil {
			storeUpdateError = cli.Store.ChatSettings.PutMutedUntil(ctx, jid, mutedUntil)
		}
	case appstate.IndexPin:
		act := mutation.Action.GetPinAction()
		eventToDispatch = &events.Pin{JID: jid, Timestamp: ts, Action: act, FromFullSync: fullSync}
		if cli.Store.ChatSettings != nil {
			storeUpdateError = cli.Store.ChatSettings.PutPinned(ctx, jid, act.GetPinned())
		}
	case appstate.IndexArchive:
		act := mutation.Action.GetArchiveChatAction()
		eventToDispatch = &events.Archive{JID: jid, Timestamp: ts, Action: act, FromFullSync: fullSync}
		if cli.Store.ChatSettings != nil {
			storeUpdateError = cli.Store.ChatSettings.PutArchived(ctx, jid, act.GetArchived())
		}
	case appstate.IndexContact:
		act := mutation.Action.GetContactAction()
		eventToDispatch = &events.Contact{JID: jid, Timestamp: ts, Action: act, FromFullSync: fullSync}
		if cli.Store.Contacts != nil {
			storeUpdateError = cli.Store.Contacts.PutContactName(ctx, jid, act.GetFirstName(), act.GetFullName())
		}
	case appstate.IndexClearChat:
		act := mutation.Action.GetClearChatAction()
		var deleteMedia bool
		// TODO what's index 2 here?
		if len(mutation.Index) > 3 && mutation.Index[3] == "1" {
			deleteMedia = true
		}
		eventToDispatch = &events.ClearChat{
			JID:          jid,
			Timestamp:    ts,
			Action:       act,
			DeleteMedia:  deleteMedia,
			FromFullSync: fullSync,
		}
	case appstate.IndexDeleteChat:
		act := mutation.Action.GetDeleteChatAction()
		var deleteMedia bool
		if len(mutation.Index) > 2 && mutation.Index[2] == "1" {
			deleteMedia = true
		}
		eventToDispatch = &events.DeleteChat{
			JID:          jid,
			Timestamp:    ts,
			Action:       act,
			DeleteMedia:  deleteMedia,
			FromFullSync: fullSync,
		}
	case appstate.IndexStar:
		if len(mutation.Index) < 5 {
			return
		}
		evt := events.Star{
			ChatJID:      jid,
			MessageID:    mutation.Index[2],
			Timestamp:    ts,
			Action:       mutation.Action.GetStarAction(),
			IsFromMe:     mutation.Index[3] == "1",
			FromFullSync: fullSync,
		}
		if mutation.Index[4] != "0" {
			evt.SenderJID, _ = types.ParseJID(mutation.Index[4])
		}
		eventToDispatch = &evt
	case appstate.IndexDeleteMessageForMe:
		if len(mutation.Index) < 5 {
			return
		}
		evt := events.DeleteForMe{
			ChatJID:      jid,
			MessageID:    mutation.Index[2],
			Timestamp:    ts,
			Action:       mutation.Action.GetDeleteMessageForMeAction(),
			IsFromMe:     mutation.Index[3] == "1",
			FromFullSync: fullSync,
		}
		if mutation.Index[4] != "0" {
			evt.SenderJID, _ = types.ParseJID(mutation.Index[4])
		}
		eventToDispatch = &evt
	case appstate.IndexMarkChatAsRead:
		eventToDispatch = &events.MarkChatAsRead{
			JID:          jid,
			Timestamp:    ts,
			Action:       mutation.Action.GetMarkChatAsReadAction(),
			FromFullSync: fullSync,
		}
	case appstate.IndexSettingPushName:
		eventToDispatch = &events.PushNameSetting{
			Timestamp:    ts,
			Action:       mutation.Action.GetPushNameSetting(),
			FromFullSync: fullSync,
		}
		cli.Store.PushName = mutation.Action.GetPushNameSetting().GetName()
		err := cli.Store.Save(ctx)
		if err != nil {
			cli.Log.Errorf("Failed to save device store after updating push name: %v", err)
		}
	case appstate.IndexSettingUnarchiveChats:
		eventToDispatch = &events.UnarchiveChatsSetting{
			Timestamp:    ts,
			Action:       mutation.Action.GetUnarchiveChatsSetting(),
			FromFullSync: fullSync,
		}
	case appstate.IndexUserStatusMute:
		eventToDispatch = &events.UserStatusMute{
			JID:          jid,
			Timestamp:    ts,
			Action:       mutation.Action.GetUserStatusMuteAction(),
			FromFullSync: fullSync,
		}
	case appstate.IndexLabelEdit:
		act := mutation.Action.GetLabelEditAction()
		eventToDispatch = &events.LabelEdit{
			Timestamp:    ts,
			LabelID:      mutation.Index[1],
			Action:       act,
			FromFullSync: fullSync,
		}
	case appstate.IndexLabelAssociationChat:
		if len(mutation.Index) < 3 {
			return
		}
		jid, _ = types.ParseJID(mutation.Index[2])
		act := mutation.Action.GetLabelAssociationAction()
		eventToDispatch = &events.LabelAssociationChat{
			JID:          jid,
			Timestamp:    ts,
			LabelID:      mutation.Index[1],
			Action:       act,
			FromFullSync: fullSync,
		}
	case appstate.IndexLabelAssociationMessage:
		if len(mutation.Index) < 6 {
			return
		}
		jid, _ = types.ParseJID(mutation.Index[2])
		act := mutation.Action.GetLabelAssociationAction()
		eventToDispatch = &events.LabelAssociationMessage{
			JID:          jid,
			Timestamp:    ts,
			LabelID:      mutation.Index[1],
			MessageID:    mutation.Index[3],
			Action:       act,
			FromFullSync: fullSync,
		}
	}
	if storeUpdateError != nil {
		cli.Log.Errorf("Failed to update device store after app state mutation: %v", storeUpdateError)
	}
	return
}

func (cli *Client) downloadExternalAppStateBlob(ctx context.Context, ref *waServerSync.ExternalBlobReference) ([]byte, error) {
	return cli.Download(ctx, ref)
}

func (cli *Client) fetchAppStatePatches(ctx context.Context, name appstate.WAPatchName, fromVersion uint64, snapshot bool) (*appstate.PatchList, error) {
	attrs := waBinary.Attrs{
		"name":            string(name),
		"return_snapshot": snapshot,
	}
	if !snapshot {
		attrs["version"] = fromVersion
	}
	resp, err := cli.sendIQ(ctx, infoQuery{
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
	collection, ok := resp.GetOptionalChildByTag("sync", "collection")
	if !ok {
		return nil, &ElementMissingError{Tag: "collection", In: "app state patch response"}
	}
	return appstate.ParsePatchList(ctx, &collection, cli.downloadExternalAppStateBlob)
}

func (cli *Client) requestMissingAppStateKeys(ctx context.Context, patches *appstate.PatchList) {
	cli.appStateKeyRequestsLock.Lock()
	rawKeyIDs := cli.appStateProc.GetMissingKeyIDs(ctx, patches)
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
	cli.requestAppStateKeys(ctx, filteredKeyIDs)
}

func (cli *Client) requestAppStateKeys(ctx context.Context, rawKeyIDs [][]byte) {
	keyIDs := make([]*waE2E.AppStateSyncKeyId, len(rawKeyIDs))
	debugKeyIDs := make([]string, len(rawKeyIDs))
	for i, keyID := range rawKeyIDs {
		keyIDs[i] = &waE2E.AppStateSyncKeyId{KeyID: keyID}
		debugKeyIDs[i] = hex.EncodeToString(keyID)
	}
	msg := &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Type: waE2E.ProtocolMessage_APP_STATE_SYNC_KEY_REQUEST.Enum(),
			AppStateSyncKeyRequest: &waE2E.AppStateSyncKeyRequest{
				KeyIDs: keyIDs,
			},
		},
	}
	if len(debugKeyIDs) == 0 {
		return
	}
	cli.Log.Infof("Sending key request for app state keys %+v", debugKeyIDs)
	_, err := cli.SendPeerMessage(ctx, msg)
	if err != nil {
		cli.Log.Warnf("Failed to send app state key request: %v", err)
	}
}

// SendAppState sends the given app state patch, then triggers a background resync of that app state type
// to update local caches and send events for the updates.
//
// You can use the Build methods in the appstate package to build the parameter for this method, e.g.
//
//	cli.SendAppState(ctx, appstate.BuildMute(targetJID, true, 24 * time.Hour))
func (cli *Client) SendAppState(ctx context.Context, patch appstate.PatchInfo) error {
	return cli.sendAppState(ctx, patch, true)
}

func (cli *Client) sendAppState(ctx context.Context, patch appstate.PatchInfo, allowRetry bool) error {
	if cli == nil {
		return ErrClientIsNil
	}
	version, hash, err := cli.Store.AppState.GetAppStateVersion(ctx, string(patch.Type))
	if err != nil {
		return err
	}
	// TODO create new key instead of reusing the primary client's keys
	latestKeyID, err := cli.Store.AppStateKeys.GetLatestAppStateSyncKeyID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest app state key ID: %w", err)
	} else if latestKeyID == nil {
		return fmt.Errorf("no app state keys found, creating app state keys is not yet supported")
	}

	state := appstate.HashState{Version: version, Hash: hash}

	encodedPatch, err := cli.appStateProc.EncodePatch(ctx, latestKeyID, state, patch)
	if err != nil {
		return err
	}

	resp, err := cli.sendIQ(ctx, infoQuery{
		Namespace: "w:sync:app:state",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "sync",
			Content: []waBinary.Node{{
				Tag: "collection",
				Attrs: waBinary.Attrs{
					"name":            string(patch.Type),
					"version":         version,
					"return_snapshot": false,
				},
				Content: []waBinary.Node{{
					Tag:     "patch",
					Content: encodedPatch,
				}},
			}},
		}},
	})
	if err != nil {
		return err
	}

	respCollection, ok := resp.GetOptionalChildByTag("sync", "collection")
	if !ok {
		return &ElementMissingError{Tag: "collection", In: "app state send response"}
	}
	respCollectionAttr := respCollection.AttrGetter()
	if respCollectionAttr.OptionalString("type") == "error" {
		errorTag, ok := respCollection.GetOptionalChildByTag("error")

		mainErr := fmt.Errorf("%w: %s", ErrAppStateUpdate, respCollection.XMLString())
		if ok {
			mainErr = fmt.Errorf("%w (%s): %s", ErrAppStateUpdate, patch.Type, errorTag.XMLString())
		}
		if ok && errorTag.AttrGetter().Int("code") == 409 && allowRetry {
			zerolog.Ctx(ctx).Warn().Err(mainErr).Msg("Failed to update app state, trying to apply conflicts and retry")
			var eventsToDispatch []any
			patches, err := appstate.ParsePatchList(ctx, &respCollection, cli.downloadExternalAppStateBlob)
			if err != nil {
				return fmt.Errorf("%w (also, parsing patches in the response failed: %w)", mainErr, err)
			} else if state, err = cli.applyAppStatePatches(ctx, patch.Type, state, patches, false, &eventsToDispatch); err != nil {
				return fmt.Errorf("%w (also, applying patches in the response failed: %w)", mainErr, err)
			} else {
				zerolog.Ctx(ctx).Debug().Msg("Retrying app state send after applying conflicting patches")
				go func() {
					for _, evt := range eventsToDispatch {
						cli.dispatchEvent(evt)
					}
				}()
				return cli.sendAppState(ctx, patch, false)
			}
		}
		return mainErr
	}
	eventsToDispatch, err := cli.fetchAppState(ctx, patch.Type, false, false)
	if err != nil {
		return fmt.Errorf("failed to fetch app state after sending update: %w", err)
	}
	go func() {
		for _, evt := range eventsToDispatch {
			cli.dispatchEvent(evt)
		}
	}()

	return nil
}

func (cli *Client) MarkNotDirty(ctx context.Context, cleanType string, ts time.Time) error {
	_, err := cli.sendIQ(ctx, infoQuery{
		Namespace: "urn:xmpp:whatsapp:dirty",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "clean",
			Attrs: waBinary.Attrs{
				"type":      cleanType,
				"timestamp": ts.Unix(),
			},
		}},
	})
	return err
}

// BuildFatalAppStateExceptionNotification builds a message to request the user's primary device
// to reset specific app state collections. This will cause all linked devices to be logged out.
//
// The built message can be sent using Client.SendPeerMessage.
// There is no response, as the client will get logged out.
func BuildFatalAppStateExceptionNotification(collections ...appstate.WAPatchName) *waE2E.Message {
	return &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Type: waE2E.ProtocolMessage_APP_STATE_FATAL_EXCEPTION_NOTIFICATION.Enum(),
			AppStateFatalExceptionNotification: &waE2E.AppStateFatalExceptionNotification{
				CollectionNames: exslices.CastToString[string](collections),
				Timestamp:       ptr.Ptr(time.Now().UnixMilli()),
			},
		},
	}
}

// BuildAppStateRecoveryRequest builds a message to request the user's primary device to send
// an unencrypted copy of the given app state collection.
//
// The built message can be sent using Client.SendPeerMessage.
// The response will come as a ProtocolMessage with type `PEER_DATA_OPERATION_RESPONSE_MESSAGE`.
func BuildAppStateRecoveryRequest(collection appstate.WAPatchName) *waE2E.Message {
	return &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Type: waE2E.ProtocolMessage_PEER_DATA_OPERATION_REQUEST_MESSAGE.Enum(),
			PeerDataOperationRequestMessage: &waE2E.PeerDataOperationRequestMessage{
				PeerDataOperationRequestType: waE2E.PeerDataOperationRequestType_COMPANION_SYNCD_SNAPSHOT_FATAL_RECOVERY.Enum(),
				SyncdCollectionFatalRecoveryRequest: &waE2E.PeerDataOperationRequestMessage_SyncDCollectionFatalRecoveryRequest{
					CollectionName: (*string)(&collection),
					Timestamp:      ptr.Ptr(time.Now().Unix()),
				},
			},
		},
	}
}
