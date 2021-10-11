// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	"bytes"
	"compress/zlib"
	"crypto/rand"
	"fmt"
	"io"
	"strconv"

	"google.golang.org/protobuf/proto"

	"github.com/RadicalApp/libsignal-protocol-go/groups"
	"github.com/RadicalApp/libsignal-protocol-go/protocol"
	"github.com/RadicalApp/libsignal-protocol-go/serialize"
	"github.com/RadicalApp/libsignal-protocol-go/session"

	whatsapp "go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
)

var pbSerializer = serialize.NewProtoBufSerializer()

func (cli *Client) decryptDM(child *waBinary.Node, from waBinary.FullJID, isPreKey bool) ([]byte, error) {
	content, _ := child.Content.([]byte)

	builder := session.NewBuilderFromSignal(cli.Session, from.SignalAddress(), serialize.NewJSONSerializer())
	cipher := session.NewCipher(builder, from.SignalAddress())
	if isPreKey {
		preKeyMsg, err := protocol.NewPreKeySignalMessageFromBytes(content, pbSerializer.PreKeySignalMessage, pbSerializer.SignalMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to parse prekey message: %w", err)
		}
		plaintext, _, err := cipher.DecryptMessageReturnKey(preKeyMsg)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt prekey message: %w", err)
		}
		return unpadMessage(plaintext)
	} else {
		msg, err := protocol.NewSignalMessageFromBytes(content, pbSerializer.SignalMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to parse normal message: %w", err)
		}
		plaintext, err := cipher.Decrypt(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt normal message: %w", err)
		}
		return unpadMessage(plaintext)
	}
}

func (cli *Client) decryptGroupMsg(child *waBinary.Node, from waBinary.FullJID, chat waBinary.FullJID) ([]byte, error) {
	content, _ := child.Content.([]byte)

	senderKeyName := protocol.NewSenderKeyName(chat.String(), from.SignalAddress())
	builder := groups.NewGroupSessionBuilder(cli.Session, pbSerializer)
	cipher := groups.NewGroupCipher(builder, senderKeyName, cli.Session)
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

var CheckPadding = true

func unpadMessage(plaintext []byte) ([]byte, error) {
	if CheckPadding {
		lastByte := plaintext[len(plaintext)-1]
		expectedPadding := bytes.Repeat([]byte{lastByte}, int(lastByte))
		if !bytes.HasSuffix(plaintext, expectedPadding) {
			return nil, fmt.Errorf("plaintext doesn't have expected padding")
		}
	}
	return plaintext[:len(plaintext)-int(plaintext[len(plaintext)-1])], nil
}

func padMessage(plaintext []byte) []byte {
	var pad [1]byte
	_, err := rand.Read(pad[:])
	if err != nil {
		panic(err)
	}
	plaintext = append(plaintext, bytes.Repeat(pad[:], int(pad[0]&0xf))...)
	return plaintext
}

func (cli *Client) decryptMessages(info *MessageInfo, nodes []waBinary.Node) {
	cli.Log.Debugln("Decrypting", len(nodes), "messages from", info.FromString())
	for _, child := range nodes {
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
			if err != nil {
				cli.Log.Warnfln("Error decrypting message from %s: %v", info.FromString(), err)
				continue
			}
		} else if info.Chat != nil && encType == "skmsg" {
			decrypted, err = cli.decryptGroupMsg(&child, info.From, *info.Chat)
			if err != nil {
				cli.Log.Warnfln("Error decrypting message from %s: %v", info.FromString(), err)
				continue
			}
		} else {
			cli.Log.Warnfln("Unhandled encrypted message (type %s) from %s", encType, info.FromString())
			continue
		}

		var msg waProto.Message
		err = proto.Unmarshal(decrypted, &msg)
		if err != nil {
			cli.Log.Warnfln("Error unmarshaling decrypted message from %s: %v", info.FromString(), err)
		}

		cli.handleDecryptedMessage(info, &msg)

		cli.ackMessage(info)
	}
}

type MessageInfo struct {
	From waBinary.FullJID
	Chat *waBinary.FullJID
	ID   string
	Type string

	Notify    string
	Timestamp int64
	Category  string
}

func (mi *MessageInfo) FromString() string {
	if mi.Chat != nil {
		return fmt.Sprintf("%s in %s", mi.From, mi.Chat)
	} else {
		return mi.From.String()
	}
}

func parseMessageInfo(node *waBinary.Node) (*MessageInfo, error) {
	var info MessageInfo

	from, ok := node.Attrs["from"].(waBinary.FullJID)
	if !ok {
		return nil, fmt.Errorf("didn't find valid `from` attribute in message")
	}
	if from.Server == waBinary.DefaultGroupServer {
		info.Chat = &from
		info.From, ok = node.Attrs["participant"].(waBinary.FullJID)
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

func (cli *Client) handleMessage(node *waBinary.Node) bool {
	if node.Tag != "message" {
		return false
	}

	info, err := parseMessageInfo(node)
	if err != nil {
		cli.Log.Warnln("Failed to parse message:", err)
		return true
	}

	go cli.decryptMessages(info, node.GetChildren())

	return true
}

func (cli *Client) handleSenderKeyDistributionMessage(chat, from waBinary.FullJID, rawSKDMsg *waProto.SenderKeyDistributionMessage) {
	builder := groups.NewGroupSessionBuilder(cli.Session, pbSerializer)
	senderKeyName := protocol.NewSenderKeyName(chat.String(), from.SignalAddress())
	sdkMsg, err := protocol.NewSenderKeyDistributionMessageFromBytes(rawSKDMsg.AxolotlSenderKeyDistributionMessage, pbSerializer.SenderKeyDistributionMessage)
	if err != nil {
		cli.Log.Errorfln("Failed to parse sender key distribution message from %s for %s: %v", from, chat, err)
		return
	}
	builder.Process(senderKeyName, sdkMsg)
}

func (cli *Client) handleHistorySyncNotification(notif *waProto.HistorySyncNotification) {
	var historySync waProto.HistorySync
	if data, err := cli.downloadMedia(notif.GetDirectPath(), notif.FileEncSha256, notif.MediaKey, int(notif.GetFileLength()), whatsapp.MediaHistory, "md-msg-hist"); err != nil {
		cli.Log.Errorln("Failed to download history sync data:", err)
	} else if reader, err := zlib.NewReader(bytes.NewReader(data)); err != nil {
		cli.Log.Errorln("Failed to create zlib reader for history sync data:", err)
	} else if rawData, err := io.ReadAll(reader); err != nil {
		cli.Log.Errorln("Failed to decompress history sync data:", err)
	} else if err = proto.Unmarshal(rawData, &historySync); err != nil {
		cli.Log.Errorln("Failed to unmarshal history sync data:", err)
	} else {
		cli.Log.Debugln("Received history sync")
		fmt.Printf("%+v\n", &historySync)
	}
}

func (cli *Client) handleProtocolMessage(info *MessageInfo, msg *waProto.Message) {
	if msg.GetProtocolMessage().GetHistorySyncNotification() != nil {
		cli.handleHistorySyncNotification(msg.GetProtocolMessage().GetHistorySyncNotification())
		cli.sendProtocolMessageReceipt(info.ID, "hist_sync")
	}

	if info.Category == "peer" {
		cli.sendProtocolMessageReceipt(info.ID, "peer_msg")
	}
}

func (cli *Client) handleDecryptedMessage(info *MessageInfo, msg *waProto.Message) {
	if msg.GetSenderKeyDistributionMessage() != nil {
		cli.handleSenderKeyDistributionMessage(*info.Chat, info.From, msg.SenderKeyDistributionMessage)
	}
	if msg.GetProtocolMessage() != nil {
		cli.handleProtocolMessage(info, msg)
	}
	fmt.Printf("Received message: %+v -- info: %+v\n", msg, info)
}

func (cli *Client) sendProtocolMessageReceipt(id, msgType string) {
	if len(id) == 0 {
		return
	}
	err := cli.sendNode(waBinary.Node{
		Tag: "receipt",
		Attrs: map[string]interface{}{
			"id":   id,
			"type": msgType,
			"to":   waBinary.NewJID(cli.Session.ID.User, waBinary.GroupParticipantServer),
		},
		Content: nil,
	})
	if err != nil {
		cli.Log.Warnfln("Failed to send acknowledgement for protocol message %s: %v", id, err)
	}
}

func (cli *Client) ackMessage(info *MessageInfo) {
	attrs := map[string]interface{}{
		"class": "receipt",
		"id":    info.ID,
	}
	if info.Chat != nil {
		attrs["to"] = waBinary.NewJID(cli.Session.ID.User, waBinary.GroupParticipantServer)
		// TODO is this really supposed to be the user instead of info.Participant?
		attrs["participant"] = waBinary.NewADJID(cli.Session.ID.User, 0, 0)
	} else {
		attrs["to"] = info.From
	}
	err := cli.sendNode(waBinary.Node{
		Tag:     "ack",
		Attrs:   attrs,
		Content: nil,
	})
	if err != nil {
		cli.Log.Warnfln("Failed to send acknowledgement for %s: %v", info.ID, err)
	}
}
