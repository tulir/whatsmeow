// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/libsignal/groups"
	"go.mau.fi/libsignal/protocol"
	"go.mau.fi/libsignal/session"
	"go.mau.fi/libsignal/signalerror"
	"go.mau.fi/util/random"
	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/proto/waLidMigrationSyncPayload"
	"go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var pbSerializer = store.SignalProtobufSerializer

func (cli *Client) handleEncryptedMessage(ctx context.Context, node *waBinary.Node) {
	info, err := cli.parseMessageInfo(node)
	if err != nil {
		cli.Log.Warnf("Failed to parse message: %v", err)
	} else {
		if !info.SenderAlt.IsEmpty() {
			cli.StoreLIDPNMapping(ctx, info.SenderAlt, info.Sender)
		} else if !info.RecipientAlt.IsEmpty() {
			cli.StoreLIDPNMapping(ctx, info.RecipientAlt, info.Chat)
		}
		if info.VerifiedName != nil && len(info.VerifiedName.Details.GetVerifiedName()) > 0 {
			go cli.updateBusinessName(ctx, info.Sender, info.SenderAlt, info, info.VerifiedName.Details.GetVerifiedName())
		}
		if len(info.PushName) > 0 && info.PushName != "-" && (cli.MessengerConfig == nil || info.PushName != "username") {
			go cli.updatePushName(ctx, info.Sender, info.SenderAlt, info, info.PushName)
		}
		if info.Sender.Server == types.NewsletterServer {
			var cancelled bool
			defer cli.maybeDeferredAck(ctx, node)(&cancelled)
			cancelled = cli.handlePlaintextMessage(ctx, info, node)
		} else {
			cli.decryptMessages(ctx, info, node)
		}
	}
}

func (cli *Client) parseMessageSource(node *waBinary.Node, requireParticipant bool) (source types.MessageSource, err error) {
	clientID := cli.getOwnID()
	clientLID := cli.getOwnLID()
	if clientID.IsEmpty() {
		err = ErrNotLoggedIn
		return
	}
	ag := node.AttrGetter()
	from := ag.JID("from")
	source.AddressingMode = types.AddressingMode(ag.OptionalString("addressing_mode"))
	if from.Server == types.GroupServer || from.Server == types.BroadcastServer {
		source.IsGroup = true
		source.Chat = from
		if requireParticipant {
			source.Sender = ag.JID("participant")
		} else {
			source.Sender = ag.OptionalJIDOrEmpty("participant")
		}
		if source.AddressingMode == types.AddressingModeLID {
			source.SenderAlt = ag.OptionalJIDOrEmpty("participant_pn")
		} else {
			source.SenderAlt = ag.OptionalJIDOrEmpty("participant_lid")
		}
		if source.Sender.User == clientID.User || source.Sender.User == clientLID.User {
			source.IsFromMe = true
		}
		if from.Server == types.BroadcastServer {
			source.BroadcastListOwner = ag.OptionalJIDOrEmpty("recipient")
			participants, ok := node.GetOptionalChildByTag("participants")
			if ok && source.IsFromMe {
				children := participants.GetChildren()
				source.BroadcastRecipients = make([]types.BroadcastRecipient, 0, len(children))
				for _, child := range children {
					if child.Tag != "to" {
						continue
					}
					cag := child.AttrGetter()
					mainJID := cag.JID("jid")
					if mainJID.Server == types.HiddenUserServer {
						source.BroadcastRecipients = append(source.BroadcastRecipients, types.BroadcastRecipient{
							LID: mainJID,
							PN:  cag.OptionalJIDOrEmpty("peer_recipient_pn"),
						})
					} else {
						source.BroadcastRecipients = append(source.BroadcastRecipients, types.BroadcastRecipient{
							LID: cag.OptionalJIDOrEmpty("peer_recipient_lid"),
							PN:  mainJID,
						})
					}
				}
			}
		}
	} else if from.Server == types.NewsletterServer {
		source.Chat = from
		source.Sender = from
		// TODO IsFromMe?
	} else if from.User == clientID.User || from.User == clientLID.User {
		if from.Server == types.HostedServer {
			from.Server = types.DefaultUserServer
		} else if from.Server == types.HostedLIDServer {
			from.Server = types.HiddenUserServer
		}
		source.IsFromMe = true
		source.Sender = from
		recipient := ag.OptionalJID("recipient")
		if recipient != nil {
			source.Chat = *recipient
		} else {
			source.Chat = from.ToNonAD()
		}
		if source.Chat.Server == types.HiddenUserServer || source.Chat.Server == types.HostedLIDServer {
			source.RecipientAlt = ag.OptionalJIDOrEmpty("peer_recipient_pn")
		} else {
			source.RecipientAlt = ag.OptionalJIDOrEmpty("peer_recipient_lid")
		}
	} else if from.IsBot() {
		source.Sender = from
		meta := node.GetChildByTag("meta")
		ag = meta.AttrGetter()
		targetChatJID := ag.OptionalJID("target_chat_jid")
		if targetChatJID != nil {
			source.Chat = targetChatJID.ToNonAD()
		} else {
			source.Chat = from
		}
	} else {
		if from.Server == types.HostedServer {
			from.Server = types.DefaultUserServer
		} else if from.Server == types.HostedLIDServer {
			from.Server = types.HiddenUserServer
		}
		source.Chat = from.ToNonAD()
		source.Sender = from
		if source.Sender.Server == types.HiddenUserServer || source.Chat.Server == types.HostedLIDServer {
			source.SenderAlt = ag.OptionalJIDOrEmpty("sender_pn")
		} else {
			source.SenderAlt = ag.OptionalJIDOrEmpty("sender_lid")
		}
	}
	if !source.SenderAlt.IsEmpty() && source.SenderAlt.Device == 0 {
		source.SenderAlt.Device = source.Sender.Device
	}
	err = ag.Error()
	return
}

func (cli *Client) parseMsgBotInfo(node waBinary.Node) (botInfo types.MsgBotInfo, err error) {
	botNode := node.GetChildByTag("bot")

	ag := botNode.AttrGetter()
	botInfo.EditType = types.BotEditType(ag.String("edit"))
	if botInfo.EditType == types.EditTypeInner || botInfo.EditType == types.EditTypeLast {
		botInfo.EditTargetID = types.MessageID(ag.String("edit_target_id"))
		botInfo.EditSenderTimestampMS = ag.UnixMilli("sender_timestamp_ms")
	}
	err = ag.Error()
	return
}

func (cli *Client) parseMsgMetaInfo(node waBinary.Node) (metaInfo types.MsgMetaInfo, err error) {
	metaNode := node.GetChildByTag("meta")

	ag := metaNode.AttrGetter()
	metaInfo.TargetID = types.MessageID(ag.OptionalString("target_id"))
	metaInfo.TargetSender = ag.OptionalJIDOrEmpty("target_sender_jid")
	metaInfo.TargetChat = ag.OptionalJIDOrEmpty("target_chat_jid")
	deprecatedLIDSession, ok := ag.GetBool("deprecated_lid_session", false)
	if ok {
		metaInfo.DeprecatedLIDSession = &deprecatedLIDSession
	}
	metaInfo.ThreadMessageID = types.MessageID(ag.OptionalString("thread_msg_id"))
	metaInfo.ThreadMessageSenderJID = ag.OptionalJIDOrEmpty("thread_msg_sender_jid")
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
	info.ServerID = types.MessageServerID(ag.OptionalInt("server_id"))
	info.Timestamp = ag.UnixTime("t")
	info.PushName = ag.OptionalString("notify")
	info.Category = ag.OptionalString("category")
	info.Type = ag.OptionalString("type")
	info.Edit = types.EditAttribute(ag.OptionalString("edit"))
	if !ag.OK() {
		return nil, ag.Error()
	}

	for _, child := range node.GetChildren() {
		switch child.Tag {
		case "multicast":
			info.Multicast = true
		case "verified_name":
			info.VerifiedName, err = parseVerifiedNameContent(child)
			if err != nil {
				cli.Log.Warnf("Failed to parse verified_name node in %s: %v", info.ID, err)
			}
		case "bot":
			info.MsgBotInfo, err = cli.parseMsgBotInfo(child)
			if err != nil {
				cli.Log.Warnf("Failed to parse <bot> node in %s: %v", info.ID, err)
			}
		case "meta":
			info.MsgMetaInfo, err = cli.parseMsgMetaInfo(child)
			if err != nil {
				cli.Log.Warnf("Failed to parse <meta> node in %s: %v", info.ID, err)
			}
		case "franking":
			// TODO
		case "trace":
			// TODO
		default:
			if mediaType, ok := child.AttrGetter().GetString("mediatype", false); ok {
				info.MediaType = mediaType
			}
		}
	}

	return &info, nil
}

func (cli *Client) handlePlaintextMessage(ctx context.Context, info *types.MessageInfo, node *waBinary.Node) (handlerFailed bool) {
	// TODO edits have an additional <meta msg_edit_t="1696321271735" original_msg_t="1696321248"/> node
	plaintext, ok := node.GetOptionalChildByTag("plaintext")
	if !ok {
		// 3:
		return
	}
	plaintextBody, ok := plaintext.Content.([]byte)
	if !ok {
		cli.Log.Warnf("Plaintext message from %s doesn't have byte content", info.SourceString())
		return
	}

	var msg waE2E.Message
	err := proto.Unmarshal(plaintextBody, &msg)
	if err != nil {
		cli.Log.Warnf("Error unmarshaling plaintext message from %s: %v", info.SourceString(), err)
		return
	}
	cli.storeMessageSecret(ctx, info, &msg)
	evt := &events.Message{
		Info:       *info,
		RawMessage: &msg,
	}
	meta, ok := node.GetOptionalChildByTag("meta")
	if ok {
		evt.NewsletterMeta = &events.NewsletterMessageMeta{
			EditTS:     meta.AttrGetter().UnixMilli("msg_edit_t"),
			OriginalTS: meta.AttrGetter().UnixTime("original_msg_t"),
		}
	}
	return cli.dispatchEvent(evt.UnwrapRaw())
}

func (cli *Client) migrateSessionStore(ctx context.Context, pn, lid types.JID) {
	err := cli.Store.Sessions.MigratePNToLID(ctx, pn, lid)
	if err != nil {
		cli.Log.Errorf("Failed to migrate signal store from %s to %s: %v", pn, lid, err)
	}
}

func (cli *Client) decryptMessages(ctx context.Context, info *types.MessageInfo, node *waBinary.Node) {
	unavailableNode, ok := node.GetOptionalChildByTag("unavailable")
	if ok && len(node.GetChildrenByTag("enc")) == 0 {
		uType := events.UnavailableType(unavailableNode.AttrGetter().String("type"))
		cli.Log.Warnf("Unavailable message %s from %s (type: %q)", info.ID, info.SourceString(), uType)
		cli.backgroundIfAsyncAck(func() {
			cli.immediateRequestMessageFromPhone(ctx, info)
			cli.sendAck(ctx, node, 0)
		})
		cli.dispatchEvent(&events.UndecryptableMessage{Info: *info, IsUnavailable: true, UnavailableType: uType})
		return
	}

	children := node.GetChildren()
	cli.Log.Debugf("Decrypting message from %s", info.SourceString())
	containsDirectMsg := false
	senderEncryptionJID := info.Sender
	if info.Sender.Server == types.DefaultUserServer && !info.Sender.IsBot() {
		if info.SenderAlt.Server == types.HiddenUserServer {
			senderEncryptionJID = info.SenderAlt
			cli.migrateSessionStore(ctx, info.Sender, info.SenderAlt)
		} else if lid, err := cli.Store.LIDs.GetLIDForPN(ctx, info.Sender); err != nil {
			cli.Log.Errorf("Failed to get LID for %s: %v", info.Sender, err)
		} else if !lid.IsEmpty() {
			cli.migrateSessionStore(ctx, info.Sender, lid)
			senderEncryptionJID = lid
			info.SenderAlt = lid
		} else {
			cli.Log.Warnf("No LID found for %s", info.Sender)
		}
	}
	var recognizedStanza, protobufFailed bool
	for _, child := range children {
		if child.Tag != "enc" {
			continue
		}
		recognizedStanza = true
		ag := child.AttrGetter()
		encType, ok := ag.GetString("type", false)
		if !ok {
			continue
		}
		var decrypted []byte
		var ciphertextHash *[32]byte
		var err error
		if encType == "pkmsg" || encType == "msg" {
			decrypted, ciphertextHash, err = cli.decryptDM(ctx, &child, senderEncryptionJID, encType == "pkmsg", info.Timestamp)
			containsDirectMsg = true
		} else if info.IsGroup && encType == "skmsg" {
			decrypted, ciphertextHash, err = cli.decryptGroupMsg(ctx, &child, senderEncryptionJID, info.Chat, info.Timestamp)
		} else if encType == "msmsg" && info.Sender.IsBot() {
			targetSenderJID := info.MsgMetaInfo.TargetSender
			if targetSenderJID.User == "" {
				if info.Sender.Server == types.BotServer {
					targetSenderJID = cli.getOwnLID()
				} else {
					targetSenderJID = cli.getOwnID()
				}
			}
			var decryptMessageID string
			if info.MsgBotInfo.EditType == types.EditTypeInner || info.MsgBotInfo.EditType == types.EditTypeLast {
				decryptMessageID = info.MsgBotInfo.EditTargetID
			} else {
				decryptMessageID = info.ID
			}
			var msMsg waE2E.MessageSecretMessage
			var messageSecret []byte
			if messageSecret, _, err = cli.Store.MsgSecrets.GetMessageSecret(ctx, info.Chat, targetSenderJID, info.MsgMetaInfo.TargetID); err != nil {
				err = fmt.Errorf("failed to get message secret for %s: %v", info.MsgMetaInfo.TargetID, err)
			} else if messageSecret == nil {
				err = fmt.Errorf("message secret for %s not found", info.MsgMetaInfo.TargetID)
			} else if err = proto.Unmarshal(child.Content.([]byte), &msMsg); err != nil {
				err = fmt.Errorf("failed to unmarshal MessageSecretMessage protobuf: %v", err)
			} else {
				decrypted, err = cli.decryptBotMessage(ctx, messageSecret, &msMsg, decryptMessageID, targetSenderJID, info)
			}
		} else {
			cli.Log.Warnf("Unhandled encrypted message (type %s) from %s", encType, info.SourceString())
			continue
		}

		if errors.Is(err, EventAlreadyProcessed) {
			cli.Log.Debugf("Ignoring message %s from %s: %v", info.ID, info.SourceString(), err)
			continue
		} else if errors.Is(err, signalerror.ErrOldCounter) {
			cli.Log.Warnf("Ignoring message %s from %s: %v", info.ID, info.SourceString(), err)
			continue
		} else if err != nil {
			cli.Log.Warnf("Error decrypting message %s from %s: %v", info.ID, info.SourceString(), err)
			if ctx.Err() != nil || errors.Is(err, context.Canceled) {
				return
			}
			isUnavailable := encType == "skmsg" && !containsDirectMsg && errors.Is(err, signalerror.ErrNoSenderKeyForUser)
			if encType == "msmsg" {
				cli.backgroundIfAsyncAck(func() {
					cli.sendAck(ctx, node, NackMissingMessageSecret)
				})
			} else if cli.SynchronousAck {
				cli.sendRetryReceipt(ctx, node, info, isUnavailable)
				// TODO this probably isn't supposed to ack
				cli.sendAck(ctx, node, 0)
			} else {
				go cli.sendRetryReceipt(context.WithoutCancel(ctx), node, info, isUnavailable)
				go cli.sendAck(ctx, node, 0)
			}
			cli.dispatchEvent(&events.UndecryptableMessage{
				Info:            *info,
				IsUnavailable:   isUnavailable,
				DecryptFailMode: events.DecryptFailMode(ag.OptionalString("decrypt-fail")),
			})
			return
		}
		retryCount := ag.OptionalInt("count")
		cli.cancelDelayedRequestFromPhone(info.ID)

		var msg waE2E.Message
		var handlerFailed bool
		switch ag.Int("v") {
		case 2:
			err = proto.Unmarshal(decrypted, &msg)
			if err != nil {
				cli.Log.Warnf("Error unmarshaling decrypted message from %s: %v", info.SourceString(), err)
				protobufFailed = true
				continue
			}
			protobufFailed = false
			handlerFailed = cli.handleDecryptedMessage(ctx, info, &msg, retryCount)
		case 3:
			handlerFailed, protobufFailed = cli.handleDecryptedArmadillo(ctx, info, decrypted, retryCount)
		default:
			cli.Log.Warnf("Unknown version %d in decrypted message from %s", ag.Int("v"), info.SourceString())
		}
		if handlerFailed {
			cli.Log.Warnf("Handler for %s failed", info.ID)
			return
		}
		if ciphertextHash != nil && cli.EnableDecryptedEventBuffer {
			// Use the context passed to decryptMessages
			err = cli.Store.EventBuffer.ClearBufferedEventPlaintext(ctx, *ciphertextHash)
			if err != nil {
				zerolog.Ctx(ctx).Err(err).
					Hex("ciphertext_hash", ciphertextHash[:]).
					Str("message_id", info.ID).
					Msg("Failed to clear buffered event plaintext")
			} else {
				zerolog.Ctx(ctx).Debug().
					Hex("ciphertext_hash", ciphertextHash[:]).
					Str("message_id", info.ID).
					Msg("Deleted event plaintext from buffer")
			}

			if time.Since(cli.lastDecryptedBufferClear) > 12*time.Hour && ctx.Err() == nil {
				cli.lastDecryptedBufferClear = time.Now()
				go func() {
					err := cli.Store.EventBuffer.DeleteOldBufferedHashes(context.WithoutCancel(ctx))
					if err != nil {
						zerolog.Ctx(ctx).Err(err).Msg("Failed to delete old buffered hashes")
					}
				}()
			}
		}
	}
	cli.backgroundIfAsyncAck(func() {
		if !recognizedStanza {
			cli.sendAck(ctx, node, NackUnrecognizedStanza)
		} else if protobufFailed {
			cli.sendAck(ctx, node, NackInvalidProtobuf)
		} else {
			cli.sendMessageReceipt(ctx, info, node)
		}
	})
	return
}

func (cli *Client) clearUntrustedIdentity(ctx context.Context, target types.JID) error {
	err := cli.Store.Identities.DeleteIdentity(ctx, target.SignalAddress().String())
	if err != nil {
		return fmt.Errorf("failed to delete identity: %w", err)
	}
	err = cli.Store.Sessions.DeleteSession(ctx, target.SignalAddress().String())
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	go cli.dispatchEvent(&events.IdentityChange{JID: target, Timestamp: time.Now(), Implicit: true})
	return nil
}

var EventAlreadyProcessed = errors.New("event was already processed")

func (cli *Client) bufferedDecrypt(
	ctx context.Context,
	ciphertext []byte,
	serverTimestamp time.Time,
	decrypt func(context.Context) ([]byte, error),
) (plaintext []byte, ciphertextHash [32]byte, err error) {
	if !cli.EnableDecryptedEventBuffer {
		plaintext, err = decrypt(ctx)
		return
	}
	ciphertextHash = sha256.Sum256(ciphertext)
	var buf *store.BufferedEvent
	buf, err = cli.Store.EventBuffer.GetBufferedEvent(ctx, ciphertextHash)
	if err != nil {
		err = fmt.Errorf("failed to get buffered event: %w", err)
		return
	} else if buf != nil {
		if buf.Plaintext == nil {
			zerolog.Ctx(ctx).Debug().
				Hex("ciphertext_hash", ciphertextHash[:]).
				Time("insertion_time", buf.InsertTime).
				Msg("Returning event already processed error")
			err = fmt.Errorf("%w at %s", EventAlreadyProcessed, buf.InsertTime.String())
			return
		}
		zerolog.Ctx(ctx).Debug().
			Hex("ciphertext_hash", ciphertextHash[:]).
			Time("insertion_time", buf.InsertTime).
			Msg("Returning previously decrypted plaintext")
		plaintext = buf.Plaintext
		return
	}

	err = cli.Store.EventBuffer.DoDecryptionTxn(ctx, func(ctx context.Context) (innerErr error) {
		plaintext, innerErr = decrypt(ctx)
		if innerErr != nil {
			return
		}
		innerErr = cli.Store.EventBuffer.PutBufferedEvent(ctx, ciphertextHash, plaintext, serverTimestamp)
		if innerErr != nil {
			innerErr = fmt.Errorf("failed to save decrypted event to buffer: %w", innerErr)
		}
		return
	})
	if err == nil {
		zerolog.Ctx(ctx).Debug().
			Hex("ciphertext_hash", ciphertextHash[:]).
			Msg("Successfully decrypted and saved event")
	}
	return
}

func (cli *Client) decryptDM(ctx context.Context, child *waBinary.Node, from types.JID, isPreKey bool, serverTS time.Time) ([]byte, *[32]byte, error) {
	content, ok := child.Content.([]byte)
	if !ok {
		return nil, nil, fmt.Errorf("message content is not a byte slice")
	}

	builder := session.NewBuilderFromSignal(cli.Store, from.SignalAddress(), pbSerializer)
	cipher := session.NewCipher(builder, from.SignalAddress())
	var plaintext []byte
	var ciphertextHash [32]byte
	if isPreKey {
		preKeyMsg, err := protocol.NewPreKeySignalMessageFromBytes(content, pbSerializer.PreKeySignalMessage, pbSerializer.SignalMessage)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse prekey message: %w", err)
		}
		plaintext, ciphertextHash, err = cli.bufferedDecrypt(ctx, content, serverTS, func(decryptCtx context.Context) ([]byte, error) {
			pt, innerErr := cipher.DecryptMessage(decryptCtx, preKeyMsg)
			if cli.AutoTrustIdentity && errors.Is(innerErr, signalerror.ErrUntrustedIdentity) {
				cli.Log.Warnf("Got %v error while trying to decrypt prekey message from %s, clearing stored identity and retrying", innerErr, from)
				if innerErr = cli.clearUntrustedIdentity(decryptCtx, from); innerErr != nil {
					innerErr = fmt.Errorf("failed to clear untrusted identity: %w", innerErr)
					return nil, innerErr
				}
				pt, innerErr = cipher.DecryptMessage(decryptCtx, preKeyMsg)
			}
			return pt, innerErr
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decrypt prekey message: %w", err)
		}
	} else {
		msg, err := protocol.NewSignalMessageFromBytes(content, pbSerializer.SignalMessage)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse normal message: %w", err)
		}
		plaintext, ciphertextHash, err = cli.bufferedDecrypt(ctx, content, serverTS, func(decryptCtx context.Context) ([]byte, error) {
			return cipher.Decrypt(decryptCtx, msg)
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decrypt normal message: %w", err)
		}
	}
	var err error
	plaintext, err = unpadMessage(plaintext, child.AttrGetter().Int("v"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to unpad message: %w", err)
	}
	return plaintext, &ciphertextHash, nil
}

func (cli *Client) decryptGroupMsg(ctx context.Context, child *waBinary.Node, from types.JID, chat types.JID, serverTS time.Time) ([]byte, *[32]byte, error) {
	content, ok := child.Content.([]byte)
	if !ok {
		return nil, nil, fmt.Errorf("message content is not a byte slice")
	}

	senderKeyName := protocol.NewSenderKeyName(chat.String(), from.SignalAddress())
	builder := groups.NewGroupSessionBuilder(cli.Store, pbSerializer)
	cipher := groups.NewGroupCipher(builder, senderKeyName, cli.Store)
	msg, err := protocol.NewSenderKeyMessageFromBytes(content, pbSerializer.SenderKeyMessage)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse group message: %w", err)
	}
	plaintext, ciphertextHash, err := cli.bufferedDecrypt(ctx, content, serverTS, func(decryptCtx context.Context) ([]byte, error) {
		return cipher.Decrypt(decryptCtx, msg)
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decrypt group message: %w", err)
	}
	plaintext, err = unpadMessage(plaintext, child.AttrGetter().Int("v"))
	if err != nil {
		return nil, nil, err
	}
	return plaintext, &ciphertextHash, nil
}

const checkPadding = true

func isValidPadding(plaintext []byte) bool {
	lastByte := plaintext[len(plaintext)-1]
	expectedPadding := bytes.Repeat([]byte{lastByte}, int(lastByte))
	return bytes.HasSuffix(plaintext, expectedPadding)
}

func unpadMessage(plaintext []byte, version int) ([]byte, error) {
	if version == 3 {
		return plaintext, nil
	} else if len(plaintext) == 0 {
		return nil, fmt.Errorf("plaintext is empty")
	} else if checkPadding && !isValidPadding(plaintext) {
		return nil, fmt.Errorf("plaintext doesn't have expected padding")
	} else {
		return plaintext[:len(plaintext)-int(plaintext[len(plaintext)-1])], nil
	}
}

func padMessage(plaintext []byte) []byte {
	pad := random.Bytes(1)
	pad[0] &= 0xf
	if pad[0] == 0 {
		pad[0] = 0xf
	}
	plaintext = append(plaintext, bytes.Repeat(pad, int(pad[0]))...)
	return plaintext
}

func (cli *Client) handleSenderKeyDistributionMessage(ctx context.Context, chat, from types.JID, axolotlSKDM []byte) {
	builder := groups.NewGroupSessionBuilder(cli.Store, pbSerializer)
	senderKeyName := protocol.NewSenderKeyName(chat.String(), from.SignalAddress())
	sdkMsg, err := protocol.NewSenderKeyDistributionMessageFromBytes(axolotlSKDM, pbSerializer.SenderKeyDistributionMessage)
	if err != nil {
		cli.Log.Errorf("Failed to parse sender key distribution message from %s for %s: %v", from, chat, err)
		return
	}
	err = builder.Process(ctx, senderKeyName, sdkMsg)
	if err != nil {
		cli.Log.Errorf("Failed to process sender key distribution message from %s for %s: %v", from, chat, err)
		return
	}
	cli.Log.Debugf("Processed sender key distribution message from %s in %s", senderKeyName.Sender().String(), senderKeyName.GroupID())
}

func (cli *Client) handleHistorySyncNotificationLoop() {
	defer func() {
		cli.historySyncHandlerStarted.Store(false)
		err := recover()
		if err != nil {
			cli.Log.Errorf("History sync handler panicked: %v\n%s", err, debug.Stack())
		}

		// Check in case something new appeared in the channel between the loop stopping
		// and the atomic variable being updated. If yes, restart the loop.
		if len(cli.historySyncNotifications) > 0 && cli.historySyncHandlerStarted.CompareAndSwap(false, true) {
			cli.Log.Warnf("New history sync notifications appeared after loop stopped, restarting loop...")
			go cli.handleHistorySyncNotificationLoop()
		}
	}()
	ctx := cli.BackgroundEventCtx
	for {
		select {
		case notif := <-cli.historySyncNotifications:
			blob, err := cli.DownloadHistorySync(ctx, notif, false)
			if err != nil {
				cli.Log.Errorf("Failed to download history sync: %v", err)
			} else {
				cli.dispatchEvent(&events.HistorySync{Data: blob})
			}
		case <-time.After(1 * time.Minute):
			return
		}
	}
}

// DownloadHistorySync will download and parse the history sync blob from the given history sync notification.
//
// You only need to call this manually if you set [Client.ManualHistorySyncDownload] to true.
// By default, whatsmeow will call this automatically and dispatch an [events.HistorySync] with the parsed data.
func (cli *Client) DownloadHistorySync(ctx context.Context, notif *waE2E.HistorySyncNotification, synchronousStorage bool) (*waHistorySync.HistorySync, error) {
	var data []byte
	var err error
	if notif.InitialHistBootstrapInlinePayload != nil {
		data = notif.InitialHistBootstrapInlinePayload
	} else if data, err = cli.Download(ctx, notif); err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	var historySync waHistorySync.HistorySync
	if reader, err := zlib.NewReader(bytes.NewReader(data)); err != nil {
		return nil, fmt.Errorf("failed to prepare to decompress: %w", err)
	} else if rawData, err := io.ReadAll(reader); err != nil {
		return nil, fmt.Errorf("failed to decompress: %w", err)
	} else if err = proto.Unmarshal(rawData, &historySync); err != nil {
		return nil, fmt.Errorf("failed to unmarshal: %w", err)
	}
	cli.Log.Debugf("Received history sync (type %s, chunk %d, progress %d)", historySync.GetSyncType(), historySync.GetChunkOrder(), historySync.GetProgress())
	doStorage := func(ctx context.Context) {
		if historySync.GetSyncType() == waHistorySync.HistorySync_PUSH_NAME {
			cli.handleHistoricalPushNames(ctx, historySync.GetPushnames())
		} else if len(historySync.GetConversations()) > 0 {
			cli.storeHistoricalMessageSecrets(ctx, historySync.GetConversations())
		}
		if len(historySync.GetPhoneNumberToLidMappings()) > 0 {
			cli.storeHistoricalPNLIDMappings(ctx, historySync.GetPhoneNumberToLidMappings())
		}
		if historySync.GlobalSettings != nil {
			cli.storeGlobalSettings(ctx, historySync.GlobalSettings)
		}
	}
	if synchronousStorage {
		doStorage(ctx)
	} else {
		go doStorage(context.WithoutCancel(ctx))
	}
	return &historySync, nil
}

func (cli *Client) handleAppStateSyncKeyShare(ctx context.Context, keys *waE2E.AppStateSyncKeyShare) {
	onlyResyncIfNotSynced := true

	cli.Log.Debugf("Got %d new app state keys", len(keys.GetKeys()))
	cli.appStateKeyRequestsLock.RLock()
	for _, key := range keys.GetKeys() {
		marshaledFingerprint, err := proto.Marshal(key.GetKeyData().GetFingerprint())
		if err != nil {
			cli.Log.Errorf("Failed to marshal fingerprint of app state sync key %X", key.GetKeyID().GetKeyID())
			continue
		}
		_, isReRequest := cli.appStateKeyRequests[hex.EncodeToString(key.GetKeyID().GetKeyID())]
		if isReRequest {
			onlyResyncIfNotSynced = false
		}
		err = cli.Store.AppStateKeys.PutAppStateSyncKey(ctx, key.GetKeyID().GetKeyID(), store.AppStateSyncKey{
			Data:        key.GetKeyData().GetKeyData(),
			Fingerprint: marshaledFingerprint,
			Timestamp:   key.GetKeyData().GetTimestamp(),
		})
		if err != nil {
			cli.Log.Errorf("Failed to store app state sync key %X: %v", key.GetKeyID().GetKeyID(), err)
			continue
		}
		cli.Log.Debugf("Received app state sync key %X (ts: %d)", key.GetKeyID().GetKeyID(), key.GetKeyData().GetTimestamp())
	}
	cli.appStateKeyRequestsLock.RUnlock()

	for _, name := range appstate.AllPatchNames {
		err := cli.FetchAppState(ctx, name, false, onlyResyncIfNotSynced)
		if err != nil {
			cli.Log.Errorf("Failed to do initial fetch of app state %s: %v", name, err)
		}
	}
}

func (cli *Client) handlePlaceholderResendResponse(msg *waE2E.PeerDataOperationRequestResponseMessage) (ok bool) {
	reqID := msg.GetStanzaID()
	parts := msg.GetPeerDataOperationResult()
	cli.Log.Debugf("Handling response to placeholder resend request %s with %d items", reqID, len(parts))
	ok = true
	for i, part := range parts {
		var webMsg waWeb.WebMessageInfo
		if resp := part.GetPlaceholderMessageResendResponse(); resp == nil {
			cli.Log.Warnf("Missing response in item #%d of response to %s", i+1, reqID)
		} else if err := proto.Unmarshal(resp.GetWebMessageInfoBytes(), &webMsg); err != nil {
			cli.Log.Warnf("Failed to unmarshal protobuf web message in item #%d of response to %s: %v", i+1, reqID, err)
		} else if msgEvt, err := cli.ParseWebMessage(types.EmptyJID, &webMsg); err != nil {
			cli.Log.Warnf("Failed to parse web message info in item #%d of response to %s: %v", i+1, reqID, err)
		} else {
			msgEvt.UnavailableRequestID = reqID
			ok = !cli.dispatchEvent(msgEvt) && ok
		}
	}
	return
}

func (cli *Client) handleProtocolMessage(ctx context.Context, info *types.MessageInfo, msg *waE2E.Message) (ok bool) {
	ok = true
	protoMsg := msg.GetProtocolMessage()

	if !info.IsFromMe {
		return
	}

	if protoMsg.GetHistorySyncNotification() != nil {
		if !cli.ManualHistorySyncDownload {
			cli.historySyncNotifications <- protoMsg.HistorySyncNotification
			if cli.historySyncHandlerStarted.CompareAndSwap(false, true) {
				go cli.handleHistorySyncNotificationLoop()
			}
		}
		go cli.sendProtocolMessageReceipt(ctx, info.ID, types.ReceiptTypeHistorySync)
	}

	if protoMsg.GetLidMigrationMappingSyncMessage() != nil {
		cli.storeLIDSyncMessage(ctx, protoMsg.GetLidMigrationMappingSyncMessage().GetEncodedMappingPayload())
	}

	if info.Sender.Device == 0 {
		peerResp := protoMsg.GetPeerDataOperationRequestResponseMessage()
		switch peerResp.GetPeerDataOperationRequestType() {
		case waE2E.PeerDataOperationRequestType_PLACEHOLDER_MESSAGE_RESEND:
			ok = cli.handlePlaceholderResendResponse(peerResp) && ok
		case waE2E.PeerDataOperationRequestType_COMPANION_SYNCD_SNAPSHOT_FATAL_RECOVERY:
			ok = cli.handleAppStateRecovery(ctx, peerResp.GetStanzaID(), peerResp.GetPeerDataOperationResult()) && ok
		}
	}

	if protoMsg.GetAppStateSyncKeyShare() != nil {
		go cli.handleAppStateSyncKeyShare(context.WithoutCancel(ctx), protoMsg.AppStateSyncKeyShare)
	}

	if info.Category == "peer" {
		go cli.sendProtocolMessageReceipt(ctx, info.ID, types.ReceiptTypePeerMsg)
	}
	return
}

func (cli *Client) processProtocolParts(ctx context.Context, info *types.MessageInfo, msg *waE2E.Message) (ok bool) {
	ok = true
	cli.storeMessageSecret(ctx, info, msg)
	// Hopefully sender key distribution messages and protocol messages can't be inside ephemeral messages
	if msg.GetDeviceSentMessage().GetMessage() != nil {
		msg = msg.GetDeviceSentMessage().GetMessage()
	}
	if msg.GetSenderKeyDistributionMessage() != nil {
		if !info.IsGroup {
			cli.Log.Warnf("Got sender key distribution message in non-group chat from %s", info.Sender)
		} else {
			encryptionIdentity := info.Sender
			if encryptionIdentity.Server == types.DefaultUserServer && info.SenderAlt.Server == types.HiddenUserServer {
				encryptionIdentity = info.SenderAlt
			}
			cli.handleSenderKeyDistributionMessage(ctx, info.Chat, encryptionIdentity, msg.SenderKeyDistributionMessage.AxolotlSenderKeyDistributionMessage)
		}
	}
	// N.B. Edits are protocol messages, but they're also wrapped inside EditedMessage,
	// which is only unwrapped after processProtocolParts, so this won't trigger for edits.
	if msg.GetProtocolMessage() != nil {
		ok = cli.handleProtocolMessage(ctx, info, msg) && ok
	}
	return
}

func (cli *Client) storeMessageSecret(ctx context.Context, info *types.MessageInfo, msg *waE2E.Message) {
	if msgSecret := msg.GetMessageContextInfo().GetMessageSecret(); len(msgSecret) > 0 {
		err := cli.Store.MsgSecrets.PutMessageSecret(ctx, info.Chat, info.Sender, info.ID, msgSecret)
		if err != nil {
			cli.Log.Errorf("Failed to store message secret key for %s: %v", info.ID, err)
		} else {
			cli.Log.Debugf("Stored message secret key for %s", info.ID)
		}
	}
}

func (cli *Client) storeHistoricalMessageSecrets(ctx context.Context, conversations []*waHistorySync.Conversation) {
	var secrets []store.MessageSecretInsert
	var privacyTokens []store.PrivacyToken
	ownID := cli.getOwnID().ToNonAD()
	if ownID.IsEmpty() {
		return
	}
	for _, conv := range conversations {
		chatJID, _ := types.ParseJID(conv.GetID())
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
				if senderJID.IsEmpty() || msgKey.GetID() == "" {
					continue
				}
				secrets = append(secrets, store.MessageSecretInsert{
					Chat:   chatJID,
					Sender: senderJID,
					ID:     msgKey.GetID(),
					Secret: secret,
				})
			}
		}
	}
	if len(secrets) > 0 {
		cli.Log.Debugf("Storing %d message secret keys in history sync", len(secrets))
		err := cli.Store.MsgSecrets.PutMessageSecrets(ctx, secrets)
		if err != nil {
			cli.Log.Errorf("Failed to store message secret keys in history sync: %v", err)
		} else {
			cli.Log.Infof("Stored %d message secret keys from history sync", len(secrets))
		}
	}
	if len(privacyTokens) > 0 {
		cli.Log.Debugf("Storing %d privacy tokens in history sync", len(privacyTokens))
		err := cli.Store.PrivacyTokens.PutPrivacyTokens(ctx, privacyTokens...)
		if err != nil {
			cli.Log.Errorf("Failed to store privacy tokens in history sync: %v", err)
		} else {
			cli.Log.Infof("Stored %d privacy tokens from history sync", len(privacyTokens))
		}
	}
}

func (cli *Client) storeLIDSyncMessage(ctx context.Context, msg []byte) {
	var decoded waLidMigrationSyncPayload.LIDMigrationMappingSyncPayload
	err := proto.Unmarshal(msg, &decoded)
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("Failed to unmarshal LID migration mapping sync payload")
		return
	}
	if cli.Store.LIDMigrationTimestamp == 0 && decoded.GetChatDbMigrationTimestamp() > 0 {
		cli.Store.LIDMigrationTimestamp = int64(decoded.GetChatDbMigrationTimestamp())
		err = cli.Store.Save(ctx)
		if err != nil {
			zerolog.Ctx(ctx).Err(err).
				Int64("lid_migration_timestamp", cli.Store.LIDMigrationTimestamp).
				Msg("Failed to save chat DB LID migration timestamp")
		} else {
			zerolog.Ctx(ctx).Debug().
				Int64("lid_migration_timestamp", cli.Store.LIDMigrationTimestamp).
				Msg("Saved chat DB LID migration timestamp")
		}
	}
	lidPairs := make([]store.LIDMapping, len(decoded.PnToLidMappings))
	for i, mapping := range decoded.PnToLidMappings {
		lidPairs[i] = store.LIDMapping{
			LID: types.JID{User: strconv.FormatUint(mapping.GetAssignedLid(), 10), Server: types.HiddenUserServer},
			PN:  types.JID{User: strconv.FormatUint(mapping.GetPn(), 10), Server: types.DefaultUserServer},
		}
	}
	err = cli.Store.LIDs.PutManyLIDMappings(ctx, lidPairs)
	if err != nil {
		zerolog.Ctx(ctx).Err(err).
			Int("pair_count", len(lidPairs)).
			Msg("Failed to store phone number to LID mappings from sync message")
	} else {
		zerolog.Ctx(ctx).Debug().
			Int("pair_count", len(lidPairs)).
			Msg("Stored PN-LID mappings from sync message")
	}
}

func (cli *Client) storeGlobalSettings(ctx context.Context, settings *waHistorySync.GlobalSettings) {
	if cli.Store.LIDMigrationTimestamp == 0 && settings.GetChatDbLidMigrationTimestamp() > 0 {
		cli.Store.LIDMigrationTimestamp = settings.GetChatDbLidMigrationTimestamp()
		err := cli.Store.Save(ctx)
		if err != nil {
			zerolog.Ctx(ctx).Err(err).
				Int64("lid_migration_timestamp", cli.Store.LIDMigrationTimestamp).
				Msg("Failed to save chat DB LID migration timestamp")
		} else {
			zerolog.Ctx(ctx).Debug().
				Int64("lid_migration_timestamp", cli.Store.LIDMigrationTimestamp).
				Msg("Saved chat DB LID migration timestamp")
		}
	}
}

func (cli *Client) storeHistoricalPNLIDMappings(ctx context.Context, mappings []*waHistorySync.PhoneNumberToLIDMapping) {
	lidPairs := make([]store.LIDMapping, 0, len(mappings))
	for _, mapping := range mappings {
		pn, err := types.ParseJID(mapping.GetPnJID())
		if err != nil {
			zerolog.Ctx(ctx).Err(err).
				Str("pn_jid", mapping.GetPnJID()).
				Str("lid_jid", mapping.GetLidJID()).
				Msg("Failed to parse phone number from history sync")
			continue
		}
		if pn.Server == types.LegacyUserServer {
			pn.Server = types.DefaultUserServer
		}
		lid, err := types.ParseJID(mapping.GetLidJID())
		if err != nil {
			zerolog.Ctx(ctx).Err(err).
				Str("pn_jid", mapping.GetPnJID()).
				Str("lid_jid", mapping.GetLidJID()).
				Msg("Failed to parse LID from history sync")
			continue
		}
		lidPairs = append(lidPairs, store.LIDMapping{
			LID: lid,
			PN:  pn,
		})
	}
	err := cli.Store.LIDs.PutManyLIDMappings(ctx, lidPairs)
	if err != nil {
		zerolog.Ctx(ctx).Err(err).
			Int("pair_count", len(lidPairs)).
			Msg("Failed to store phone number to LID mappings from history sync")
	} else {
		zerolog.Ctx(ctx).Debug().
			Int("pair_count", len(lidPairs)).
			Msg("Stored PN-LID mappings from history sync")
	}
}

func (cli *Client) handleDecryptedMessage(ctx context.Context, info *types.MessageInfo, msg *waE2E.Message, retryCount int) (handlerFailed bool) {
	ok := cli.processProtocolParts(ctx, info, msg)
	if !ok {
		return false
	}
	evt := &events.Message{Info: *info, RawMessage: msg, RetryCount: retryCount}
	return cli.dispatchEvent(evt.UnwrapRaw())
}

func (cli *Client) sendProtocolMessageReceipt(ctx context.Context, id types.MessageID, msgType types.ReceiptType) {
	if len(id) == 0 {
		return
	}
	err := cli.sendNode(ctx, waBinary.Node{
		Tag: "receipt",
		Attrs: waBinary.Attrs{
			"id":   string(id),
			"type": string(msgType),
			"to":   cli.getOwnID().ToNonAD(),
		},
		Content: nil,
	})
	if err != nil {
		cli.Log.Warnf("Failed to send acknowledgement for protocol message %s: %v", id, err)
	}
}
