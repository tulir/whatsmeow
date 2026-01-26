// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package appstate implements encoding and decoding WhatsApp's app state patches.
package appstate

import (
	"context"
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

// Constants for the regular_low app state indexes.
const (
	IndexPin                       = "pin_v1"
	IndexRecentEmojiWeightsAction  = "recent_emoji_weights_action"
	IndexArchive                   = "archive"
	IndexSentinel                  = "sentinel"
	IndexMarkChatAsRead            = "markChatAsRead"
	IndexSettingUnarchiveChats     = "setting_unarchiveChats"
	IndexAndroidUnsupportedActions = "android_unsupported_actions"
	IndexTimeFormat                = "time_format"
	IndexNux                       = "nux"
	IndexPrimaryVersion            = "primary_version"
	IndexFavoriteSticker           = "favoriteSticker"
	IndexRemoveRecentSticker       = "removeRecentSticker"
	IndexBotWelcomeRequest         = "bot_welcome_request"
	IndexPaymentInfo               = "payment_info"
	IndexCustomPaymentMethods      = "custom_payment_methods"
	IndexLock                      = "lock"
	IndexSettingChatLock           = "setting_chatLock"
	IndexDeviceCapabilities        = "device_capabilities"
	IndexNoteEdit                  = "note_edit"
	IndexMerchantPaymentPartner    = "merchant_payment_partner"
	IndexPaymentTOS                = "payment_tos"
	IndexAIThreadRename            = "ai_thread_rename"
	IndexInteractiveMessageAction  = "interactive_message_action"
	IndexSettingsSync              = "settings_sync"
)

// Constants for the regular app state indexes.
const (
	IndexQuickReply                                      = "quick_reply"
	IndexLabelAssociationMessage                         = "label_message"
	IndexLabelEdit                                       = "label_edit"
	IndexLabelAssociationChat                            = "label_jid"
	IndexPrimaryFeature                                  = "primary_feature"
	IndexDeviceAgent                                     = "deviceAgent"
	IndexSubscription                                    = "subscription"
	IndexAgentChatAssignment                             = "agentChatAssignment"
	IndexAgentChatAssignmentOpenedStatus                 = "agentChatAssignmentOpenedStatus"
	IndexPNForLIDChat                                    = "pnForLidChat"
	IndexMarketingMessage                                = "marketingMessage"
	IndexMarketingMessageBroadcast                       = "marketingMessageBroadcast"
	IndexExternalWebBeta                                 = "external_web_beta"
	IndexSettingRrelayAllCalls                           = "setting_relayAllCalls"
	IndexCallLog                                         = "call_log"
	IndexDeleteIndividualCallLog                         = "delete_individual_call_log"
	IndexLabelReordering                                 = "label_reordering"
	IndexSettingDisableLinkPreviews                      = "setting_disableLinkPreviews"
	IndexUsernameChatStartMode                           = "usernameChatStartMode"
	IndexNotificationActivitySetting                     = "notificationActivitySetting"
	IndexSettingChannelsPersonalisedRecommendationOptout = "setting_channels_personalised_recommendation_optout"
	IndexBroadcastJID                                    = "broadcast_jid"
	IndexDetectedOutcomesStatusAction                    = "detected_outcomes_status_action"
	IndexBusinessBroadcastList                           = "business_broadcast_list"
	IndexMusicUserID                                     = "music_user_id"
	IndexAvatarUpdatedAction                             = "avatar_updated_action"
	IndexGalaxyFlowAction                                = "galaxy_flow_action"
	IndexNewsletterSavedInterests                        = "newsletter_saved_interests"
	IndexShareOwnPN                                      = "shareOwnPn"
	IndexBroadcast                                       = "broadcast"
)

// Constants for the regular_high app state indexes.
const (
	IndexStar                                         = "star"
	IndexMute                                         = "mute"
	IndexDeleteMessageForMe                           = "deleteMessageForMe"
	IndexClearChat                                    = "clearChat"
	IndexDeleteChat                                   = "deleteChat"
	IndexUserStatusMute                               = "userStatusMute"
	IndexUGCBot                                       = "ugc_bot"
	IndexStatusPrivacy                                = "status_privacy"
	IndexFavorites                                    = "favorites"
	IndexWaffleAccountLinkState                       = "waffle_account_link_state"
	IndexCTWAPerCustomerDataSharing                   = "ctwaPerCustomerDataSharing"
	IndexMaibaAIFeaturesControl                       = "maiba_ai_features_control"
	IndexStatusPostOptInNotificationPreferencesAction = "status_post_opt_in_notification_preferences_action"
	IndexPrivateProcessingSetting                     = "private_processing_setting"
	IndexAIThreadDelete                               = "ai_thread_delete"
)

// Constants for the critical_unblock_low app state indexes.
const (
	IndexContact    = "contact"
	IndexLIDContact = "lid_contact"
)

// Constants for the critical_block app state indexes.
const (
	IndexSettingSecurityNotification = "setting_securityNotification"
	IndexSettingPushName             = "setting_pushName"
	IndexSettingLocale               = "setting_locale"
	IndexGeneratedWUI                = "generated_wui"
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

func (proc *Processor) getAppStateKey(ctx context.Context, keyID []byte) (keys ExpandedAppStateKeys, err error) {
	keyCacheID := base64.RawStdEncoding.EncodeToString(keyID)
	var ok bool

	proc.keyCacheLock.Lock()
	defer proc.keyCacheLock.Unlock()

	keys, ok = proc.keyCache[keyCacheID]
	if !ok {
		var keyData *store.AppStateSyncKey
		keyData, err = proc.Store.AppStateKeys.GetAppStateSyncKey(ctx, keyID)
		if keyData != nil {
			keys = expandAppStateKeys(keyData.Data)
			proc.keyCache[keyCacheID] = keys
		} else if err == nil {
			err = ErrKeyNotFound
		}
	}
	return
}

func (proc *Processor) GetMissingKeyIDs(ctx context.Context, pl *PatchList) [][]byte {
	cache := make(map[string]bool)
	var missingKeys [][]byte
	checkMissing := func(keyID []byte) {
		if keyID == nil {
			return
		}
		stringKeyID := base64.RawStdEncoding.EncodeToString(keyID)
		_, alreadyAdded := cache[stringKeyID]
		if !alreadyAdded {
			keyData, err := proc.Store.AppStateKeys.GetAppStateSyncKey(ctx, keyID)
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
		checkMissing(pl.Snapshot.GetKeyID().GetID())
		for _, record := range pl.Snapshot.GetRecords() {
			checkMissing(record.GetKeyID().GetID())
		}
	}
	for _, patch := range pl.Patches {
		checkMissing(patch.GetKeyID().GetID())
	}
	return missingKeys
}
