// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mau.fi/libsignal/signalerror"
	"google.golang.org/protobuf/proto"

	"go.mau.fi/libsignal/groups"
	"go.mau.fi/libsignal/keys/prekey"
	"go.mau.fi/libsignal/protocol"
	"go.mau.fi/libsignal/session"

	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
)

// GenerateMessageID generates a random string that can be used as a message ID on WhatsApp.
//
//	msgID := whatsmeow.GenerateMessageID()
//	cli.SendMessage(context.Background(), targetJID, &waProto.Message{...}, whatsmeow.SendRequestExtra{ID: msgID})
func GenerateMessageID() types.MessageID {
	id := make([]byte, 8)
	_, err := rand.Read(id)
	if err != nil {
		// Out of entropy
		panic(err)
	}
	return "3EB0" + strings.ToUpper(hex.EncodeToString(id))
}

type MessageDebugTimings struct {
	Queue time.Duration

	Marshal         time.Duration
	GetParticipants time.Duration
	GetDevices      time.Duration
	GroupEncrypt    time.Duration
	PeerEncrypt     time.Duration

	Send  time.Duration
	Resp  time.Duration
	Retry time.Duration
}

type SendResponse struct {
	// The message timestamp returned by the server
	Timestamp time.Time

	// The ID of the sent message
	ID types.MessageID

	// Message handling duration, used for debugging
	DebugTimings MessageDebugTimings
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
	// Should the message be sent as a peer message (protocol messages to your own devices, e.g. app state key requests)
	Peer bool
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
//	cli.SendMessage(context.Background(), targetJID, &waProto.Message{
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
func (cli *Client) SendMessage(ctx context.Context, to types.JID, message *waProto.Message, extra ...SendRequestExtra) (resp SendResponse, err error) {
	var req SendRequestExtra
	if len(extra) > 1 {
		err = errors.New("only one extra parameter may be provided to SendMessage")
		return
	} else if len(extra) == 1 {
		req = extra[0]
	}
	if to.AD && !req.Peer {
		err = ErrRecipientADJID
		return
	}
	ownID := cli.getOwnID()
	if ownID.IsEmpty() {
		err = ErrNotLoggedIn
		return
	}

	if len(req.ID) == 0 {
		req.ID = GenerateMessageID()
	}
	resp.ID = req.ID

	start := time.Now()
	// Sending multiple messages at a time can cause weird issues and makes it harder to retry safely
	cli.messageSendLock.Lock()
	resp.DebugTimings.Queue = time.Since(start)
	defer cli.messageSendLock.Unlock()

	respChan := cli.waitResponse(req.ID)
	// Peer message retries aren't implemented yet
	if !req.Peer {
		cli.addRecentMessage(to, req.ID, message)
	}
	if message.GetMessageContextInfo().GetMessageSecret() != nil {
		err = cli.Store.MsgSecrets.PutMessageSecret(to, ownID, req.ID, message.GetMessageContextInfo().GetMessageSecret())
		if err != nil {
			cli.Log.Warnf("Failed to store message secret key for outgoing message %s: %v", req.ID, err)
		} else {
			cli.Log.Debugf("Stored message secret key for outgoing message %s", req.ID)
		}
	}
	var phash string
	var data []byte
	switch to.Server {
	case types.GroupServer, types.BroadcastServer:
		phash, data, err = cli.sendGroup(ctx, to, ownID, req.ID, message, &resp.DebugTimings)
	case types.DefaultUserServer:
		if req.Peer {
			data, err = cli.sendPeerMessage(to, req.ID, message, &resp.DebugTimings)
		} else {
			data, err = cli.sendDM(ctx, to, ownID, req.ID, message, &resp.DebugTimings)
		}
	default:
		err = fmt.Errorf("%w %s", ErrUnknownServer, to.Server)
	}
	start = time.Now()
	if err != nil {
		cli.cancelResponse(req.ID, respChan)
		return
	}
	var respNode *waBinary.Node
	select {
	case respNode = <-respChan:
	case <-ctx.Done():
		err = ctx.Err()
		return
	}
	resp.DebugTimings.Resp = time.Since(start)
	if isDisconnectNode(respNode) {
		start = time.Now()
		respNode, err = cli.retryFrame("message send", req.ID, data, respNode, ctx, 0)
		resp.DebugTimings.Retry = time.Since(start)
		if err != nil {
			return
		}
	}
	ag := respNode.AttrGetter()
	resp.Timestamp = ag.UnixTime("t")
	if errorCode := ag.Int("error"); errorCode != 0 {
		err = fmt.Errorf("%w %d", ErrServerReturnedError, errorCode)
	}
	expectedPHash := ag.OptionalString("phash")
	if len(expectedPHash) > 0 && phash != expectedPHash {
		cli.Log.Warnf("Server returned different participant list hash when sending to %s. Some devices may not have received the message.", to)
		// TODO also invalidate device list caches
		cli.groupParticipantsCacheLock.Lock()
		delete(cli.groupParticipantsCache, to)
		cli.groupParticipantsCacheLock.Unlock()
	}
	return
}

// RevokeMessage deletes the given message from everyone in the chat.
//
// This method will wait for the server to acknowledge the revocation message before returning.
// The return value is the timestamp of the message from the server.
//
// Deprecated: This method is deprecated in favor of BuildRevoke
func (cli *Client) RevokeMessage(chat types.JID, id types.MessageID) (SendResponse, error) {
	return cli.SendMessage(context.TODO(), chat, cli.BuildRevoke(chat, types.EmptyJID, id))
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
func (cli *Client) BuildRevoke(chat, sender types.JID, id types.MessageID) *waProto.Message {
	key := &waProto.MessageKey{
		FromMe:    proto.Bool(true),
		Id:        proto.String(id),
		RemoteJid: proto.String(chat.String()),
	}
	if !sender.IsEmpty() && sender.User != cli.getOwnID().User {
		key.FromMe = proto.Bool(false)
		if chat.Server != types.DefaultUserServer {
			key.Participant = proto.String(sender.ToNonAD().String())
		}
	}
	return &waProto.Message{
		ProtocolMessage: &waProto.ProtocolMessage{
			Type: waProto.ProtocolMessage_REVOKE.Enum(),
			Key:  key,
		},
	}
}

// BuildEdit builds a message edit message using the given variables.
// The built message can be sent normally using Client.SendMessage.
//
//	resp, err := cli.SendMessage(context.Background(), chat, cli.BuildEdit(chat, originalMessageID, &waProto.Message{
//		Conversation: proto.String("edited message"),
//	})
func (cli *Client) BuildEdit(chat types.JID, id types.MessageID, newContent *waProto.Message) *waProto.Message {
	return &waProto.Message{
		EditedMessage: &waProto.FutureProofMessage{
			Message: &waProto.Message{
				ProtocolMessage: &waProto.ProtocolMessage{
					Key: &waProto.MessageKey{
						FromMe:    proto.Bool(true),
						Id:        proto.String(id),
						RemoteJid: proto.String(chat.String()),
					},
					Type:          waProto.ProtocolMessage_MESSAGE_EDIT.Enum(),
					EditedMessage: newContent,
					TimestampMs:   proto.Int64(time.Now().UnixMilli()),
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
func (cli *Client) SetDisappearingTimer(chat types.JID, timer time.Duration) (err error) {
	switch chat.Server {
	case types.DefaultUserServer:
		_, err = cli.SendMessage(context.TODO(), chat, &waProto.Message{
			ProtocolMessage: &waProto.ProtocolMessage{
				Type:                waProto.ProtocolMessage_EPHEMERAL_SETTING.Enum(),
				EphemeralExpiration: proto.Uint32(uint32(timer.Seconds())),
			},
		})
	case types.GroupServer:
		if timer == 0 {
			_, err = cli.sendGroupIQ(context.TODO(), iqSet, chat, waBinary.Node{Tag: "not_ephemeral"})
		} else {
			_, err = cli.sendGroupIQ(context.TODO(), iqSet, chat, waBinary.Node{
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
		participantsStrings[i] = part.String()
	}

	sort.Strings(participantsStrings)
	hash := sha256.Sum256([]byte(strings.Join(participantsStrings, "")))
	return fmt.Sprintf("2:%s", base64.RawStdEncoding.EncodeToString(hash[:6]))
}

func (cli *Client) sendGroup(ctx context.Context, to, ownID types.JID, id types.MessageID, message *waProto.Message, timings *MessageDebugTimings) (string, []byte, error) {
	var participants []types.JID
	var err error
	start := time.Now()
	if to.Server == types.GroupServer {
		participants, err = cli.getGroupMembers(ctx, to)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get group members: %w", err)
		}
	} else {
		// TODO use context
		participants, err = cli.getBroadcastListParticipants(to)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get broadcast list members: %w", err)
		}
	}
	timings.GetParticipants = time.Since(start)
	start = time.Now()
	plaintext, _, err := marshalMessage(to, message)
	timings.Marshal = time.Since(start)
	if err != nil {
		return "", nil, err
	}

	start = time.Now()
	builder := groups.NewGroupSessionBuilder(cli.Store, pbSerializer)
	senderKeyName := protocol.NewSenderKeyName(to.String(), ownID.SignalAddress())
	signalSKDMessage, err := builder.Create(senderKeyName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create sender key distribution message to send %s to %s: %w", id, to, err)
	}
	skdMessage := &waProto.Message{
		SenderKeyDistributionMessage: &waProto.SenderKeyDistributionMessage{
			GroupId:                             proto.String(to.String()),
			AxolotlSenderKeyDistributionMessage: signalSKDMessage.Serialize(),
		},
	}
	skdPlaintext, err := proto.Marshal(skdMessage)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal sender key distribution message to send %s to %s: %w", id, to, err)
	}

	cipher := groups.NewGroupCipher(builder, senderKeyName, cli.Store)
	encrypted, err := cipher.Encrypt(padMessage(plaintext))
	if err != nil {
		return "", nil, fmt.Errorf("failed to encrypt group message to send %s to %s: %w", id, to, err)
	}
	ciphertext := encrypted.SignedSerialize()
	timings.GroupEncrypt = time.Since(start)

	node, allDevices, err := cli.prepareMessageNode(ctx, to, ownID, id, message, participants, skdPlaintext, nil, timings)
	if err != nil {
		return "", nil, err
	}

	phash := participantListHashV2(allDevices)
	node.Attrs["phash"] = phash
	node.Content = append(node.GetChildren(), waBinary.Node{
		Tag:     "enc",
		Content: ciphertext,
		Attrs:   waBinary.Attrs{"v": "2", "type": "skmsg"},
	})

	start = time.Now()
	data, err := cli.sendNodeAndGetData(*node)
	timings.Send = time.Since(start)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send message node: %w", err)
	}
	return phash, data, nil
}

func (cli *Client) sendPeerMessage(to types.JID, id types.MessageID, message *waProto.Message, timings *MessageDebugTimings) ([]byte, error) {
	node, err := cli.preparePeerMessageNode(to, id, message, timings)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	data, err := cli.sendNodeAndGetData(*node)
	timings.Send = time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("failed to send message node: %w", err)
	}
	return data, nil
}

func (cli *Client) sendDM(ctx context.Context, to, ownID types.JID, id types.MessageID, message *waProto.Message, timings *MessageDebugTimings) ([]byte, error) {
	start := time.Now()
	messagePlaintext, deviceSentMessagePlaintext, err := marshalMessage(to, message)
	timings.Marshal = time.Since(start)
	if err != nil {
		return nil, err
	}

	node, _, err := cli.prepareMessageNode(ctx, to, ownID, id, message, []types.JID{to, ownID.ToNonAD()}, messagePlaintext, deviceSentMessagePlaintext, timings)
	if err != nil {
		return nil, err
	}
	start = time.Now()
	data, err := cli.sendNodeAndGetData(*node)
	timings.Send = time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("failed to send message node: %w", err)
	}
	return data, nil
}

func getTypeFromMessage(msg *waProto.Message) string {
	switch {
	case msg.ViewOnceMessage != nil:
		return getTypeFromMessage(msg.ViewOnceMessage.Message)
	case msg.ViewOnceMessageV2 != nil:
		return getTypeFromMessage(msg.ViewOnceMessageV2.Message)
	case msg.EphemeralMessage != nil:
		return getTypeFromMessage(msg.EphemeralMessage.Message)
	case msg.DocumentWithCaptionMessage != nil:
		return getTypeFromMessage(msg.DocumentWithCaptionMessage.Message)
	case msg.ReactionMessage != nil:
		return "reaction"
	case msg.PollCreationMessage != nil, msg.PollUpdateMessage != nil:
		return "poll"
	case msg.Conversation != nil, msg.ExtendedTextMessage != nil, msg.ProtocolMessage != nil:
		return "text"
	//TODO this requires setting mediatype in the enc nodes
	//case msg.ImageMessage != nil, msg.DocumentMessage != nil, msg.AudioMessage != nil, msg.VideoMessage != nil:
	//	return "media"
	default:
		return "text"
	}
}

const (
	EditAttributeEmpty        = ""
	EditAttributeMessageEdit  = "1"
	EditAttributeSenderRevoke = "7"
	EditAttributeAdminRevoke  = "8"
)

const RemoveReactionText = ""

func getEditAttribute(msg *waProto.Message) string {
	switch {
	case msg.ProtocolMessage != nil && msg.ProtocolMessage.GetKey() != nil:
		switch msg.ProtocolMessage.GetType() {
		case waProto.ProtocolMessage_REVOKE:
			if msg.ProtocolMessage.GetKey().GetFromMe() {
				return EditAttributeSenderRevoke
			} else {
				return EditAttributeAdminRevoke
			}
		case waProto.ProtocolMessage_MESSAGE_EDIT:
			if msg.EditedMessage != nil {
				return EditAttributeMessageEdit
			}
		}
	case msg.ReactionMessage != nil && msg.ReactionMessage.GetText() == RemoveReactionText:
		return EditAttributeSenderRevoke
	case msg.KeepInChatMessage != nil && msg.KeepInChatMessage.GetKey().GetFromMe() && msg.KeepInChatMessage.GetKeepType() == waProto.KeepType_UNDO_KEEP_FOR_ALL:
		return EditAttributeSenderRevoke
	}
	return EditAttributeEmpty
}

func (cli *Client) preparePeerMessageNode(to types.JID, id types.MessageID, message *waProto.Message, timings *MessageDebugTimings) (*waBinary.Node, error) {
	attrs := waBinary.Attrs{
		"id":       id,
		"type":     "text",
		"category": "peer",
		"to":       to,
	}
	if message.GetProtocolMessage().GetType() == waProto.ProtocolMessage_APP_STATE_SYNC_KEY_REQUEST {
		attrs["push_priority"] = "high"
	}
	start := time.Now()
	plaintext, err := proto.Marshal(message)
	timings.Marshal = time.Since(start)
	if err != nil {
		err = fmt.Errorf("failed to marshal message: %w", err)
		return nil, err
	}
	start = time.Now()
	encrypted, isPreKey, err := cli.encryptMessageForDevice(plaintext, to, nil)
	timings.PeerEncrypt = time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt peer message for %s: %v", to, err)
	}
	content := []waBinary.Node{*encrypted}
	if isPreKey {
		content = append(content, cli.makeDeviceIdentityNode())
	}
	return &waBinary.Node{
		Tag:     "message",
		Attrs:   attrs,
		Content: content,
	}, nil
}

func (cli *Client) prepareMessageNode(ctx context.Context, to, ownID types.JID, id types.MessageID, message *waProto.Message, participants []types.JID, plaintext, dsmPlaintext []byte, timings *MessageDebugTimings) (*waBinary.Node, []types.JID, error) {
	start := time.Now()
	allDevices, err := cli.GetUserDevicesContext(ctx, participants)
	timings.GetDevices = time.Since(start)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get device list: %w", err)
	}

	attrs := waBinary.Attrs{
		"id":   id,
		"type": getTypeFromMessage(message),
		"to":   to,
	}
	if editAttr := getEditAttribute(message); editAttr != "" {
		attrs["edit"] = editAttr
	}

	start = time.Now()
	participantNodes, includeIdentity := cli.encryptMessageForDevices(ctx, allDevices, ownID, id, plaintext, dsmPlaintext)
	timings.PeerEncrypt = time.Since(start)
	content := []waBinary.Node{{
		Tag:     "participants",
		Content: participantNodes,
	}}
	if includeIdentity {
		content = append(content, cli.makeDeviceIdentityNode())
	}
	if attrs["type"] == "poll" {
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
	return &waBinary.Node{
		Tag:     "message",
		Attrs:   attrs,
		Content: content,
	}, allDevices, nil
}

func marshalMessage(to types.JID, message *waProto.Message) (plaintext, dsmPlaintext []byte, err error) {
	plaintext, err = proto.Marshal(message)
	if err != nil {
		err = fmt.Errorf("failed to marshal message: %w", err)
		return
	}

	if to.Server != types.GroupServer {
		dsmPlaintext, err = proto.Marshal(&waProto.Message{
			DeviceSentMessage: &waProto.DeviceSentMessage{
				DestinationJid: proto.String(to.String()),
				Message:        message,
			},
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

func (cli *Client) encryptMessageForDevices(ctx context.Context, allDevices []types.JID, ownID types.JID, id string, msgPlaintext, dsmPlaintext []byte) ([]waBinary.Node, bool) {
	includeIdentity := false
	participantNodes := make([]waBinary.Node, 0, len(allDevices))
	var retryDevices []types.JID
	for _, jid := range allDevices {
		plaintext := msgPlaintext
		if jid.User == ownID.User && dsmPlaintext != nil {
			if jid == ownID {
				continue
			}
			plaintext = dsmPlaintext
		}
		encrypted, isPreKey, err := cli.encryptMessageForDeviceAndWrap(plaintext, jid, nil)
		if errors.Is(err, ErrNoSession) {
			retryDevices = append(retryDevices, jid)
			continue
		} else if err != nil {
			cli.Log.Warnf("Failed to encrypt %s for %s: %v", id, jid, err)
			continue
		}
		participantNodes = append(participantNodes, *encrypted)
		if isPreKey {
			includeIdentity = true
		}
	}
	if len(retryDevices) > 0 {
		bundles, err := cli.fetchPreKeys(ctx, retryDevices)
		if err != nil {
			cli.Log.Warnf("Failed to fetch prekeys for %v to retry encryption: %v", retryDevices, err)
		} else {
			for _, jid := range retryDevices {
				resp := bundles[jid]
				if resp.err != nil {
					cli.Log.Warnf("Failed to fetch prekey for %s: %v", jid, resp.err)
					continue
				}
				plaintext := msgPlaintext
				if jid.User == ownID.User && dsmPlaintext != nil {
					plaintext = dsmPlaintext
				}
				encrypted, isPreKey, err := cli.encryptMessageForDeviceAndWrap(plaintext, jid, resp.bundle)
				if err != nil {
					cli.Log.Warnf("Failed to encrypt %s for %s (retry): %v", id, jid, err)
					continue
				}
				participantNodes = append(participantNodes, *encrypted)
				if isPreKey {
					includeIdentity = true
				}
			}
		}
	}
	return participantNodes, includeIdentity
}

func (cli *Client) encryptMessageForDeviceAndWrap(plaintext []byte, to types.JID, bundle *prekey.Bundle) (*waBinary.Node, bool, error) {
	node, includeDeviceIdentity, err := cli.encryptMessageForDevice(plaintext, to, bundle)
	if err != nil {
		return nil, false, err
	}
	return &waBinary.Node{
		Tag:     "to",
		Attrs:   waBinary.Attrs{"jid": to},
		Content: []waBinary.Node{*node},
	}, includeDeviceIdentity, nil
}

func (cli *Client) encryptMessageForDevice(plaintext []byte, to types.JID, bundle *prekey.Bundle) (*waBinary.Node, bool, error) {
	builder := session.NewBuilderFromSignal(cli.Store, to.SignalAddress(), pbSerializer)
	if bundle != nil {
		cli.Log.Debugf("Processing prekey bundle for %s", to)
		err := builder.ProcessBundle(bundle)
		if cli.AutoTrustIdentity && errors.Is(err, signalerror.ErrUntrustedIdentity) {
			cli.Log.Warnf("Got %v error while trying to process prekey bundle for %s, clearing stored identity and retrying", err, to)
			cli.clearUntrustedIdentity(to)
			err = builder.ProcessBundle(bundle)
		}
		if err != nil {
			return nil, false, fmt.Errorf("failed to process prekey bundle: %w", err)
		}
	} else if !cli.Store.ContainsSession(to.SignalAddress()) {
		return nil, false, ErrNoSession
	}
	cipher := session.NewCipher(builder, to.SignalAddress())
	ciphertext, err := cipher.Encrypt(padMessage(plaintext))
	if err != nil {
		return nil, false, fmt.Errorf("cipher encryption failed: %w", err)
	}

	encType := "msg"
	if ciphertext.Type() == protocol.PREKEY_TYPE {
		encType = "pkmsg"
	}

	return &waBinary.Node{
		Tag: "enc",
		Attrs: waBinary.Attrs{
			"v":    "2",
			"type": encType,
		},
		Content: ciphertext.Serialize(),
	}, encType == "pkmsg", nil
}
