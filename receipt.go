// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"fmt"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func (cli *Client) handleReceipt(node *waBinary.Node) {
	receipt, err := cli.parseReceipt(node)
	if err != nil {
		cli.Log.Warnf("Failed to parse receipt: %v", err)
	} else if receipt != nil {
		if receipt.Type == types.ReceiptTypeRetry {
			go func() {
				err := cli.handleRetryReceipt(receipt, node)
				if err != nil {
					cli.Log.Errorf("Failed to handle retry receipt for %s/%s from %s: %v", receipt.Chat, receipt.MessageIDs[0], receipt.Sender, err)
				}
			}()
		}
		go cli.dispatchEvent(receipt)
	}
	go cli.sendAck(node)
}

func (cli *Client) handleGroupedReceipt(partialReceipt events.Receipt, participants *waBinary.Node) {
	pag := participants.AttrGetter()
	partialReceipt.MessageIDs = []types.MessageID{pag.String("key")}
	for _, child := range participants.GetChildren() {
		if child.Tag != "user" {
			cli.Log.Warnf("Unexpected node in grouped receipt participants: %s", child.XMLString())
			continue
		}
		ag := child.AttrGetter()
		receipt := partialReceipt
		receipt.Timestamp = ag.UnixTime("t")
		receipt.MessageSource.Sender = ag.JID("jid")
		if !ag.OK() {
			cli.Log.Warnf("Failed to parse user node %s in grouped receipt: %v", child.XMLString(), ag.Error())
			continue
		}
		go cli.dispatchEvent(&receipt)
	}
}

func (cli *Client) parseReceipt(node *waBinary.Node) (*events.Receipt, error) {
	ag := node.AttrGetter()
	source, err := cli.parseMessageSource(node, false)
	if err != nil {
		return nil, err
	}
	receipt := events.Receipt{
		MessageSource: source,
		Timestamp:     ag.UnixTime("t"),
		Type:          types.ReceiptType(ag.OptionalString("type")),
	}
	if source.IsGroup && source.Sender.IsEmpty() {
		participantTags := node.GetChildrenByTag("participants")
		if len(participantTags) == 0 {
			return nil, &ElementMissingError{Tag: "participants", In: "grouped receipt"}
		}
		for _, pcp := range participantTags {
			cli.handleGroupedReceipt(receipt, &pcp)
		}
		return nil, nil
	}
	mainMessageID := ag.String("id")
	if !ag.OK() {
		return nil, fmt.Errorf("failed to parse read receipt attrs: %+v", ag.Errors)
	}

	receiptChildren := node.GetChildren()
	if len(receiptChildren) == 1 && receiptChildren[0].Tag == "list" {
		listChildren := receiptChildren[0].GetChildren()
		receipt.MessageIDs = make([]string, 1, len(listChildren)+1)
		receipt.MessageIDs[0] = mainMessageID
		for _, item := range listChildren {
			if id, ok := item.Attrs["id"].(string); ok && item.Tag == "item" {
				receipt.MessageIDs = append(receipt.MessageIDs, id)
			}
		}
	} else {
		receipt.MessageIDs = []types.MessageID{mainMessageID}
	}
	return &receipt, nil
}

func (cli *Client) sendAck(node *waBinary.Node) {
	attrs := waBinary.Attrs{
		"class": node.Tag,
		"id":    node.Attrs["id"],
	}
	attrs["to"] = node.Attrs["from"]
	if participant, ok := node.Attrs["participant"]; ok {
		attrs["participant"] = participant
	}
	if recipient, ok := node.Attrs["recipient"]; ok {
		attrs["recipient"] = recipient
	}
	if receiptType, ok := node.Attrs["type"]; node.Tag != "message" && ok {
		attrs["type"] = receiptType
	}
	err := cli.sendNode(waBinary.Node{
		Tag:   "ack",
		Attrs: attrs,
	})
	if err != nil {
		cli.Log.Warnf("Failed to send acknowledgement for %s %s: %v", node.Tag, node.Attrs["id"], err)
	}
}

// MarkRead sends a read receipt for the given message IDs including the given timestamp as the read at time.
//
// The first JID parameter (chat) must always be set to the chat ID (user ID in DMs and group ID in group chats).
// The second JID parameter (sender) must be set in group chats and must be the user ID who sent the message.
//
// You can mark multiple messages as read at the same time, but only if the messages were sent by the same user.
// To mark messages by different users as read, you must call MarkRead multiple times (once for each user).
//
// To mark a voice message as played, specify types.ReceiptTypePlayed as the last parameter.
// Providing more than one receipt type will panic: the parameter is only a vararg for backwards compatibility.
func (cli *Client) MarkRead(ids []types.MessageID, timestamp time.Time, chat, sender types.JID, receiptTypeExtra ...types.ReceiptType) error {
	if len(ids) == 0 {
		return fmt.Errorf("no message IDs specified")
	}
	receiptType := types.ReceiptTypeRead
	if len(receiptTypeExtra) == 1 {
		receiptType = receiptTypeExtra[0]
	} else if len(receiptTypeExtra) > 1 {
		panic(fmt.Errorf("too many receipt types specified"))
	}
	node := waBinary.Node{
		Tag: "receipt",
		Attrs: waBinary.Attrs{
			"id":   ids[0],
			"type": string(receiptType),
			"to":   chat,
			"t":    timestamp.Unix(),
		},
	}
	if chat.Server == types.NewsletterServer || cli.GetPrivacySettings().ReadReceipts == types.PrivacySettingNone {
		switch receiptType {
		case types.ReceiptTypeRead:
			node.Attrs["type"] = string(types.ReceiptTypeReadSelf)
			// TODO change played to played-self?
		}
	}
	if !sender.IsEmpty() && chat.Server != types.DefaultUserServer && chat.Server != types.MessengerServer {
		node.Attrs["participant"] = sender.ToNonAD()
	}
	if len(ids) > 1 {
		children := make([]waBinary.Node, len(ids)-1)
		for i := 1; i < len(ids); i++ {
			children[i-1].Tag = "item"
			children[i-1].Attrs = waBinary.Attrs{"id": ids[i]}
		}
		node.Content = []waBinary.Node{{
			Tag:     "list",
			Content: children,
		}}
	}
	return cli.sendNode(node)
}

// SetForceActiveDeliveryReceipts will force the client to send normal delivery
// receipts (which will show up as the two gray ticks on WhatsApp), even if the
// client isn't marked as online.
//
// By default, clients that haven't been marked as online will send delivery
// receipts with type="inactive", which is transmitted to the sender, but not
// rendered in the official WhatsApp apps. This is consistent with how WhatsApp
// web works when it's not in the foreground.
//
// To mark the client as online, use
//
//	cli.SendPresence(types.PresenceAvailable)
//
// Note that if you turn this off (i.e. call SetForceActiveDeliveryReceipts(false)),
// receipts will act like the client is offline until SendPresence is called again.
func (cli *Client) SetForceActiveDeliveryReceipts(active bool) {
	if active {
		cli.sendActiveReceipts.Store(2)
	} else {
		cli.sendActiveReceipts.Store(0)
	}
}

func (cli *Client) sendMessageReceipt(info *types.MessageInfo) {
	attrs := waBinary.Attrs{
		"id": info.ID,
	}
	if info.IsFromMe {
		attrs["type"] = string(types.ReceiptTypeSender)
	} else if cli.sendActiveReceipts.Load() == 0 {
		attrs["type"] = string(types.ReceiptTypeInactive)
	}
	attrs["to"] = info.Chat
	if info.IsGroup {
		attrs["participant"] = info.Sender
	} else if info.IsFromMe {
		attrs["recipient"] = info.Sender
	} else {
		// Override the to attribute with the JID version with a device number
		attrs["to"] = info.Sender
	}
	err := cli.sendNode(waBinary.Node{
		Tag:   "receipt",
		Attrs: attrs,
	})
	if err != nil {
		cli.Log.Warnf("Failed to send receipt for %s: %v", info.ID, err)
	}
}
