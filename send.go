// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/libsignal/groups"
	"go.mau.fi/libsignal/keys/prekey"
	"go.mau.fi/libsignal/protocol"
	"go.mau.fi/libsignal/session"
	"go.mau.fi/libsignal/signalerror"
	"go.mau.fi/util/random"
	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waAICommon"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const WebMessageIDPrefix = "3EB0"

// GenerateMessageID generates a random string that can be used as a message ID on WhatsApp.
//
//	msgID := cli.GenerateMessageID()
//	cli.SendMessage(context.Background(), targetJID, &waE2E.Message{...}, whatsmeow.SendRequestExtra{ID: msgID})
func (cli *Client) GenerateMessageID() types.MessageID {
	if cli != nil && cli.MessengerConfig != nil {
		return types.MessageID(strconv.FormatInt(GenerateFacebookMessageID(), 10))
	}
	data := make([]byte, 8, 8+20+16)
	binary.BigEndian.PutUint64(data, uint64(time.Now().Unix()))
	ownID := cli.getOwnID()
	if !ownID.IsEmpty() {
		data = append(data, []byte(ownID.User)...)
		data = append(data, []byte("@c.us")...)
	}
	data = append(data, random.Bytes(16)...)
	hash := sha256.Sum256(data)
	return WebMessageIDPrefix + strings.ToUpper(hex.EncodeToString(hash[:9]))
}

func GenerateFacebookMessageID() int64 {
	const randomMask = (1 << 22) - 1
	return (time.Now().UnixMilli() << 22) | (int64(binary.BigEndian.Uint32(random.Bytes(4))) & randomMask)
}

// GenerateMessageID generates a random string that can be used as a message ID on WhatsApp.
//
//	msgID := whatsmeow.GenerateMessageID()
//	cli.SendMessage(context.Background(), targetJID, &waE2E.Message{...}, whatsmeow.SendRequestExtra{ID: msgID})
//
// Deprecated: WhatsApp web has switched to using a hash of the current timestamp, user id and random bytes. Use Client.GenerateMessageID instead.
func GenerateMessageID() types.MessageID {
	return WebMessageIDPrefix + strings.ToUpper(hex.EncodeToString(random.Bytes(8)))
}

type MessageDebugTimings struct {
	LIDFetch time.Duration
	Queue    time.Duration

	Marshal         time.Duration
	GetParticipants time.Duration
	GetDevices      time.Duration
	GroupEncrypt    time.Duration
	PeerEncrypt     time.Duration

	Send  time.Duration
	Resp  time.Duration
	Retry time.Duration
}

func (mdt MessageDebugTimings) MarshalZerologObject(evt *zerolog.Event) {
	if mdt.LIDFetch != 0 {
		evt.Dur("lid_fetch", mdt.LIDFetch)
	}
	evt.Dur("queue", mdt.Queue)
	evt.Dur("marshal", mdt.Marshal)
	if mdt.GetParticipants != 0 {
		evt.Dur("get_participants", mdt.GetParticipants)
	}
	evt.Dur("get_devices", mdt.GetDevices)
	if mdt.GroupEncrypt != 0 {
		evt.Dur("group_encrypt", mdt.GroupEncrypt)
	}
	evt.Dur("peer_encrypt", mdt.PeerEncrypt)
	evt.Dur("send", mdt.Send)
	evt.Dur("resp", mdt.Resp)
	if mdt.Retry != 0 {
		evt.Dur("retry", mdt.Retry)
	}
}

type SendResponse struct {
	// The message timestamp returned by the server
	Timestamp time.Time

	// The ID of the sent message
	ID types.MessageID

	// The server-specified ID of the sent message. Only present for newsletter messages.
	ServerID types.MessageServerID

	// Message handling duration, used for debugging
	DebugTimings MessageDebugTimings

	// The identity the message was sent with (LID or PN)
	// This is currently not reliable in all cases.
	Sender types.JID
}

// SendRequestExtra contains the optional parameters for SendMessage.
//
// By default, optional parameters don't have to be provided at all, e.g.
//
//	cli.SendMessage(ctx, to, message)
//
// When providing optional parameters, add a single instance of this struct as the last parameter:
//
//	cli.SendMessage(ctx, to, message, whatsmeow.SendRequestExtra{...})
//
// Trying to add multiple extra parameters will return an error.
type SendRequestExtra struct {
	// The message ID to use when sending. If this is not provided, a random message ID will be generated
	ID types.MessageID
	// JID of the bot to be invoked (optional)
	InlineBotJID types.JID
	// Should the message be sent as a peer message (protocol messages to your own devices, e.g. app state key requests)
	Peer bool
	// A timeout for the send request. Unlike timeouts using the context parameter, this only applies
	// to the actual response waiting and not preparing/encrypting the message.
	// Defaults to 75 seconds. The timeout can be disabled by using a negative value.
	Timeout time.Duration
	// When sending media to newsletters, the Handle field returned by the file upload.
	MediaHandle string

	Meta *types.MsgMetaInfo
	// use this only if you know what you are doing
	AdditionalNodes *[]waBinary.Node
}

// SendMessage sends the given message.
//
// This method will wait for the server to acknowledge the message before returning.
// The return value is the timestamp of the message from the server.
//
// Optional parameters like the message ID can be specified with the SendRequestExtra struct.
// Only one extra parameter is allowed, put all necessary parameters in the same struct.
//
// The message itself can contain anything you want (within the protobuf schema).
// e.g. for a simple text message, use the Conversation field:
//
//	cli.SendMessage(context.Background(), targetJID, &waE2E.Message{
//		Conversation: proto.String("Hello, World!"),
//	})
//
// Things like replies, mentioning users and the "forwarded" flag are stored in ContextInfo,
// which can be put in ExtendedTextMessage and any of the media message types.
//
// For uploading and sending media/attachments, see the Upload method.
//
// For other message types, you'll have to figure it out yourself. Looking at the protobuf schema
// in binary/proto/def.proto may be useful to find out all the allowed fields. Printing the RawMessage
// field in incoming message events to figure out what it contains is also a good way to learn how to
// send the same kind of message.
func (cli *Client) SendMessage(ctx context.Context, to types.JID, message *waE2E.Message, extra ...SendRequestExtra) (resp SendResponse, err error) {
	if cli == nil {
		err = ErrClientIsNil
		return
	}
	var req SendRequestExtra
	if len(extra) > 1 {
		err = errors.New("only one extra parameter may be provided to SendMessage")
		return
	} else if len(extra) == 1 {
		req = extra[0]
	}
	if to.Device > 0 && !req.Peer {
		err = ErrRecipientADJID
		return
	}
	ownID := cli.getOwnID()
	if ownID.IsEmpty() {
		err = ErrNotLoggedIn
		return
	}

	if req.Timeout == 0 {
		req.Timeout = defaultRequestTimeout
	}
	if len(req.ID) == 0 {
		req.ID = cli.GenerateMessageID()
	}
	if to.Server == types.NewsletterServer {
		// TODO somehow deduplicate this with the code in sendNewsletter?
		if message.EditedMessage != nil {
			req.ID = types.MessageID(message.GetEditedMessage().GetMessage().GetProtocolMessage().GetKey().GetID())
		} else if message.ProtocolMessage != nil && message.ProtocolMessage.GetType() == waE2E.ProtocolMessage_REVOKE {
			req.ID = types.MessageID(message.GetProtocolMessage().GetKey().GetID())
		}
	}
	resp.ID = req.ID

	isInlineBotMode := false

	if !req.InlineBotJID.IsEmpty() {
		if !req.InlineBotJID.IsBot() {
			err = ErrInvalidInlineBotID
			return
		}
		isInlineBotMode = true
	}

	isBotMode := isInlineBotMode || to.IsBot()
	needsMessageSecret := isBotMode || cli.shouldIncludeReportingToken(message)
	var extraParams nodeExtraParams

	if needsMessageSecret {
		if message.MessageContextInfo == nil {
			message.MessageContextInfo = &waE2E.MessageContextInfo{}
		}
		if message.MessageContextInfo.MessageSecret == nil {
			message.MessageContextInfo.MessageSecret = random.Bytes(32)
		}
	}

	if isBotMode {
		if message.MessageContextInfo.BotMetadata == nil {
			message.MessageContextInfo.BotMetadata = &waAICommon.BotMetadata{
				PersonaID: proto.String("867051314767696$760019659443059"),
			}
		}

		if isInlineBotMode {
			// inline mode specific code
			messageSecret := message.GetMessageContextInfo().GetMessageSecret()
			message = &waE2E.Message{
				BotInvokeMessage: &waE2E.FutureProofMessage{
					Message: &waE2E.Message{
						ExtendedTextMessage: message.ExtendedTextMessage,
						MessageContextInfo: &waE2E.MessageContextInfo{
							BotMetadata: message.MessageContextInfo.BotMetadata,
						},
					},
				},
				MessageContextInfo: message.MessageContextInfo,
			}

			botMessage := &waE2E.Message{
				BotInvokeMessage: message.BotInvokeMessage,
				MessageContextInfo: &waE2E.MessageContextInfo{
					BotMetadata:      message.MessageContextInfo.BotMetadata,
					BotMessageSecret: applyBotMessageHKDF(messageSecret),
				},
			}

			messagePlaintext, _, marshalErr := marshalMessage(req.InlineBotJID, botMessage)
			if marshalErr != nil {
				err = marshalErr
				return
			}

			var participantNodes []waBinary.Node
			participantNodes, _, err = cli.encryptMessageForDevices(ctx, []types.JID{req.InlineBotJID}, resp.ID, messagePlaintext, nil, waBinary.Attrs{})
			if err != nil {
				return
			}
			extraParams.botNode = &waBinary.Node{
				Tag:     "bot",
				Attrs:   nil,
				Content: participantNodes,
			}
		}
	}

	var groupParticipants []types.JID
	if to.Server == types.GroupServer || to.Server == types.BroadcastServer {
		start := time.Now()
		if to.Server == types.GroupServer {
			var cachedData *groupMetaCache
			cachedData, err = cli.getCachedGroupData(ctx, to)
			if err != nil {
				err = fmt.Errorf("failed to get group members: %w", err)
				return
			}
			groupParticipants = cachedData.Members
			// TODO this is fairly hacky, is there a proper way to determine which identity the message is sent with?
			if cachedData.AddressingMode == types.AddressingModeLID {
				ownID = cli.getOwnLID()
				extraParams.addressingMode = types.AddressingModeLID
			} else if cachedData.CommunityAnnouncementGroup && req.Meta != nil {
				ownID = cli.getOwnLID()
				// Why is this set to PN?
				extraParams.addressingMode = types.AddressingModePN
			}
		} else {
			groupParticipants, err = cli.getBroadcastListParticipants(ctx, to)
			if err != nil {
				err = fmt.Errorf("failed to get broadcast list members: %w", err)
				return
			}
		}
		resp.DebugTimings.GetParticipants = time.Since(start)
	} else if to.Server == types.HiddenUserServer {
		ownID = cli.getOwnLID()
	} else if to.Server == types.DefaultUserServer && cli.Store.LIDMigrationTimestamp > 0 && !req.Peer {
		start := time.Now()
		var toLID types.JID
		toLID, err = cli.Store.LIDs.GetLIDForPN(ctx, to)
		if err != nil {
			err = fmt.Errorf("failed to get LID for PN %s: %w", to, err)
			return
		} else if toLID.IsEmpty() {
			var info map[types.JID]types.UserInfo
			info, err = cli.GetUserInfo(ctx, []types.JID{to})
			if err != nil {
				err = fmt.Errorf("failed to get user info for %s to fill LID cache: %w", to, err)
				return
			} else if toLID = info[to].LID; toLID.IsEmpty() {
				err = fmt.Errorf("no LID found for %s from server", to)
				return
			}
		}
		resp.DebugTimings.LIDFetch = time.Since(start)
		cli.Log.Debugf("Replacing SendMessage destination with LID as migration timestamp is set %s -> %s", to, toLID)
		to = toLID
		ownID = cli.getOwnLID()
	}
	if req.Meta != nil {
		extraParams.metaNode = &waBinary.Node{
			Tag:   "meta",
			Attrs: waBinary.Attrs{},
		}
		if req.Meta.DeprecatedLIDSession != nil {
			extraParams.metaNode.Attrs["deprecated_lid_session"] = *req.Meta.DeprecatedLIDSession
		}
		if req.Meta.ThreadMessageID != "" {
			extraParams.metaNode.Attrs["thread_msg_id"] = req.Meta.ThreadMessageID
			extraParams.metaNode.Attrs["thread_msg_sender_jid"] = req.Meta.ThreadMessageSenderJID
		}
	}

	if req.AdditionalNodes != nil {
		extraParams.additionalNodes = req.AdditionalNodes
	}

	resp.Sender = ownID

	start := time.Now()
	// Sending multiple messages at a time can cause weird issues and makes it harder to retry safely
	// This is also required for the session prefetching that makes group sends faster
	// (everything will explode if you send a message to the same user twice in parallel)
	cli.messageSendLock.Lock()
	resp.DebugTimings.Queue = time.Since(start)
	defer cli.messageSendLock.Unlock()

	// Peer message retries aren't implemented yet
	if !req.Peer {
		err = cli.addRecentMessage(ctx, to, req.ID, message, nil)
		if err != nil {
			return
		}
	}

	if message.GetMessageContextInfo().GetMessageSecret() != nil {
		err = cli.Store.MsgSecrets.PutMessageSecret(ctx, to, ownID, req.ID, message.GetMessageContextInfo().GetMessageSecret())
		if err != nil {
			cli.Log.Warnf("Failed to store message secret key for outgoing message %s: %v", req.ID, err)
		} else {
			cli.Log.Debugf("Stored message secret key for outgoing message %s", req.ID)
		}
	}

	respChan := cli.waitResponse(req.ID)
	var phash string
	var data []byte
	switch to.Server {
	case types.GroupServer, types.BroadcastServer:
		phash, data, err = cli.sendGroup(ctx, ownID, to, groupParticipants, req.ID, message, &resp.DebugTimings, extraParams)
	case types.DefaultUserServer, types.BotServer, types.HiddenUserServer:
		if req.Peer {
			data, err = cli.sendPeerMessage(ctx, to, req.ID, message, &resp.DebugTimings)
		} else {
			phash, data, err = cli.sendDM(ctx, ownID, to, req.ID, message, &resp.DebugTimings, extraParams)
		}
	case types.NewsletterServer:
		data, err = cli.sendNewsletter(ctx, to, req.ID, message, req.MediaHandle, &resp.DebugTimings)
	default:
		err = fmt.Errorf("%w %s", ErrUnknownServer, to.Server)
	}
	start = time.Now()
	if err != nil {
		cli.cancelResponse(req.ID, respChan)
		return
	}
	var respNode *waBinary.Node
	var timeoutChan <-chan time.Time
	if req.Timeout > 0 {
		timeoutChan = time.After(req.Timeout)
	} else {
		timeoutChan = make(<-chan time.Time)
	}
	select {
	case respNode = <-respChan:
	case <-timeoutChan:
		cli.cancelResponse(req.ID, respChan)
		err = ErrMessageTimedOut
		return
	case <-ctx.Done():
		cli.cancelResponse(req.ID, respChan)
		err = ctx.Err()
		return
	}
	resp.DebugTimings.Resp = time.Since(start)
	if isDisconnectNode(respNode) {
		start = time.Now()
		respNode, err = cli.retryFrame(ctx, "message send", req.ID, data, respNode, 0)
		resp.DebugTimings.Retry = time.Since(start)
		if err != nil {
			return
		}
	}
	ag := respNode.AttrGetter()
	resp.ServerID = types.MessageServerID(ag.OptionalInt("server_id"))
	resp.Timestamp = ag.UnixTime("t")
	if errorCode := ag.Int("error"); errorCode != 0 {
		err = fmt.Errorf("%w %d", ErrServerReturnedError, errorCode)
	}
	expectedPHash := ag.OptionalString("phash")
	if len(expectedPHash) > 0 && phash != expectedPHash {
		cli.Log.Warnf("Server returned different participant list hash (%s != %s) when sending to %s. Some devices may not have received the message.", phash, expectedPHash, to)
		switch to.Server {
		case types.GroupServer:
			// TODO also invalidate device list caches
			cli.groupCacheLock.Lock()
			delete(cli.groupCache, to)
			cli.groupCacheLock.Unlock()
		case types.BroadcastServer:
			// TODO do something
		case types.DefaultUserServer, types.HiddenUserServer, types.BotServer, types.HostedServer, types.HostedLIDServer:
			cli.userDevicesCacheLock.Lock()
			delete(cli.userDevicesCache, to)
			cli.userDevicesCacheLock.Unlock()
		}
	}
	return
}

func (cli *Client) SendPeerMessage(ctx context.Context, message *waE2E.Message) (SendResponse, error) {
	ownID := cli.getOwnID().ToNonAD()
	if ownID.IsEmpty() {
		return SendResponse{}, ErrNotLoggedIn
	}
	return cli.SendMessage(ctx, ownID, message, SendRequestExtra{Peer: true})
}

// RevokeMessage deletes the given message from everyone in the chat.
//
// This method will wait for the server to acknowledge the revocation message before returning.
// The return value is the timestamp of the message from the server.
//
// Deprecated: This method is deprecated in favor of BuildRevoke
func (cli *Client) RevokeMessage(ctx context.Context, chat types.JID, id types.MessageID) (SendResponse, error) {
	return cli.SendMessage(ctx, chat, cli.BuildRevoke(chat, types.EmptyJID, id))
}

// BuildMessageKey builds a MessageKey object, which is used to refer to previous messages
// for things such as replies, revocations and reactions.
func (cli *Client) BuildMessageKey(chat, sender types.JID, id types.MessageID) *waCommon.MessageKey {
	key := &waCommon.MessageKey{
		FromMe:    proto.Bool(true),
		ID:        proto.String(id),
		RemoteJID: proto.String(chat.String()),
	}
	if !sender.IsEmpty() && sender.User != cli.getOwnID().User && sender.User != cli.getOwnLID().User {
		key.FromMe = proto.Bool(false)
		if chat.Server != types.DefaultUserServer && chat.Server != types.HiddenUserServer && chat.Server != types.MessengerServer {
			key.Participant = proto.String(sender.ToNonAD().String())
		}
	}
	return key
}

// BuildRevoke builds a message revocation message using the given variables.
// The built message can be sent normally using Client.SendMessage.
//
// To revoke your own messages, pass your JID or an empty JID as the second parameter (sender).
//
//	resp, err := cli.SendMessage(context.Background(), chat, cli.BuildRevoke(chat, types.EmptyJID, originalMessageID)
//
// To revoke someone else's messages when you are group admin, pass the message sender's JID as the second parameter.
//
//	resp, err := cli.SendMessage(context.Background(), chat, cli.BuildRevoke(chat, senderJID, originalMessageID)
func (cli *Client) BuildRevoke(chat, sender types.JID, id types.MessageID) *waE2E.Message {
	return &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Type: waE2E.ProtocolMessage_REVOKE.Enum(),
			Key:  cli.BuildMessageKey(chat, sender, id),
		},
	}
}

// BuildReaction builds a message reaction message using the given variables.
// The built message can be sent normally using Client.SendMessage.
//
//	resp, err := cli.SendMessage(context.Background(), chat, cli.BuildReaction(chat, senderJID, targetMessageID, "🐈️")
//
// Note that for newsletter messages, you need to use NewsletterSendReaction instead of BuildReaction + SendMessage.
func (cli *Client) BuildReaction(chat, sender types.JID, id types.MessageID, reaction string) *waE2E.Message {
	return &waE2E.Message{
		ReactionMessage: &waE2E.ReactionMessage{
			Key:               cli.BuildMessageKey(chat, sender, id),
			Text:              proto.String(reaction),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	}
}

// BuildUnavailableMessageRequest builds a message to request the user's primary device to send
// the copy of a message that this client was unable to decrypt.
//
// The built message can be sent using Client.SendPeerMessage.
// The full response will come as a ProtocolMessage with type `PEER_DATA_OPERATION_REQUEST_RESPONSE_MESSAGE`.
// The response events will also be dispatched as normal *events.Message's with UnavailableRequestID set to the request message ID.
func (cli *Client) BuildUnavailableMessageRequest(chat, sender types.JID, id string) *waE2E.Message {
	return &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Type: waE2E.ProtocolMessage_PEER_DATA_OPERATION_REQUEST_MESSAGE.Enum(),
			PeerDataOperationRequestMessage: &waE2E.PeerDataOperationRequestMessage{
				PeerDataOperationRequestType: waE2E.PeerDataOperationRequestType_PLACEHOLDER_MESSAGE_RESEND.Enum(),
				PlaceholderMessageResendRequest: []*waE2E.PeerDataOperationRequestMessage_PlaceholderMessageResendRequest{{
					MessageKey: cli.BuildMessageKey(chat, sender, id),
				}},
			},
		},
	}
}

// BuildHistorySyncRequest builds a message to request additional history from the user's primary device.
//
// The built message can be sent using Client.SendPeerMessage.
// The response will come as an *events.HistorySync with type `ON_DEMAND`.
//
// The response will contain to `count` messages immediately before the given message.
// The recommended number of messages to request at a time is 50.
func (cli *Client) BuildHistorySyncRequest(lastKnownMessageInfo *types.MessageInfo, count int) *waE2E.Message {
	return &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Type: waE2E.ProtocolMessage_PEER_DATA_OPERATION_REQUEST_MESSAGE.Enum(),
			PeerDataOperationRequestMessage: &waE2E.PeerDataOperationRequestMessage{
				PeerDataOperationRequestType: waE2E.PeerDataOperationRequestType_HISTORY_SYNC_ON_DEMAND.Enum(),
				HistorySyncOnDemandRequest: &waE2E.PeerDataOperationRequestMessage_HistorySyncOnDemandRequest{
					ChatJID:              proto.String(lastKnownMessageInfo.Chat.String()),
					OldestMsgID:          proto.String(lastKnownMessageInfo.ID),
					OldestMsgFromMe:      proto.Bool(lastKnownMessageInfo.IsFromMe),
					OnDemandMsgCount:     proto.Int32(int32(count)),
					OldestMsgTimestampMS: proto.Int64(lastKnownMessageInfo.Timestamp.UnixMilli()),
				},
			},
		},
	}
}

// EditWindow specifies how long a message can be edited for after it was sent.
const EditWindow = 20 * time.Minute

// BuildEdit builds a message edit message using the given variables.
// The built message can be sent normally using Client.SendMessage.
//
//	resp, err := cli.SendMessage(context.Background(), chat, cli.BuildEdit(chat, originalMessageID, &waE2E.Message{
//		Conversation: proto.String("edited message"),
//	})
func (cli *Client) BuildEdit(chat types.JID, id types.MessageID, newContent *waE2E.Message) *waE2E.Message {
	return &waE2E.Message{
		EditedMessage: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ProtocolMessage: &waE2E.ProtocolMessage{
					Key: &waCommon.MessageKey{
						FromMe:    proto.Bool(true),
						ID:        proto.String(id),
						RemoteJID: proto.String(chat.String()),
					},
					Type:          waE2E.ProtocolMessage_MESSAGE_EDIT.Enum(),
					EditedMessage: newContent,
					TimestampMS:   proto.Int64(time.Now().UnixMilli()),
				},
			},
		},
	}
}

const (
	DisappearingTimerOff     = time.Duration(0)
	DisappearingTimer24Hours = 24 * time.Hour
	DisappearingTimer7Days   = 7 * 24 * time.Hour
	DisappearingTimer90Days  = 90 * 24 * time.Hour
)

// ParseDisappearingTimerString parses common human-readable disappearing message timer strings into Duration values.
// If the string doesn't look like one of the allowed values (0, 24h, 7d, 90d), the second return value is false.
func ParseDisappearingTimerString(val string) (time.Duration, bool) {
	switch strings.ReplaceAll(strings.ToLower(val), " ", "") {
	case "0d", "0h", "0s", "0", "off":
		return DisappearingTimerOff, true
	case "1day", "day", "1d", "1", "24h", "24", "86400s", "86400":
		return DisappearingTimer24Hours, true
	case "1week", "week", "7d", "7", "168h", "168", "604800s", "604800":
		return DisappearingTimer7Days, true
	case "3months", "3m", "3mo", "90d", "90", "2160h", "2160", "7776000s", "7776000":
		return DisappearingTimer90Days, true
	default:
		return 0, false
	}
}

// SetDisappearingTimer sets the disappearing timer in a chat. Both private chats and groups are supported, but they're
// set with different methods.
//
// Note that while this function allows passing non-standard durations, official WhatsApp apps will ignore those,
// and in groups the server will just reject the change. You can use the DisappearingTimer<Duration> constants for convenience.
//
// In groups, the server will echo the change as a notification, so it'll show up as a *events.GroupInfo update.
func (cli *Client) SetDisappearingTimer(ctx context.Context, chat types.JID, timer time.Duration, settingTS time.Time) (err error) {
	switch chat.Server {
	case types.DefaultUserServer, types.HiddenUserServer:
		if settingTS.IsZero() {
			settingTS = time.Now()
		}
		_, err = cli.SendMessage(ctx, chat, &waE2E.Message{
			ProtocolMessage: &waE2E.ProtocolMessage{
				Type:                      waE2E.ProtocolMessage_EPHEMERAL_SETTING.Enum(),
				EphemeralExpiration:       proto.Uint32(uint32(timer.Seconds())),
				EphemeralSettingTimestamp: proto.Int64(settingTS.Unix()),
			},
		})
	case types.GroupServer:
		if timer == 0 {
			_, err = cli.sendGroupIQ(ctx, iqSet, chat, waBinary.Node{Tag: "not_ephemeral"})
		} else {
			_, err = cli.sendGroupIQ(ctx, iqSet, chat, waBinary.Node{
				Tag: "ephemeral",
				Attrs: waBinary.Attrs{
					"expiration": strconv.Itoa(int(timer.Seconds())),
				},
			})
			if errors.Is(err, ErrIQBadRequest) {
				err = wrapIQError(ErrInvalidDisappearingTimer, err)
			}
		}
	default:
		err = fmt.Errorf("can't set disappearing time in a %s chat", chat.Server)
	}
	return
}

func participantListHashV2(participants []types.JID) string {
	participantsStrings := make([]string, len(participants))
	for i, part := range participants {
		participantsStrings[i] = part.ADString()
	}

	sort.Strings(participantsStrings)
	hash := sha256.Sum256([]byte(strings.Join(participantsStrings, "")))
	return fmt.Sprintf("2:%s", base64.RawStdEncoding.EncodeToString(hash[:6]))
}

func (cli *Client) sendNewsletter(
	ctx context.Context,
	to types.JID,
	id types.MessageID,
	message *waE2E.Message,
	mediaID string,
	timings *MessageDebugTimings,
) ([]byte, error) {
	attrs := waBinary.Attrs{
		"to":   to,
		"id":   id,
		"type": getTypeFromMessage(message),
	}
	if mediaID != "" {
		attrs["media_id"] = mediaID
	}
	if message.EditedMessage != nil {
		attrs["edit"] = string(types.EditAttributeAdminEdit)
		message = message.GetEditedMessage().GetMessage().GetProtocolMessage().GetEditedMessage()
	} else if message.ProtocolMessage != nil && message.ProtocolMessage.GetType() == waE2E.ProtocolMessage_REVOKE {
		attrs["edit"] = string(types.EditAttributeAdminRevoke)
		message = nil
	}
	start := time.Now()
	plaintext, _, err := marshalMessage(to, message)
	timings.Marshal = time.Since(start)
	if err != nil {
		return nil, err
	}
	plaintextNode := waBinary.Node{
		Tag:     "plaintext",
		Content: plaintext,
		Attrs:   waBinary.Attrs{},
	}
	if message != nil {
		if mediaType := getMediaTypeFromMessage(message); mediaType != "" {
			plaintextNode.Attrs["mediatype"] = mediaType
		}
	}
	node := waBinary.Node{
		Tag:     "message",
		Attrs:   attrs,
		Content: []waBinary.Node{plaintextNode},
	}
	start = time.Now()
	data, err := cli.sendNodeAndGetData(ctx, node)
	timings.Send = time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("failed to send message node: %w", err)
	}
	return data, nil
}

type nodeExtraParams struct {
	botNode         *waBinary.Node
	metaNode        *waBinary.Node
	additionalNodes *[]waBinary.Node
	addressingMode  types.AddressingMode
}

func (cli *Client) sendGroup(
	ctx context.Context,
	ownID,
	to types.JID,
	participants []types.JID,
	id types.MessageID,
	message *waE2E.Message,
	timings *MessageDebugTimings,
	extraParams nodeExtraParams,
) (string, []byte, error) {
	start := time.Now()
	plaintext, _, err := marshalMessage(to, message)
	timings.Marshal = time.Since(start)
	if err != nil {
		return "", nil, err
	}

	start = time.Now()
	builder := groups.NewGroupSessionBuilder(cli.Store, pbSerializer)
	senderKeyName := protocol.NewSenderKeyName(to.String(), cli.getOwnLID().SignalAddress())
	signalSKDMessage, err := builder.Create(ctx, senderKeyName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create sender key distribution message to send %s to %s: %w", id, to, err)
	}
	skdMessage := &waE2E.Message{
		SenderKeyDistributionMessage: &waE2E.SenderKeyDistributionMessage{
			GroupID:                             proto.String(to.String()),
			AxolotlSenderKeyDistributionMessage: signalSKDMessage.Serialize(),
		},
	}
	skdPlaintext, err := proto.Marshal(skdMessage)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal sender key distribution message to send %s to %s: %w", id, to, err)
	}

	cipher := groups.NewGroupCipher(builder, senderKeyName, cli.Store)
	encrypted, err := cipher.Encrypt(ctx, padMessage(plaintext))
	if err != nil {
		return "", nil, fmt.Errorf("failed to encrypt group message to send %s to %s: %w", id, to, err)
	}
	ciphertext := encrypted.SignedSerialize()
	timings.GroupEncrypt = time.Since(start)

	node, allDevices, err := cli.prepareMessageNode(
		ctx, to, id, message, participants, skdPlaintext, nil, timings, extraParams,
	)
	if err != nil {
		return "", nil, err
	}

	phash := participantListHashV2(allDevices)
	node.Attrs["phash"] = phash
	skMsg := waBinary.Node{
		Tag:     "enc",
		Content: ciphertext,
		Attrs:   waBinary.Attrs{"v": "2", "type": "skmsg"},
	}
	if mediaType := getMediaTypeFromMessage(message); mediaType != "" {
		skMsg.Attrs["mediatype"] = mediaType
	}
	node.Content = append(node.GetChildren(), skMsg)
	if cli.shouldIncludeReportingToken(message) && message.GetMessageContextInfo().GetMessageSecret() != nil {
		node.Content = append(node.GetChildren(), cli.getMessageReportingToken(plaintext, message, ownID, to, id))
	}

	start = time.Now()
	data, err := cli.sendNodeAndGetData(ctx, *node)
	timings.Send = time.Since(start)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send message node: %w", err)
	}
	return phash, data, nil
}

func (cli *Client) sendPeerMessage(
	ctx context.Context,
	to types.JID,
	id types.MessageID,
	message *waE2E.Message,
	timings *MessageDebugTimings,
) ([]byte, error) {
	node, err := cli.preparePeerMessageNode(ctx, to, id, message, timings)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	data, err := cli.sendNodeAndGetData(ctx, *node)
	timings.Send = time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("failed to send message node: %w", err)
	}
	return data, nil
}

func (cli *Client) sendDM(
	ctx context.Context,
	ownID,
	to types.JID,
	id types.MessageID,
	message *waE2E.Message,
	timings *MessageDebugTimings,
	extraParams nodeExtraParams,
) (string, []byte, error) {
	start := time.Now()
	messagePlaintext, deviceSentMessagePlaintext, err := marshalMessage(to, message)
	timings.Marshal = time.Since(start)
	if err != nil {
		return "", nil, err
	}

	node, allDevices, err := cli.prepareMessageNode(
		ctx, to, id, message, []types.JID{to, ownID.ToNonAD()},
		messagePlaintext, deviceSentMessagePlaintext, timings, extraParams,
	)
	if err != nil {
		return "", nil, err
	}
	phash := participantListHashV2(allDevices)

	if cli.shouldIncludeReportingToken(message) && message.GetMessageContextInfo().GetMessageSecret() != nil {
		node.Content = append(node.GetChildren(), cli.getMessageReportingToken(messagePlaintext, message, ownID, to, id))
	}

	if tcToken, err := cli.Store.PrivacyTokens.GetPrivacyToken(ctx, to); err != nil {
		cli.Log.Warnf("Failed to get privacy token for %s: %v", to, err)
	} else if tcToken != nil {
		node.Content = append(node.GetChildren(), waBinary.Node{
			Tag:     "tctoken",
			Content: tcToken.Token,
		})
	}

	start = time.Now()
	data, err := cli.sendNodeAndGetData(ctx, *node)
	timings.Send = time.Since(start)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send message node: %w", err)
	}
	return phash, data, nil
}

func getTypeFromMessage(msg *waE2E.Message) string {
	switch {
	case msg.ViewOnceMessage != nil:
		return getTypeFromMessage(msg.ViewOnceMessage.Message)
	case msg.ViewOnceMessageV2 != nil:
		return getTypeFromMessage(msg.ViewOnceMessageV2.Message)
	case msg.ViewOnceMessageV2Extension != nil:
		return getTypeFromMessage(msg.ViewOnceMessageV2Extension.Message)
	case msg.LottieStickerMessage != nil:
		return getTypeFromMessage(msg.LottieStickerMessage.Message)
	case msg.EphemeralMessage != nil:
		return getTypeFromMessage(msg.EphemeralMessage.Message)
	case msg.DocumentWithCaptionMessage != nil:
		return getTypeFromMessage(msg.DocumentWithCaptionMessage.Message)
	case msg.ReactionMessage != nil, msg.EncReactionMessage != nil:
		return "reaction"
	case msg.PollCreationMessage != nil, msg.PollUpdateMessage != nil:
		return "poll"
	case getMediaTypeFromMessage(msg) != "":
		return "media"
	case msg.Conversation != nil, msg.ExtendedTextMessage != nil, msg.ProtocolMessage != nil:
		return "text"
	default:
		return "text"
	}
}

func getMediaTypeFromMessage(msg *waE2E.Message) string {
	switch {
	case msg.ViewOnceMessage != nil:
		return getMediaTypeFromMessage(msg.ViewOnceMessage.Message)
	case msg.ViewOnceMessageV2 != nil:
		return getMediaTypeFromMessage(msg.ViewOnceMessageV2.Message)
	case msg.ViewOnceMessageV2Extension != nil:
		return getMediaTypeFromMessage(msg.ViewOnceMessageV2Extension.Message)
	case msg.LottieStickerMessage != nil:
		return getMediaTypeFromMessage(msg.LottieStickerMessage.Message)
	case msg.EphemeralMessage != nil:
		return getMediaTypeFromMessage(msg.EphemeralMessage.Message)
	case msg.DocumentWithCaptionMessage != nil:
		return getMediaTypeFromMessage(msg.DocumentWithCaptionMessage.Message)
	case msg.ExtendedTextMessage != nil && msg.ExtendedTextMessage.Title != nil:
		return "url"
	case msg.ImageMessage != nil:
		return "image"
	case msg.StickerMessage != nil:
		return "sticker"
	case msg.DocumentMessage != nil:
		return "document"
	case msg.AudioMessage != nil:
		if msg.AudioMessage.GetPTT() {
			return "ptt"
		} else {
			return "audio"
		}
	case msg.VideoMessage != nil:
		if msg.VideoMessage.GetGifPlayback() {
			return "gif"
		} else {
			return "video"
		}
	case msg.ContactMessage != nil:
		return "vcard"
	case msg.ContactsArrayMessage != nil:
		return "contact_array"
	case msg.ListMessage != nil:
		return "list"
	case msg.ListResponseMessage != nil:
		return "list_response"
	case msg.ButtonsResponseMessage != nil:
		return "buttons_response"
	case msg.OrderMessage != nil:
		return "order"
	case msg.ProductMessage != nil:
		return "product"
	case msg.InteractiveResponseMessage != nil:
		return "native_flow_response"
	default:
		return ""
	}
}

func getButtonTypeFromMessage(msg *waE2E.Message) string {
	switch {
	case msg.ViewOnceMessage != nil:
		return getButtonTypeFromMessage(msg.ViewOnceMessage.Message)
	case msg.ViewOnceMessageV2 != nil:
		return getButtonTypeFromMessage(msg.ViewOnceMessageV2.Message)
	case msg.EphemeralMessage != nil:
		return getButtonTypeFromMessage(msg.EphemeralMessage.Message)
	case msg.ButtonsMessage != nil:
		return "buttons"
	case msg.ButtonsResponseMessage != nil:
		return "buttons_response"
	case msg.ListMessage != nil:
		return "list"
	case msg.ListResponseMessage != nil:
		return "list_response"
	case msg.InteractiveResponseMessage != nil:
		return "interactive_response"
	default:
		return ""
	}
}

func getButtonAttributes(msg *waE2E.Message) waBinary.Attrs {
	switch {
	case msg.ViewOnceMessage != nil:
		return getButtonAttributes(msg.ViewOnceMessage.Message)
	case msg.ViewOnceMessageV2 != nil:
		return getButtonAttributes(msg.ViewOnceMessageV2.Message)
	case msg.EphemeralMessage != nil:
		return getButtonAttributes(msg.EphemeralMessage.Message)
	case msg.TemplateMessage != nil:
		return waBinary.Attrs{}
	case msg.ListMessage != nil:
		return waBinary.Attrs{
			"v":    "2",
			"type": strings.ToLower(waE2E.ListMessage_ListType_name[int32(msg.ListMessage.GetListType())]),
		}
	default:
		return waBinary.Attrs{}
	}
}

const RemoveReactionText = ""

func getEditAttribute(msg *waE2E.Message) types.EditAttribute {
	switch {
	case msg.EditedMessage != nil && msg.EditedMessage.Message != nil:
		return getEditAttribute(msg.EditedMessage.Message)
	case msg.ProtocolMessage != nil && msg.ProtocolMessage.GetKey() != nil:
		switch msg.ProtocolMessage.GetType() {
		case waE2E.ProtocolMessage_REVOKE:
			if msg.ProtocolMessage.GetKey().GetFromMe() {
				return types.EditAttributeSenderRevoke
			} else {
				return types.EditAttributeAdminRevoke
			}
		case waE2E.ProtocolMessage_MESSAGE_EDIT:
			if msg.ProtocolMessage.EditedMessage != nil {
				return types.EditAttributeMessageEdit
			}
		}
	case msg.ReactionMessage != nil && msg.ReactionMessage.GetText() == RemoveReactionText:
		return types.EditAttributeSenderRevoke
	case msg.KeepInChatMessage != nil && msg.KeepInChatMessage.GetKey().GetFromMe() && msg.KeepInChatMessage.GetKeepType() == waE2E.KeepType_UNDO_KEEP_FOR_ALL:
		return types.EditAttributeSenderRevoke
	}
	return types.EditAttributeEmpty
}

func (cli *Client) preparePeerMessageNode(
	ctx context.Context,
	to types.JID,
	id types.MessageID,
	message *waE2E.Message,
	timings *MessageDebugTimings,
) (*waBinary.Node, error) {
	attrs := waBinary.Attrs{
		"id":       id,
		"type":     "text",
		"category": "peer",
		"to":       to,
	}
	if message.GetProtocolMessage().GetType() == waE2E.ProtocolMessage_APP_STATE_SYNC_KEY_REQUEST {
		attrs["push_priority"] = "high"
	}
	start := time.Now()
	plaintext, err := proto.Marshal(message)
	timings.Marshal = time.Since(start)
	if err != nil {
		err = fmt.Errorf("failed to marshal message: %w", err)
		return nil, err
	}
	encryptionIdentity := to
	if to.Server == types.DefaultUserServer {
		encryptionIdentity, err = cli.Store.LIDs.GetLIDForPN(ctx, to)
		if err != nil {
			return nil, fmt.Errorf("failed to get LID for PN %s: %w", to, err)
		}
	}
	start = time.Now()
	encrypted, isPreKey, err := cli.encryptMessageForDevice(ctx, plaintext, encryptionIdentity, nil, nil, nil)
	timings.PeerEncrypt = time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt peer message for %s: %v", to, err)
	}
	content := []waBinary.Node{{
		Tag: "meta",
		Attrs: waBinary.Attrs{
			"appdata": "default",
		},
	}, *encrypted}
	if isPreKey && cli.MessengerConfig == nil {
		content = append(content, cli.makeDeviceIdentityNode())
	}
	return &waBinary.Node{
		Tag:     "message",
		Attrs:   attrs,
		Content: content,
	}, nil
}

func (cli *Client) getMessageContent(
	baseNode waBinary.Node,
	message *waE2E.Message,
	msgAttrs waBinary.Attrs,
	includeIdentity bool,
	extraParams nodeExtraParams,
) []waBinary.Node {
	content := []waBinary.Node{baseNode}
	if includeIdentity {
		content = append(content, cli.makeDeviceIdentityNode())
	}
	if msgAttrs["type"] == "poll" {
		pollType := "creation"
		if message.PollUpdateMessage != nil {
			pollType = "vote"
		}
		content = append(content, waBinary.Node{
			Tag: "meta",
			Attrs: waBinary.Attrs{
				"polltype": pollType,
			},
		})
	}

	if extraParams.botNode != nil {
		content = append(content, *extraParams.botNode)
	}
	if extraParams.metaNode != nil {
		content = append(content, *extraParams.metaNode)
	}
	if extraParams.additionalNodes != nil {
		content = append(content, *extraParams.additionalNodes...)
	}

	if buttonType := getButtonTypeFromMessage(message); buttonType != "" {
		content = append(content, waBinary.Node{
			Tag: "biz",
			Content: []waBinary.Node{{
				Tag:   buttonType,
				Attrs: getButtonAttributes(message),
			}},
		})
	}
	return content
}

func (cli *Client) prepareMessageNode(
	ctx context.Context,
	to types.JID,
	id types.MessageID,
	message *waE2E.Message,
	participants []types.JID,
	plaintext, dsmPlaintext []byte,
	timings *MessageDebugTimings,
	extraParams nodeExtraParams,
) (*waBinary.Node, []types.JID, error) {
	start := time.Now()
	allDevices, err := cli.GetUserDevices(ctx, participants)
	timings.GetDevices = time.Since(start)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get device list: %w", err)
	}

	if to.Server == types.GroupServer {
		allDevices = slices.DeleteFunc(allDevices, func(jid types.JID) bool {
			return jid.Server == types.HostedServer || jid.Server == types.HostedLIDServer
		})
	}

	msgType := getTypeFromMessage(message)
	encAttrs := waBinary.Attrs{}
	// Only include encMediaType for 1:1 messages (groups don't have a device-sent message plaintext)
	if encMediaType := getMediaTypeFromMessage(message); dsmPlaintext != nil && encMediaType != "" {
		encAttrs["mediatype"] = encMediaType
	}
	attrs := waBinary.Attrs{
		"id":   id,
		"type": msgType,
		"to":   to,
	}
	// TODO this is a very hacky hack for announcement group messages, why is it pn anyway?
	if extraParams.addressingMode != "" {
		attrs["addressing_mode"] = string(extraParams.addressingMode)
	}
	if editAttr := getEditAttribute(message); editAttr != "" {
		attrs["edit"] = string(editAttr)
		encAttrs["decrypt-fail"] = string(events.DecryptFailHide)
	}
	if msgType == "reaction" || message.GetPollUpdateMessage() != nil {
		encAttrs["decrypt-fail"] = string(events.DecryptFailHide)
	}

	start = time.Now()
	participantNodes, includeIdentity, err := cli.encryptMessageForDevices(
		ctx, allDevices, id, plaintext, dsmPlaintext, encAttrs,
	)
	timings.PeerEncrypt = time.Since(start)
	if err != nil {
		return nil, nil, err
	}
	participantNode := waBinary.Node{
		Tag:     "participants",
		Content: participantNodes,
	}
	return &waBinary.Node{
		Tag:   "message",
		Attrs: attrs,
		Content: cli.getMessageContent(
			participantNode, message, attrs, includeIdentity, extraParams,
		),
	}, allDevices, nil
}

func marshalMessage(to types.JID, message *waE2E.Message) (plaintext, dsmPlaintext []byte, err error) {
	if message == nil && to.Server == types.NewsletterServer {
		return
	}
	plaintext, err = proto.Marshal(message)
	if err != nil {
		err = fmt.Errorf("failed to marshal message: %w", err)
		return
	}

	if to.Server != types.GroupServer && to.Server != types.NewsletterServer {
		dsmPlaintext, err = proto.Marshal(&waE2E.Message{
			DeviceSentMessage: &waE2E.DeviceSentMessage{
				DestinationJID: proto.String(to.String()),
				Message:        message,
			},
			MessageContextInfo: message.MessageContextInfo,
		})
		if err != nil {
			err = fmt.Errorf("failed to marshal message (for own devices): %w", err)
			return
		}
	}

	return
}

func (cli *Client) makeDeviceIdentityNode() waBinary.Node {
	deviceIdentity, err := proto.Marshal(cli.Store.Account)
	if err != nil {
		panic(fmt.Errorf("failed to marshal device identity: %w", err))
	}
	return waBinary.Node{
		Tag:     "device-identity",
		Content: deviceIdentity,
	}
}

func (cli *Client) encryptMessageForDevices(
	ctx context.Context,
	allDevices []types.JID,
	id string,
	msgPlaintext, dsmPlaintext []byte,
	encAttrs waBinary.Attrs,
) ([]waBinary.Node, bool, error) {
	ownJID := cli.getOwnID()
	ownLID := cli.getOwnLID()
	includeIdentity := false
	participantNodes := make([]waBinary.Node, 0, len(allDevices))

	var pnDevices []types.JID
	for _, jid := range allDevices {
		if jid.Server == types.DefaultUserServer {
			pnDevices = append(pnDevices, jid)
		}
	}
	lidMappings, err := cli.Store.LIDs.GetManyLIDsForPNs(ctx, pnDevices)
	if err != nil {
		return nil, false, fmt.Errorf("failed to fetch LID mappings: %w", err)
	}

	encryptionIdentities := make(map[types.JID]types.JID, len(allDevices))
	sessionAddressToJID := make(map[string]types.JID, len(allDevices))
	sessionAddresses := make([]string, 0, len(allDevices))
	for _, jid := range allDevices {
		encryptionIdentity := jid
		if jid.Server == types.DefaultUserServer {
			// TODO query LID from server for missing entries
			if lidForPN, ok := lidMappings[jid]; ok && !lidForPN.IsEmpty() {
				cli.migrateSessionStore(ctx, jid, lidForPN)
				encryptionIdentity = lidForPN
			}
		}
		encryptionIdentities[jid] = encryptionIdentity
		addr := encryptionIdentity.SignalAddress().String()
		sessionAddresses = append(sessionAddresses, addr)
		sessionAddressToJID[addr] = jid
	}

	existingSessions, ctx, err := cli.Store.WithCachedSessions(ctx, sessionAddresses)
	if err != nil {
		return nil, false, fmt.Errorf("failed to prefetch sessions: %w", err)
	}
	var retryDevices []types.JID
	for addr, exists := range existingSessions {
		if !exists {
			retryDevices = append(retryDevices, sessionAddressToJID[addr])
		}
	}
	bundles := cli.fetchPreKeysNoError(ctx, retryDevices)

	for _, jid := range allDevices {
		plaintext := msgPlaintext
		if (jid.User == ownJID.User || jid.User == ownLID.User) && dsmPlaintext != nil {
			if jid == ownJID || jid == ownLID {
				continue
			}
			plaintext = dsmPlaintext
		}
		encrypted, isPreKey, err := cli.encryptMessageForDeviceAndWrap(
			ctx, plaintext, jid, encryptionIdentities[jid], bundles[jid], encAttrs, existingSessions,
		)
		if err != nil {
			// TODO return these errors if it's a fatal one (like context cancellation or database)
			cli.Log.Warnf("Failed to encrypt %s for %s: %v", id, jid, err)
			if ctx.Err() != nil {
				return nil, false, err
			}
			continue
		}

		participantNodes = append(participantNodes, *encrypted)
		if isPreKey {
			includeIdentity = true
		}
	}
	err = cli.Store.PutCachedSessions(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("failed to save cached sessions: %w", err)
	}
	return participantNodes, includeIdentity, nil
}

func (cli *Client) encryptMessageForDeviceAndWrap(
	ctx context.Context,
	plaintext []byte,
	wireIdentity,
	encryptionIdentity types.JID,
	bundle *prekey.Bundle,
	encAttrs waBinary.Attrs,
	existingSessions map[string]bool,
) (*waBinary.Node, bool, error) {
	node, includeDeviceIdentity, err := cli.encryptMessageForDevice(
		ctx, plaintext, encryptionIdentity, bundle, encAttrs, existingSessions,
	)
	if err != nil {
		return nil, false, err
	}
	return &waBinary.Node{
		Tag:     "to",
		Attrs:   waBinary.Attrs{"jid": wireIdentity},
		Content: []waBinary.Node{*node},
	}, includeDeviceIdentity, nil
}

func copyAttrs(from, to waBinary.Attrs) {
	for k, v := range from {
		to[k] = v
	}
}

func (cli *Client) encryptMessageForDevice(
	ctx context.Context,
	plaintext []byte,
	to types.JID,
	bundle *prekey.Bundle,
	extraAttrs waBinary.Attrs,
	existingSessions map[string]bool,
) (*waBinary.Node, bool, error) {
	builder := session.NewBuilderFromSignal(cli.Store, to.SignalAddress(), pbSerializer)
	if bundle != nil {
		cli.Log.Debugf("Processing prekey bundle for %s", to)
		err := builder.ProcessBundle(ctx, bundle)
		if cli.AutoTrustIdentity && errors.Is(err, signalerror.ErrUntrustedIdentity) {
			cli.Log.Warnf("Got %v error while trying to process prekey bundle for %s, clearing stored identity and retrying", err, to)
			err = cli.clearUntrustedIdentity(ctx, to)
			if err != nil {
				return nil, false, fmt.Errorf("failed to clear untrusted identity: %w", err)
			}
			err = builder.ProcessBundle(ctx, bundle)
		}
		if err != nil {
			return nil, false, fmt.Errorf("failed to process prekey bundle: %w", err)
		}
	} else {
		sessionExists, checked := existingSessions[to.SignalAddress().String()]
		if !checked {
			var err error
			sessionExists, err = cli.Store.ContainsSession(ctx, to.SignalAddress())
			if err != nil {
				return nil, false, err
			}
		}
		if !sessionExists {
			return nil, false, fmt.Errorf("%w with %s", ErrNoSession, to.SignalAddress().String())
		}
	}
	cipher := session.NewCipher(builder, to.SignalAddress())
	ciphertext, err := cipher.Encrypt(ctx, padMessage(plaintext))
	if err != nil {
		return nil, false, fmt.Errorf("cipher encryption failed: %w", err)
	}

	encAttrs := waBinary.Attrs{
		"v":    "2",
		"type": "msg",
	}
	if ciphertext.Type() == protocol.PREKEY_TYPE {
		encAttrs["type"] = "pkmsg"
	}
	copyAttrs(extraAttrs, encAttrs)

	includeDeviceIdentity := encAttrs["type"] == "pkmsg" && cli.MessengerConfig == nil
	return &waBinary.Node{
		Tag:     "enc",
		Attrs:   encAttrs,
		Content: ciphertext.Serialize(),
	}, includeDeviceIdentity, nil
}
