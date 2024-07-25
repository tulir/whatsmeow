// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package appstate implements encoding and decoding WhatsApp's app state patches.
package appstate

import (
	"encoding/base64"
	"sync"

	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/util/hkdfutil"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// WAPatchName represents a type of app state patch.
type WAPatchName string

const (
	// WAPatchCriticalBlock contains the user's settings like push name and locale.
	WAPatchCriticalBlock WAPatchName = "critical_block"
	// WAPatchCriticalUnblockLow contains the user's contact list.
	WAPatchCriticalUnblockLow WAPatchName = "critical_unblock_low"
	// WAPatchRegularLow contains some local chat settings like pin, archive status, and the setting of whether to unarchive chats when messages come in.
	WAPatchRegularLow WAPatchName = "regular_low"
	// WAPatchRegularHigh contains more local chat settings like mute status and starred messages.
	WAPatchRegularHigh WAPatchName = "regular_high"
	// WAPatchRegular contains protocol info about app state patches like key expiration.
	WAPatchRegular WAPatchName = "regular"
)

// AllPatchNames contains all currently known patch state names.
var AllPatchNames = [...]WAPatchName{WAPatchCriticalBlock, WAPatchCriticalUnblockLow, WAPatchRegularHigh, WAPatchRegular, WAPatchRegularLow}

// Constants for the first part of app state indexes.
const (
	IndexMute                    = "mute"
	IndexPin                     = "pin_v1"
	IndexArchive                 = "archive"
	IndexContact                 = "contact"
	IndexClearChat               = "clearChat"
	IndexDeleteChat              = "deleteChat"
	IndexStar                    = "star"
	IndexDeleteMessageForMe      = "deleteMessageForMe"
	IndexMarkChatAsRead          = "markChatAsRead"
	IndexSettingPushName         = "setting_pushName"
	IndexSettingUnarchiveChats   = "setting_unarchiveChats"
	IndexUserStatusMute          = "userStatusMute"
	IndexLabelEdit               = "label_edit"
	IndexLabelAssociationChat    = "label_jid"
	IndexLabelAssociationMessage = "label_message"
)

type Processor struct {
	keyCache     map[string]ExpandedAppStateKeys
	keyCacheLock sync.Mutex
	Store        *store.Device
	Log          waLog.Logger
}

func NewProcessor(store *store.Device, log waLog.Logger) *Processor {
	return &Processor{
		keyCache: make(map[string]ExpandedAppStateKeys),
		Store:    store,
		Log:      log,
	}
}

type ExpandedAppStateKeys struct {
	Index           []byte
	ValueEncryption []byte
	ValueMAC        []byte
	SnapshotMAC     []byte
	PatchMAC        []byte
}

func expandAppStateKeys(keyData []byte) (keys ExpandedAppStateKeys) {
	appStateKeyExpanded := hkdfutil.SHA256(keyData, nil, []byte("WhatsApp Mutation Keys"), 160)
	return ExpandedAppStateKeys{appStateKeyExpanded[0:32], appStateKeyExpanded[32:64], appStateKeyExpanded[64:96], appStateKeyExpanded[96:128], appStateKeyExpanded[128:160]}
}

func (proc *Processor) getAppStateKey(keyID []byte) (keys ExpandedAppStateKeys, err error) {
	keyCacheID := base64.RawStdEncoding.EncodeToString(keyID)
	var ok bool

	proc.keyCacheLock.Lock()
	defer proc.keyCacheLock.Unlock()

	keys, ok = proc.keyCache[keyCacheID]
	if !ok {
		var keyData *store.AppStateSyncKey
		keyData, err = proc.Store.AppStateKeys.GetAppStateSyncKey(keyID)
		if keyData != nil {
			keys = expandAppStateKeys(keyData.Data)
			proc.keyCache[keyCacheID] = keys
		} else if err == nil {
			err = ErrKeyNotFound
		}
	}
	return
}

func (proc *Processor) GetMissingKeyIDs(pl *PatchList) [][]byte {
	cache := make(map[string]bool)
	var missingKeys [][]byte
	checkMissing := func(keyID []byte) {
		if keyID == nil {
			return
		}
		stringKeyID := base64.RawStdEncoding.EncodeToString(keyID)
		_, alreadyAdded := cache[stringKeyID]
		if !alreadyAdded {
			keyData, err := proc.Store.AppStateKeys.GetAppStateSyncKey(keyID)
			if err != nil {
				proc.Log.Warnf("Error fetching key %X while checking if it's missing: %v", keyID, err)
			}
			missing := keyData == nil && err == nil
			cache[stringKeyID] = missing
			if missing {
				missingKeys = append(missingKeys, keyID)
			}
		}
	}
	if pl.Snapshot != nil {
		checkMissing(pl.Snapshot.GetKeyId().GetId())
		for _, record := range pl.Snapshot.GetRecords() {
			checkMissing(record.GetKeyId().GetId())
		}
	}
	for _, patch := range pl.Patches {
		checkMissing(patch.GetKeyId().GetId())
	}
	return missingKeys
}
