// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"sync/atomic"
	"time"

	"go.mau.fi/libsignal/groups"
	"go.mau.fi/libsignal/protocol"
	"go.mau.fi/libsignal/session"
	"go.mau.fi/libsignal/signalerror"
	"go.mau.fi/util/random"
	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var pbSerializer = store.SignalProtobufSerializer

func (cli *Client) handleEncryptedMessage(node *waBinary.Node) {
	info, err := cli.parseMessageInfo(node)
	if err != nil {
		cli.Log.Warnf("Failed to parse message: %v", err)
	} else {
		if info.VerifiedName != nil && len(info.VerifiedName.Details.GetVerifiedName()) > 0 {
			go cli.updateBusinessName(info.Sender, info, info.VerifiedName.Details.GetVerifiedName())
		}
		if len(info.PushName) > 0 && info.PushName != "-" {
			go cli.updatePushName(info.Sender, info, info.PushName)
		}
		cli.decryptMessages(info, node)
	}
}

func (cli *Client) parseMessageSource(node *waBinary.Node, requireParticipant bool) (source types.MessageSource, err error) {
	clientID := cli.getOwnID()
	if clientID.IsEmpty() {
		err = ErrNotLoggedIn
		return
	}
	ag := node.AttrGetter()
	from := ag.JID("from")
	if from.Server == types.GroupServer || from.Server == types.BroadcastServer {
		source.IsGroup = true
		source.Chat = from
		if requireParticipant {
			source.Sender = ag.JID("participant")
		} else {
			source.Sender = ag.OptionalJIDOrEmpty("participant")
		}
		if source.Sender.User == clientID.User {
			source.IsFromMe = true
		}
		if from.Server == types.BroadcastServer {
			source.BroadcastListOwner = ag.OptionalJIDOrEmpty("recipient")
		}
	} else if from.User == clientID.User {
		source.IsFromMe = true
		source.Sender = from
		recipient := ag.OptionalJID("recipient")
		if recipient != nil {
			source.Chat = *recipient
		} else {
			source.Chat = from.ToNonAD()
		}
	} else {
		source.Chat = from.ToNonAD()
		source.Sender = from
	}
	err = ag.Error()
	return
}

func (cli *Client) parseMessageInfo(node *waBinary.Node) (*types.MessageInfo, error) {
	var info types.MessageInfo
	var err error
	info.MessageSource, err = cli.parseMessageSource(node, true)
	if err != nil {
		return nil, err
	}
	ag := node.AttrGetter()
	info.ID = types.MessageID(ag.String("id"))
	info.Timestamp = ag.UnixTime("t")
	info.PushName = ag.OptionalString("notify")
	info.Category = ag.OptionalString("category")
	info.Type = ag.OptionalString("type")
	if !ag.OK() {
		return nil, ag.Error()
	}

	for _, child := range node.GetChildren() {
		if child.Tag == "multicast" {
			info.Multicast = true
		} else if child.Tag == "verified_name" {
			info.VerifiedName, err = parseVerifiedNameContent(child)
			if err != nil {
				cli.Log.Warnf("Failed to parse verified_name node in %s: %v", info.ID, err)
			}
		} else if mediaType, ok := child.AttrGetter().GetString("mediatype", false); ok {
			info.MediaType = mediaType
		}
	}

	return &info, nil
}

func (cli *Client) decryptMessages(info *types.MessageInfo, node *waBinary.Node) {
	go cli.sendAck(node)
	if len(node.GetChildrenByTag("unavailable")) > 0 && len(node.GetChildrenByTag("enc")) == 0 {
		cli.Log.Warnf("Unavailable message %s from %s", info.ID, info.SourceString())
		go cli.sendRetryReceipt(node, info, true)
		cli.dispatchEvent(&events.UndecryptableMessage{Info: *info, IsUnavailable: true})
		return
	}
	children := node.GetChildren()
	cli.Log.Debugf("Decrypting %d messages from %s", len(children), info.SourceString())
	handled := false
	containsDirectMsg := false
	for _, child := range children {
		if child.Tag != "enc" {
			continue
		}
		ag := child.AttrGetter()
		encType, ok := ag.GetString("type", false)
		if !ok {
			continue
		}
		var decrypted []byte
		var err error
		if encType == "pkmsg" || encType == "msg" {
			decrypted, err = cli.decryptDM(&child, info.Sender, encType == "pkmsg")
			containsDirectMsg = true
		} else if info.IsGroup && encType == "skmsg" {
			decrypted, err = cli.decryptGroupMsg(&child, info.Sender, info.Chat)
		} else {
			cli.Log.Warnf("Unhandled encrypted message (type %s) from %s", encType, info.SourceString())
			continue
		}
		if err != nil {
			cli.Log.Warnf("Error decrypting message from %s: %v", info.SourceString(), err)
			isUnavailable := encType == "skmsg" && !containsDirectMsg && errors.Is(err, signalerror.ErrNoSenderKeyForUser)
			go cli.sendRetryReceipt(node, info, isUnavailable)
			decryptFailMode, _ := child.Attrs["decrypt-fail"].(string)
			cli.dispatchEvent(&events.UndecryptableMessage{
				Info:            *info,
				IsUnavailable:   isUnavailable,
				DecryptFailMode: events.DecryptFailMode(decryptFailMode),
			})
			return
		}

		var msg waProto.Message
		err = proto.Unmarshal(decrypted, &msg)
		if err != nil {
			cli.Log.Warnf("Error unmarshaling decrypted message from %s: %v", info.SourceString(), err)
			continue
		}
		retryCount := ag.OptionalInt("count")
		if retryCount > 0 {
			cli.cancelDelayedRequestFromPhone(info.ID)
		}

		cli.handleDecryptedMessage(info, &msg, retryCount)
		handled = true
	}
	if handled {
		go cli.sendMessageReceipt(info)
	}
}

func (cli *Client) clearUntrustedIdentity(target types.JID) {
	err := cli.Store.Identities.DeleteIdentity(target.SignalAddress().String())
	if err != nil {
		cli.Log.Warnf("Failed to delete untrusted identity of %s from store: %v", target, err)
	}
	err = cli.Store.Sessions.DeleteSession(target.SignalAddress().String())
	if err != nil {
		cli.Log.Warnf("Failed to delete session with %s (untrusted identity) from store: %v", target, err)
	}
	cli.dispatchEvent(&events.IdentityChange{JID: target, Timestamp: time.Now(), Implicit: true})
}

func (cli *Client) decryptDM(child *waBinary.Node, from types.JID, isPreKey bool) ([]byte, error) {
	content, _ := child.Content.([]byte)

	builder := session.NewBuilderFromSignal(cli.Store, from.SignalAddress(), pbSerializer)
	cipher := session.NewCipher(builder, from.SignalAddress())
	var plaintext []byte
	if isPreKey {
		preKeyMsg, err := protocol.NewPreKeySignalMessageFromBytes(content, pbSerializer.PreKeySignalMessage, pbSerializer.SignalMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to parse prekey message: %w", err)
		}
		plaintext, _, err = cipher.DecryptMessageReturnKey(preKeyMsg)
		if cli.AutoTrustIdentity && errors.Is(err, signalerror.ErrUntrustedIdentity) {
			cli.Log.Warnf("Got %v error while trying to decrypt prekey message from %s, clearing stored identity and retrying", err, from)
			cli.clearUntrustedIdentity(from)
			plaintext, _, err = cipher.DecryptMessageReturnKey(preKeyMsg)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt prekey message: %w", err)
		}
	} else {
		msg, err := protocol.NewSignalMessageFromBytes(content, pbSerializer.SignalMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to parse normal message: %w", err)
		}
		plaintext, err = cipher.Decrypt(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt normal message: %w", err)
		}
	}
	return unpadMessage(plaintext)
}

func (cli *Client) decryptGroupMsg(child *waBinary.Node, from types.JID, chat types.JID) ([]byte, error) {
	content, _ := child.Content.([]byte)

	senderKeyName := protocol.NewSenderKeyName(chat.String(), from.SignalAddress())
	builder := groups.NewGroupSessionBuilder(cli.Store, pbSerializer)
	cipher := groups.NewGroupCipher(builder, senderKeyName, cli.Store)
	msg, err := protocol.NewSenderKeyMessageFromBytes(content, pbSerializer.SenderKeyMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to parse group message: %w", err)
	}
	plaintext, err := cipher.Decrypt(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt group message: %w", err)
	}
	return unpadMessage(plaintext)
}

const checkPadding = true

func isValidPadding(plaintext []byte) bool {
	lastByte := plaintext[len(plaintext)-1]
	expectedPadding := bytes.Repeat([]byte{lastByte}, int(lastByte))
	return bytes.HasSuffix(plaintext, expectedPadding)
}

func unpadMessage(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("plaintext is empty")
	}
	if checkPadding && !isValidPadding(plaintext) {
		return nil, fmt.Errorf("plaintext doesn't have expected padding")
	}
	return plaintext[:len(plaintext)-int(plaintext[len(plaintext)-1])], nil
}

func padMessage(plaintext []byte) []byte {
	pad := random.Bytes(1)
	pad[0] &= 0xf
	if pad[0] == 0 {
		pad[0] = 0xf
	}
	plaintext = append(plaintext, bytes.Repeat(pad[:], int(pad[0]))...)
	return plaintext
}

func (cli *Client) handleSenderKeyDistributionMessage(chat, from types.JID, rawSKDMsg *waProto.SenderKeyDistributionMessage) {
	builder := groups.NewGroupSessionBuilder(cli.Store, pbSerializer)
	senderKeyName := protocol.NewSenderKeyName(chat.String(), from.SignalAddress())
	sdkMsg, err := protocol.NewSenderKeyDistributionMessageFromBytes(rawSKDMsg.AxolotlSenderKeyDistributionMessage, pbSerializer.SenderKeyDistributionMessage)
	if err != nil {
		cli.Log.Errorf("Failed to parse sender key distribution message from %s for %s: %v", from, chat, err)
		return
	}
	builder.Process(senderKeyName, sdkMsg)
	cli.Log.Debugf("Processed sender key distribution message from %s in %s", senderKeyName.Sender().String(), senderKeyName.GroupID())
}

func (cli *Client) handleHistorySyncNotificationLoop() {
	defer func() {
		atomic.StoreUint32(&cli.historySyncHandlerStarted, 0)
		err := recover()
		if err != nil {
			cli.Log.Errorf("History sync handler panicked: %v\n%s", err, debug.Stack())
		}

		// Check in case something new appeared in the channel between the loop stopping
		// and the atomic variable being updated. If yes, restart the loop.
		if len(cli.historySyncNotifications) > 0 && atomic.CompareAndSwapUint32(&cli.historySyncHandlerStarted, 0, 1) {
			cli.Log.Warnf("New history sync notifications appeared after loop stopped, restarting loop...")
			go cli.handleHistorySyncNotificationLoop()
		}
	}()
	for notif := range cli.historySyncNotifications {
		cli.handleHistorySyncNotification(notif)
	}
}

func (cli *Client) handleHistorySyncNotification(notif *waProto.HistorySyncNotification) {
	var historySync waProto.HistorySync
	if data, err := cli.Download(notif); err != nil {
		cli.Log.Errorf("Failed to download history sync data: %v", err)
	} else if reader, err := zlib.NewReader(bytes.NewReader(data)); err != nil {
		cli.Log.Errorf("Failed to create zlib reader for history sync data: %v", err)
	} else if rawData, err := io.ReadAll(reader); err != nil {
		cli.Log.Errorf("Failed to decompress history sync data: %v", err)
	} else if err = proto.Unmarshal(rawData, &historySync); err != nil {
		cli.Log.Errorf("Failed to unmarshal history sync data: %v", err)
	} else {
		cli.Log.Debugf("Received history sync (type %s, chunk %d)", historySync.GetSyncType(), historySync.GetChunkOrder())
		if historySync.GetSyncType() == waProto.HistorySync_PUSH_NAME {
			go cli.handleHistoricalPushNames(historySync.GetPushnames())
		} else if len(historySync.GetConversations()) > 0 {
			go cli.storeHistoricalMessageSecrets(historySync.GetConversations())
		}
		cli.dispatchEvent(&events.HistorySync{
			Data: &historySync,
		})
	}
}

func (cli *Client) handleAppStateSyncKeyShare(keys *waProto.AppStateSyncKeyShare) {
	onlyResyncIfNotSynced := true

	cli.Log.Debugf("Got %d new app state keys", len(keys.GetKeys()))
	cli.appStateKeyRequestsLock.RLock()
	for _, key := range keys.GetKeys() {
		marshaledFingerprint, err := proto.Marshal(key.GetKeyData().GetFingerprint())
		if err != nil {
			cli.Log.Errorf("Failed to marshal fingerprint of app state sync key %X", key.GetKeyId().GetKeyId())
			continue
		}
		_, isReRequest := cli.appStateKeyRequests[hex.EncodeToString(key.GetKeyId().GetKeyId())]
		if isReRequest {
			onlyResyncIfNotSynced = false
		}
		err = cli.Store.AppStateKeys.PutAppStateSyncKey(key.GetKeyId().GetKeyId(), store.AppStateSyncKey{
			Data:        key.GetKeyData().GetKeyData(),
			Fingerprint: marshaledFingerprint,
			Timestamp:   key.GetKeyData().GetTimestamp(),
		})
		if err != nil {
			cli.Log.Errorf("Failed to store app state sync key %X: %v", key.GetKeyId().GetKeyId(), err)
			continue
		}
		cli.Log.Debugf("Received app state sync key %X (ts: %d)", key.GetKeyId().GetKeyId(), key.GetKeyData().GetTimestamp())
	}
	cli.appStateKeyRequestsLock.RUnlock()

	for _, name := range appstate.AllPatchNames {
		err := cli.FetchAppState(name, false, onlyResyncIfNotSynced)
		if err != nil {
			cli.Log.Errorf("Failed to do initial fetch of app state %s: %v", name, err)
		}
	}
}

func (cli *Client) handlePlaceholderResendResponse(msg *waProto.PeerDataOperationRequestResponseMessage) {
	reqID := msg.GetStanzaId()
	parts := msg.GetPeerDataOperationResult()
	cli.Log.Debugf("Handling response to placeholder resend request %s with %d items", reqID, len(parts))
	for i, part := range parts {
		var webMsg waProto.WebMessageInfo
		if resp := part.GetPlaceholderMessageResendResponse(); resp == nil {
			cli.Log.Warnf("Missing response in item #%d of response to %s", i+1, reqID)
		} else if err := proto.Unmarshal(resp.GetWebMessageInfoBytes(), &webMsg); err != nil {
			cli.Log.Warnf("Failed to unmarshal protobuf web message in item #%d of response to %s: %v", i+1, reqID, err)
		} else if msgEvt, err := cli.ParseWebMessage(types.EmptyJID, &webMsg); err != nil {
			cli.Log.Warnf("Failed to parse web message info in item #%d of response to %s: %v", i+1, reqID, err)
		} else {
			msgEvt.UnavailableRequestID = reqID
			cli.dispatchEvent(msgEvt)
		}
	}
}

func (cli *Client) handleProtocolMessage(info *types.MessageInfo, msg *waProto.Message) {
	protoMsg := msg.GetProtocolMessage()

	if protoMsg.GetHistorySyncNotification() != nil && info.IsFromMe {
		cli.historySyncNotifications <- protoMsg.HistorySyncNotification
		if atomic.CompareAndSwapUint32(&cli.historySyncHandlerStarted, 0, 1) {
			go cli.handleHistorySyncNotificationLoop()
		}
		go cli.sendProtocolMessageReceipt(info.ID, "hist_sync")
	}

	if protoMsg.GetPeerDataOperationRequestResponseMessage().GetPeerDataOperationRequestType() == waProto.PeerDataOperationRequestType_PLACEHOLDER_MESSAGE_RESEND {
		go cli.handlePlaceholderResendResponse(protoMsg.GetPeerDataOperationRequestResponseMessage())
	}

	if protoMsg.GetAppStateSyncKeyShare() != nil && info.IsFromMe {
		go cli.handleAppStateSyncKeyShare(protoMsg.AppStateSyncKeyShare)
	}

	if info.Category == "peer" {
		go cli.sendProtocolMessageReceipt(info.ID, "peer_msg")
	}
}

func (cli *Client) processProtocolParts(info *types.MessageInfo, msg *waProto.Message) {
	// Hopefully sender key distribution messages and protocol messages can't be inside ephemeral messages
	if msg.GetDeviceSentMessage().GetMessage() != nil {
		msg = msg.GetDeviceSentMessage().GetMessage()
	}
	if msg.GetSenderKeyDistributionMessage() != nil {
		if !info.IsGroup {
			cli.Log.Warnf("Got sender key distribution message in non-group chat from", info.Sender)
		} else {
			cli.handleSenderKeyDistributionMessage(info.Chat, info.Sender, msg.SenderKeyDistributionMessage)
		}
	}
	// N.B. Edits are protocol messages, but they're also wrapped inside EditedMessage,
	// which is only unwrapped after processProtocolParts, so this won't trigger for edits.
	if msg.GetProtocolMessage() != nil {
		cli.handleProtocolMessage(info, msg)
	}
	if msgSecret := msg.GetMessageContextInfo().GetMessageSecret(); len(msgSecret) > 0 {
		err := cli.Store.MsgSecrets.PutMessageSecret(info.Chat, info.Sender, info.ID, msgSecret)
		if err != nil {
			cli.Log.Errorf("Failed to store message secret key for %s: %v", info.ID, err)
		} else {
			cli.Log.Debugf("Stored message secret key for %s", info.ID)
		}
	}
}

func (cli *Client) storeHistoricalMessageSecrets(conversations []*waProto.Conversation) {
	var secrets []store.MessageSecretInsert
	var privacyTokens []store.PrivacyToken
	ownID := cli.getOwnID().ToNonAD()
	if ownID.IsEmpty() {
		return
	}
	for _, conv := range conversations {
		chatJID, _ := types.ParseJID(conv.GetId())
		if chatJID.IsEmpty() {
			continue
		}
		if chatJID.Server == types.DefaultUserServer && conv.GetTcToken() != nil {
			ts := conv.GetTcTokenSenderTimestamp()
			if ts == 0 {
				ts = conv.GetTcTokenTimestamp()
			}
			privacyTokens = append(privacyTokens, store.PrivacyToken{
				User:      chatJID,
				Token:     conv.GetTcToken(),
				Timestamp: time.Unix(int64(ts), 0),
			})
		}
		for _, msg := range conv.GetMessages() {
			if secret := msg.GetMessage().GetMessageSecret(); secret != nil {
				var senderJID types.JID
				msgKey := msg.GetMessage().GetKey()
				if msgKey.GetFromMe() {
					senderJID = ownID
				} else if chatJID.Server == types.DefaultUserServer {
					senderJID = chatJID
				} else if msgKey.GetParticipant() != "" {
					senderJID, _ = types.ParseJID(msgKey.GetParticipant())
				} else if msg.GetMessage().GetParticipant() != "" {
					senderJID, _ = types.ParseJID(msg.GetMessage().GetParticipant())
				}
				if senderJID.IsEmpty() || msgKey.GetId() == "" {
					continue
				}
				secrets = append(secrets, store.MessageSecretInsert{
					Chat:   chatJID,
					Sender: senderJID,
					ID:     msgKey.GetId(),
					Secret: secret,
				})
			}
		}
	}
	if len(secrets) > 0 {
		cli.Log.Debugf("Storing %d message secret keys in history sync", len(secrets))
		err := cli.Store.MsgSecrets.PutMessageSecrets(secrets)
		if err != nil {
			cli.Log.Errorf("Failed to store message secret keys in history sync: %v", err)
		} else {
			cli.Log.Infof("Stored %d message secret keys from history sync", len(secrets))
		}
	}
	if len(privacyTokens) > 0 {
		cli.Log.Debugf("Storing %d privacy tokens in history sync", len(privacyTokens))
		err := cli.Store.PrivacyTokens.PutPrivacyTokens(privacyTokens...)
		if err != nil {
			cli.Log.Errorf("Failed to store privacy tokens in history sync: %v", err)
		} else {
			cli.Log.Infof("Stored %d privacy tokens from history sync", len(privacyTokens))
		}
	}
}

func (cli *Client) handleDecryptedMessage(info *types.MessageInfo, msg *waProto.Message, retryCount int) {
	cli.processProtocolParts(info, msg)
	evt := &events.Message{Info: *info, RawMessage: msg, RetryCount: retryCount}
	cli.dispatchEvent(evt.UnwrapRaw())
}

func (cli *Client) sendProtocolMessageReceipt(id, msgType string) {
	clientID := cli.Store.ID
	if len(id) == 0 || clientID == nil {
		return
	}
	err := cli.sendNode(waBinary.Node{
		Tag: "receipt",
		Attrs: waBinary.Attrs{
			"id":   id,
			"type": msgType,
			"to":   types.NewJID(clientID.User, types.LegacyUserServer),
		},
		Content: nil,
	})
	if err != nil {
		cli.Log.Warnf("Failed to send acknowledgement for protocol message %s: %v", id, err)
	}
}
