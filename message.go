// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"compress/zlib"
	"crypto/rand"
	"fmt"
	"io"
	"strconv"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/libsignal/groups"
	"go.mau.fi/libsignal/protocol"
	"go.mau.fi/libsignal/session"

	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
)

var pbSerializer = store.SignalProtobufSerializer

func (cli *Client) handleEncryptedMessage(node *waBinary.Node) {
	info, err := parseMessageInfo(node)
	info.IsFromMe = info.From.User == cli.Store.ID.User
	if err != nil {
		cli.Log.Warnf("Failed to parse message: %v", err)
	} else {
		cli.decryptMessages(info, node)
	}
}

// MessageInfo contains metadata about an incoming message.
type MessageInfo struct {
	From      waBinary.JID  // The user who sent the message.
	Chat      *waBinary.JID // For group and broadcast messages, the chat where the message was sent.
	Recipient *waBinary.JID // For direct messages sent by the user, the user who the message was sent to.
	IsFromMe  bool
	ID        string
	Type      string
	Notify    string
	Timestamp int64
	Category  string
}

// SourceString returns a log-friendly representation of who sent the message and where.
func (mi *MessageInfo) SourceString() string {
	if mi.Chat != nil {
		return fmt.Sprintf("%s in %s", mi.From, mi.Chat)
	} else if mi.Recipient != nil {
		return fmt.Sprintf("%s to %s", mi.From, mi.Recipient)
	} else {
		return mi.From.String()
	}
}

func parseMessageInfo(node *waBinary.Node) (*MessageInfo, error) {
	var info MessageInfo

	from, ok := node.Attrs["from"].(waBinary.JID)
	if !ok {
		return nil, fmt.Errorf("didn't find valid `from` attribute in message")
	}
	recipient, ok := node.Attrs["recipient"].(waBinary.JID)
	if ok {
		info.Recipient = &recipient
	}
	if from.Server == waBinary.GroupServer || from.Server == waBinary.BroadcastServer {
		info.Chat = &from
		info.From, ok = node.Attrs["participant"].(waBinary.JID)
		if !ok {
			return nil, fmt.Errorf("didn't find valid `participant` attribute in group message")
		}
	} else {
		info.From = from
	}

	info.ID, ok = node.Attrs["id"].(string)
	if !ok {
		return nil, fmt.Errorf("didn't find valid `id` attribute in message")
	}
	ts, ok := node.Attrs["t"].(string)
	if !ok {
		return nil, fmt.Errorf("didn't find valid `t` (timestamp) attribute in message")
	}
	var err error
	info.Timestamp, err = strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("didn't find valid `t` (timestamp) attribute in message: %w", err)
	}

	info.Notify, _ = node.Attrs["notify"].(string)
	info.Category, _ = node.Attrs["category"].(string)

	return &info, nil
}

func (cli *Client) decryptMessages(info *MessageInfo, node *waBinary.Node) {
	if len(node.GetChildrenByTag("unavailable")) == len(node.GetChildren()) {
		cli.Log.Warnf("Unavailable message %s from %s", info.ID, info.SourceString())
		go cli.sendRetryReceipt(node, true)
		return
	}
	children := node.GetChildren()
	cli.Log.Debugf("Decrypting %d messages from %s", len(children), info.SourceString())
	handled := false
	for _, child := range children {
		if child.Tag != "enc" {
			continue
		}
		encType, ok := child.Attrs["type"].(string)
		if !ok {
			continue
		}
		var decrypted []byte
		var err error
		if encType == "pkmsg" || encType == "msg" {
			decrypted, err = cli.decryptDM(&child, info.From, encType == "pkmsg")
		} else if info.Chat != nil && encType == "skmsg" {
			decrypted, err = cli.decryptGroupMsg(&child, info.From, *info.Chat)
		} else {
			cli.Log.Warnf("Unhandled encrypted message (type %s) from %s", encType, info.SourceString())
			continue
		}
		if err != nil {
			cli.Log.Warnf("Error decrypting message from %s: %v", info.SourceString(), err)
			go cli.sendRetryReceipt(node, false)
			return
		}

		var msg waProto.Message
		err = proto.Unmarshal(decrypted, &msg)
		if err != nil {
			cli.Log.Warnf("Error unmarshaling decrypted message from %s: %v", info.SourceString(), err)
			continue
		}

		cli.handleDecryptedMessage(info, &msg)
		handled = true
	}
	if handled {
		go func() {
			cli.sendMessageReceipt(info)
			cli.sendAck(node)
		}()
	}
}

func (cli *Client) decryptDM(child *waBinary.Node, from waBinary.JID, isPreKey bool) ([]byte, error) {
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

func (cli *Client) decryptGroupMsg(child *waBinary.Node, from waBinary.JID, chat waBinary.JID) ([]byte, error) {
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
	if checkPadding && !isValidPadding(plaintext) {
		return nil, fmt.Errorf("plaintext doesn't have expected padding")
	}
	return plaintext[:len(plaintext)-int(plaintext[len(plaintext)-1])], nil
}

func padMessage(plaintext []byte) []byte {
	var pad [1]byte
	_, err := rand.Read(pad[:])
	if err != nil {
		panic(err)
	}
	pad[0] &= 0xf
	if pad[0] == 0 {
		pad[0] = 0xf
	}
	plaintext = append(plaintext, bytes.Repeat(pad[:], int(pad[0]))...)
	return plaintext
}

func (cli *Client) handleSenderKeyDistributionMessage(chat, from waBinary.JID, rawSKDMsg *waProto.SenderKeyDistributionMessage) {
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
		cli.Log.Debugf("Received history sync")
		cli.dispatchEvent(&HistorySyncEvent{
			Data: &historySync,
		})
	}
}

func (cli *Client) handleAppStateSyncKeyShare(keys *waProto.AppStateSyncKeyShare) {
	for _, key := range keys.GetKeys() {
		marshaledFingerprint, err := proto.Marshal(key.GetKeyData().GetFingerprint())
		if err != nil {
			cli.Log.Errorf("Failed to marshal fingerprint of app state sync key %X", key.GetKeyId().GetKeyId())
			continue
		}
		err = cli.Store.AppStateKeys.PutAppStateSyncKey(key.GetKeyId().GetKeyId(), store.AppStateSyncKey{
			Data:        key.GetKeyData().GetKeyData(),
			Fingerprint: marshaledFingerprint,
			Timestamp:   key.GetKeyData().GetTimestamp(),
		})
		if err != nil {
			cli.Log.Errorf("Failed to store app state sync key %X", key.GetKeyId().GetKeyId())
			continue
		}
		cli.Log.Debugf("Received app state sync key %X", key.GetKeyId().GetKeyId())
	}
}

func (cli *Client) handleProtocolMessage(info *MessageInfo, msg *waProto.Message) {
	protoMsg := msg.GetProtocolMessage()

	if protoMsg.GetHistorySyncNotification() != nil && info.IsFromMe {
		cli.handleHistorySyncNotification(protoMsg.HistorySyncNotification)
		cli.sendProtocolMessageReceipt(info.ID, "hist_sync")
	}

	if protoMsg.GetAppStateSyncKeyShare() != nil && info.IsFromMe {
		cli.handleAppStateSyncKeyShare(protoMsg.AppStateSyncKeyShare)
	}

	if info.Category == "peer" {
		cli.sendProtocolMessageReceipt(info.ID, "peer_msg")
	}
}

func (cli *Client) handleDecryptedMessage(info *MessageInfo, msg *waProto.Message) {
	cli.Log.Infof("Received message: %+v -- info: %+v\n", msg, info)

	evt := &MessageEvent{Info: info, RawMessage: msg}

	// First unwrap device sent messages
	if msg.GetDeviceSentMessage().GetMessage() != nil {
		msg = msg.GetDeviceSentMessage().GetMessage()
		evt.DeviceSentMeta = &DeviceSentMeta{
			DestinationJID: msg.GetDeviceSentMessage().GetDestinationJid(),
			Phash:          msg.GetDeviceSentMessage().GetPhash(),
		}
	}

	if msg.GetSenderKeyDistributionMessage() != nil {
		if info.Chat == nil {
			cli.Log.Warnf("Got sender key distribution message in unknown chat from", info.From)
		} else {
			cli.handleSenderKeyDistributionMessage(*info.Chat, info.From, msg.SenderKeyDistributionMessage)
		}
	}
	if msg.GetProtocolMessage() != nil {
		go cli.handleProtocolMessage(info, msg)
	}

	// Unwrap ephemeral and view-once messages
	// Hopefully sender key distribution messages and protocol messages can't be inside ephemeral messages
	if msg.GetEphemeralMessage().GetMessage() != nil {
		msg = msg.GetEphemeralMessage().GetMessage()
		evt.IsEphemeral = true
	}
	if msg.GetViewOnceMessage().GetMessage() != nil {
		msg = msg.GetViewOnceMessage().GetMessage()
		evt.IsViewOnce = true
	}
	evt.Message = msg

	cli.dispatchEvent(evt)
}

func (cli *Client) sendProtocolMessageReceipt(id, msgType string) {
	if len(id) == 0 {
		return
	}
	err := cli.sendNode(waBinary.Node{
		Tag: "receipt",
		Attrs: waBinary.Attrs{
			"id":   id,
			"type": msgType,
			"to":   waBinary.NewJID(cli.Store.ID.User, waBinary.LegacyUserServer),
		},
		Content: nil,
	})
	if err != nil {
		cli.Log.Warnf("Failed to send acknowledgement for protocol message %s: %v", id, err)
	}
}
