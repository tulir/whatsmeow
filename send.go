// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
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
//   msgID := whatsmeow.GenerateMessageID()
//   cli.SendMessage(targetJID, msgID, &waProto.Message{...})
func GenerateMessageID() types.MessageID {
	id := make([]byte, 8)
	_, err := rand.Read(id)
	if err != nil {
		// Out of entropy
		panic(err)
	}
	return "3EB0" + strings.ToUpper(hex.EncodeToString(id))
}

// SendMessage sends the given message.
//
// If the message ID is not provided, a random message ID will be generated.
//
// This method will wait for the server to acknowledge the message before returning.
// The return value is the timestamp of the message from the server.
//
// The message itself can contain anything you want (within the protobuf schema).
// e.g. for a simple text message, use the Conversation field:
//   cli.SendMessage(targetJID, "", &waProto.Message{
//       Conversation: proto.String("Hello, World!"),
//   })
//
// Things like replies, mentioning users and the "forwarded" flag are stored in ContextInfo,
// which can be put in ExtendedTextMessage and any of the media message types.
//
// For uploading and sending media/attachments, see the Upload method.
//
// For other message types, you'll have to figure it out yourself. Looking at the protobuf schema
// in binary/proto/def.proto may be useful to find out all the allowed fields.
func (cli *Client) SendMessage(to types.JID, id types.MessageID, message *waProto.Message) (time.Time, error) {
	isPeerMessage := to.User == cli.Store.ID.User
	if to.AD && !isPeerMessage {
		return time.Time{}, ErrRecipientADJID
	}

	if len(id) == 0 {
		id = GenerateMessageID()
	}

	// Sending multiple messages at a time can cause weird issues and makes it harder to retry safely
	cli.messageSendLock.Lock()
	defer cli.messageSendLock.Unlock()

	respChan := cli.waitResponse(id)
	// Peer message retries aren't implemented yet
	if !isPeerMessage {
		cli.addRecentMessage(to, id, message)
	}
	var err error
	var phash string
	var data []byte
	switch to.Server {
	case types.GroupServer, types.BroadcastServer:
		phash, data, err = cli.sendGroup(to, id, message)
	case types.DefaultUserServer:
		if isPeerMessage {
			data, err = cli.sendPeerMessage(to, id, message)
		} else {
			data, err = cli.sendDM(to, id, message)
		}
	default:
		err = fmt.Errorf("%w %s", ErrUnknownServer, to.Server)
	}
	if err != nil {
		cli.cancelResponse(id, respChan)
		return time.Time{}, err
	}
	resp := <-respChan
	if isDisconnectNode(resp) {
		resp, err = cli.retryFrame("message send", id, data, resp, nil, 0)
		if err != nil {
			return time.Time{}, err
		}
	}
	ag := resp.AttrGetter()
	ts := ag.UnixTime("t")
	expectedPHash := ag.OptionalString("phash")
	if len(expectedPHash) > 0 && phash != expectedPHash {
		cli.Log.Warnf("Server returned different participant list hash when sending to %s. Some devices may not have received the message.", to)
		// TODO also invalidate device list caches
		cli.groupParticipantsCacheLock.Lock()
		delete(cli.groupParticipantsCache, to)
		cli.groupParticipantsCacheLock.Unlock()
	}
	return ts, nil
}

// RevokeMessage deletes the given message from everyone in the chat.
// You can only revoke your own messages, and if the message is too old, then other users will ignore the deletion.
//
// This method will wait for the server to acknowledge the revocation message before returning.
// The return value is the timestamp of the message from the server.
func (cli *Client) RevokeMessage(chat types.JID, id types.MessageID) (time.Time, error) {
	return cli.SendMessage(chat, cli.generateRequestID(), &waProto.Message{
		ProtocolMessage: &waProto.ProtocolMessage{
			Type: waProto.ProtocolMessage_REVOKE.Enum(),
			Key: &waProto.MessageKey{
				FromMe:    proto.Bool(true),
				Id:        proto.String(id),
				RemoteJid: proto.String(chat.String()),
			},
		},
	})
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
		_, err = cli.SendMessage(chat, "", &waProto.Message{
			ProtocolMessage: &waProto.ProtocolMessage{
				Type:                waProto.ProtocolMessage_EPHEMERAL_SETTING.Enum(),
				EphemeralExpiration: proto.Uint32(uint32(timer.Seconds())),
			},
		})
	case types.GroupServer:
		if timer == 0 {
			_, err = cli.sendGroupIQ(iqSet, chat, waBinary.Node{Tag: "not_ephemeral"})
		} else {
			_, err = cli.sendGroupIQ(iqSet, chat, waBinary.Node{
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

func (cli *Client) sendGroup(to types.JID, id types.MessageID, message *waProto.Message) (string, []byte, error) {
	var participants []types.JID
	var err error
	if to.Server == types.GroupServer {
		participants, err = cli.getGroupMembers(to)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get group members: %w", err)
		}
	} else {
		participants, err = cli.getBroadcastListParticipants(to)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get broadcast list members: %w", err)
		}
	}

	plaintext, _, err := marshalMessage(to, message)
	if err != nil {
		return "", nil, err
	}

	builder := groups.NewGroupSessionBuilder(cli.Store, pbSerializer)
	senderKeyName := protocol.NewSenderKeyName(to.String(), cli.Store.ID.SignalAddress())
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

	node, allDevices, err := cli.prepareMessageNode(to, id, message, participants, skdPlaintext, nil)
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

	data, err := cli.sendNodeAndGetData(*node)
	if err != nil {
		return "", nil, fmt.Errorf("failed to send message node: %w", err)
	}
	return phash, data, nil
}

func (cli *Client) sendPeerMessage(to types.JID, id types.MessageID, message *waProto.Message) ([]byte, error) {
	node, err := cli.preparePeerMessageNode(to, id, message)
	if err != nil {
		return nil, err
	}
	data, err := cli.sendNodeAndGetData(*node)
	if err != nil {
		return nil, fmt.Errorf("failed to send message node: %w", err)
	}
	return data, nil
}

func (cli *Client) sendDM(to types.JID, id types.MessageID, message *waProto.Message) ([]byte, error) {
	messagePlaintext, deviceSentMessagePlaintext, err := marshalMessage(to, message)
	if err != nil {
		return nil, err
	}

	node, _, err := cli.prepareMessageNode(to, id, message, []types.JID{to, cli.Store.ID.ToNonAD()}, messagePlaintext, deviceSentMessagePlaintext)
	if err != nil {
		return nil, err
	}
	data, err := cli.sendNodeAndGetData(*node)
	if err != nil {
		return nil, fmt.Errorf("failed to send message node: %w", err)
	}
	return data, nil
}

func getTypeFromMessage(msg *waProto.Message) string {
	switch {
	case msg.ViewOnceMessage != nil:
		return getTypeFromMessage(msg.ViewOnceMessage.Message)
	case msg.EphemeralMessage != nil:
		return getTypeFromMessage(msg.EphemeralMessage.Message)
	case msg.ReactionMessage != nil:
		return "reaction"
	case msg.Conversation != nil, msg.ExtendedTextMessage != nil, msg.ProtocolMessage != nil:
		return "text"
	//TODO this requires setting mediatype in the enc nodes
	//case msg.ImageMessage != nil, msg.DocumentMessage != nil, msg.AudioMessage != nil, msg.VideoMessage != nil:
	//	return "media"
	default:
		return "text"
	}
}

func getEditAttribute(msg *waProto.Message) string {
	if msg.ProtocolMessage != nil && msg.GetProtocolMessage().GetType() == waProto.ProtocolMessage_REVOKE && msg.GetProtocolMessage().GetKey() != nil {
		if msg.GetProtocolMessage().GetKey().GetFromMe() {
			return "7"
		} else {
			return "8"
		}
	} else if msg.ReactionMessage != nil && msg.ReactionMessage.GetText() == "" {
		return "7"
	}
	return ""
}

func (cli *Client) preparePeerMessageNode(to types.JID, id types.MessageID, message *waProto.Message) (*waBinary.Node, error) {
	attrs := waBinary.Attrs{
		"id":       id,
		"type":     "text",
		"category": "peer",
		"to":       to,
	}
	if message.GetProtocolMessage().GetType() == waProto.ProtocolMessage_APP_STATE_SYNC_KEY_REQUEST {
		attrs["push_priority"] = "high"
	}
	plaintext, err := proto.Marshal(message)
	if err != nil {
		err = fmt.Errorf("failed to marshal message: %w", err)
		return nil, err
	}
	encrypted, isPreKey, err := cli.encryptMessageForDevice(plaintext, to, nil)
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

func (cli *Client) prepareMessageNode(to types.JID, id types.MessageID, message *waProto.Message, participants []types.JID, plaintext, dsmPlaintext []byte) (*waBinary.Node, []types.JID, error) {
	allDevices, err := cli.GetUserDevices(participants)
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

	participantNodes, includeIdentity := cli.encryptMessageForDevices(allDevices, id, plaintext, dsmPlaintext)
	content := []waBinary.Node{{
		Tag:     "participants",
		Content: participantNodes,
	}}
	if includeIdentity {
		content = append(content, cli.makeDeviceIdentityNode())
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

func (cli *Client) encryptMessageForDevices(allDevices []types.JID, id string, msgPlaintext, dsmPlaintext []byte) ([]waBinary.Node, bool) {
	includeIdentity := false
	participantNodes := make([]waBinary.Node, 0, len(allDevices))
	var retryDevices []types.JID
	for _, jid := range allDevices {
		plaintext := msgPlaintext
		if jid.User == cli.Store.ID.User && dsmPlaintext != nil {
			if jid == *cli.Store.ID {
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
		bundles, err := cli.fetchPreKeys(retryDevices)
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
				if jid.User == cli.Store.ID.User && dsmPlaintext != nil {
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
