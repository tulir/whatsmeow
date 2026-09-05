// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

func nestedQuote() *waE2E.Message {
	return &waE2E.Message{Conversation: proto.String("the message being quoted by the quoted message")}
}

func TestStripQuotedMessageKnownType(t *testing.T) {
	original := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption: proto.String("hi"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID:      proto.String("OLDER"),
				QuotedMessage: nestedQuote(),
			},
		},
		MessageContextInfo: &waE2E.MessageContextInfo{MessageSecret: []byte("secret")},
	}
	stripped := stripQuotedMessage(original)
	if stripped.MessageContextInfo != nil {
		t.Error("stripped message still has MessageContextInfo")
	}
	if stripped.GetImageMessage().GetContextInfo().GetQuotedMessage() != nil {
		t.Error("stripped message still has a nested quote")
	}
	if stripped.GetImageMessage().GetCaption() != "hi" {
		t.Error("stripped message lost the image caption")
	}
	if original.MessageContextInfo == nil || original.GetImageMessage().GetContextInfo().GetQuotedMessage() == nil {
		t.Error("stripQuotedMessage mutated the input message")
	}
}

func TestStripQuotedMessageUnknownType(t *testing.T) {
	original := &waE2E.Message{
		EventMessage: &waE2E.EventMessage{
			Name: proto.String("party"),
			ContextInfo: &waE2E.ContextInfo{
				StanzaID:      proto.String("OLDER"),
				QuotedMessage: nestedQuote(),
			},
		},
		MessageContextInfo: &waE2E.MessageContextInfo{MessageSecret: []byte("secret")},
	}
	stripped := stripQuotedMessage(original)
	if stripped.MessageContextInfo != nil {
		t.Error("stripped message still has MessageContextInfo")
	}
	if stripped.GetEventMessage().GetContextInfo().GetQuotedMessage() != nil {
		t.Error("stripped message still has a nested quote")
	}
	if stripped.GetEventMessage().GetName() != "party" {
		t.Error("stripped message lost the event name")
	}
	if stripped.GetEventMessage().GetContextInfo().GetStanzaID() != "OLDER" {
		t.Error("stripped message lost the rest of the context info")
	}
	if original.MessageContextInfo == nil || original.GetEventMessage().GetContextInfo().GetQuotedMessage() == nil {
		t.Error("stripQuotedMessage mutated the input message")
	}
}

func TestStripQuotedMessageExtendedTextWithoutExtras(t *testing.T) {
	stripped := stripQuotedMessage(&waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:        proto.String("meow"),
			ContextInfo: &waE2E.ContextInfo{QuotedMessage: nestedQuote()},
		},
	})
	if stripped.GetConversation() != "meow" {
		t.Errorf("expected plain conversation, got %s", stripped.String())
	}
}

func TestBuildReplyDoesNotMutateInput(t *testing.T) {
	cli := &Client{Store: &store.Device{}}
	quotedInfo := &types.MessageInfo{
		ID: "QUOTED",
		MessageSource: types.MessageSource{
			Chat:   types.NewJID("12345", types.GroupServer),
			Sender: types.NewJID("67890", types.DefaultUserServer),
		},
	}
	quotedMsg := &waE2E.Message{
		Conversation:       proto.String("original"),
		MessageContextInfo: &waE2E.MessageContextInfo{MessageSecret: []byte("secret")},
	}
	replyContent := &waE2E.Message{Conversation: proto.String("answering")}
	reply, err := cli.BuildReply(quotedInfo, quotedMsg, replyContent)
	if err != nil {
		t.Fatalf("BuildReply returned an error: %v", err)
	}
	if replyContent.GetConversation() != "answering" || replyContent.ExtendedTextMessage != nil {
		t.Error("BuildReply mutated the reply content it was given")
	}
	if quotedMsg.MessageContextInfo == nil {
		t.Error("BuildReply mutated the quoted message it was given")
	}
	ctxInfo := reply.GetExtendedTextMessage().GetContextInfo()
	if reply.GetExtendedTextMessage().GetText() != "answering" {
		t.Error("reply lost its text")
	}
	if ctxInfo.GetStanzaID() != "QUOTED" {
		t.Errorf("unexpected stanza ID %s", ctxInfo.GetStanzaID())
	}
	if ctxInfo.GetParticipant() != "67890@s.whatsapp.net" {
		t.Errorf("unexpected participant %s", ctxInfo.GetParticipant())
	}
	if ctxInfo.GetQuotedMessage().GetConversation() != "original" {
		t.Error("reply doesn't quote the original message")
	}
	if ctxInfo.GetQuotedMessage().MessageContextInfo != nil {
		t.Error("quoted message still has MessageContextInfo")
	}
}

func TestStripQuotedMessageWrappedType(t *testing.T) {
	original := &waE2E.Message{
		ViewOnceMessageV2: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{
				ImageMessage: &waE2E.ImageMessage{
					Caption: proto.String("hi"),
					ContextInfo: &waE2E.ContextInfo{
						StanzaID:      proto.String("OLDER"),
						QuotedMessage: nestedQuote(),
					},
				},
				MessageContextInfo: &waE2E.MessageContextInfo{MessageSecret: []byte("inner secret")},
			},
		},
		MessageContextInfo: &waE2E.MessageContextInfo{MessageSecret: []byte("secret")},
	}
	stripped := stripQuotedMessage(original)
	inner := stripped.GetViewOnceMessageV2().GetMessage()
	if stripped.MessageContextInfo != nil {
		t.Error("stripped message still has MessageContextInfo")
	}
	if inner.MessageContextInfo != nil {
		t.Error("wrapped message still has MessageContextInfo")
	}
	if inner.GetImageMessage().GetContextInfo().GetQuotedMessage() != nil {
		t.Error("wrapped message still has a nested quote")
	}
	if inner.GetImageMessage().GetCaption() != "hi" {
		t.Error("wrapped message lost the image caption")
	}
	if original.GetViewOnceMessageV2().GetMessage().MessageContextInfo == nil {
		t.Error("stripQuotedMessage mutated the input message")
	}
}

func TestBuildReplyToEventMessage(t *testing.T) {
	cli := &Client{Store: &store.Device{}}
	reply, err := cli.BuildReply(
		&types.MessageInfo{
			ID:            "QUOTED",
			MessageSource: types.MessageSource{Sender: types.NewJID("67890", types.DefaultUserServer)},
		},
		&waE2E.Message{Conversation: proto.String("original")},
		&waE2E.Message{EventMessage: &waE2E.EventMessage{Name: proto.String("party")}},
	)
	if err != nil {
		t.Fatalf("BuildReply returned an error: %v", err)
	}
	ctxInfo := reply.GetEventMessage().GetContextInfo()
	if ctxInfo.GetStanzaID() != "QUOTED" {
		t.Errorf("unexpected stanza ID %s", ctxInfo.GetStanzaID())
	}
	if ctxInfo.GetQuotedMessage().GetConversation() != "original" {
		t.Error("reply doesn't quote the original message")
	}
}

func TestBuildReplyWithWrappedContent(t *testing.T) {
	cli := &Client{Store: &store.Device{}}
	reply, err := cli.BuildReply(
		&types.MessageInfo{
			ID:            "QUOTED",
			MessageSource: types.MessageSource{Sender: types.NewJID("67890", types.DefaultUserServer)},
		},
		&waE2E.Message{Conversation: proto.String("original")},
		&waE2E.Message{ViewOnceMessageV2: &waE2E.FutureProofMessage{
			Message: &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: proto.String("look")}},
		}},
	)
	if err != nil {
		t.Fatalf("BuildReply returned an error: %v", err)
	}
	ctxInfo := reply.GetViewOnceMessageV2().GetMessage().GetImageMessage().GetContextInfo()
	if ctxInfo.GetStanzaID() != "QUOTED" {
		t.Errorf("unexpected stanza ID %s", ctxInfo.GetStanzaID())
	}
	if ctxInfo.GetQuotedMessage().GetConversation() != "original" {
		t.Error("reply doesn't quote the original message")
	}
}

func TestBuildReplyReplacesExistingQuote(t *testing.T) {
	cli := &Client{Store: &store.Device{}}
	replyContent := &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{
		Text: proto.String("answering"),
		ContextInfo: &waE2E.ContextInfo{
			StanzaID:        proto.String("SOMETHING ELSE"),
			Participant:     proto.String("11111@s.whatsapp.net"),
			QuotedMessage:   &waE2E.Message{Conversation: proto.String("some other message")},
			MentionedJID:    []string{"22222@s.whatsapp.net"},
			IsForwarded:     proto.Bool(true),
			ForwardingScore: proto.Uint32(3),
		},
	}}
	reply, err := cli.BuildReply(
		&types.MessageInfo{
			ID:            "QUOTED",
			MessageSource: types.MessageSource{Sender: types.NewJID("67890", types.DefaultUserServer)},
		},
		&waE2E.Message{Conversation: proto.String("original")},
		replyContent,
	)
	if err != nil {
		t.Fatalf("BuildReply returned an error: %v", err)
	}
	ctxInfo := reply.GetExtendedTextMessage().GetContextInfo()
	if ctxInfo.GetStanzaID() != "QUOTED" {
		t.Errorf("unexpected stanza ID %s", ctxInfo.GetStanzaID())
	}
	if ctxInfo.GetParticipant() != "67890@s.whatsapp.net" {
		t.Errorf("unexpected participant %s", ctxInfo.GetParticipant())
	}
	if ctxInfo.GetQuotedMessage().GetConversation() != "original" {
		t.Error("reply still quotes the message it quoted before")
	}
	if !ctxInfo.GetIsForwarded() || ctxInfo.GetForwardingScore() != 3 {
		t.Error("reply lost the forwarding info it already had")
	}
	if len(ctxInfo.GetMentionedJID()) != 1 {
		t.Error("reply lost the mentions it already had")
	}
	if replyContent.GetExtendedTextMessage().GetContextInfo().GetStanzaID() != "SOMETHING ELSE" {
		t.Error("BuildReply mutated the reply content it was given")
	}
}

func TestBuildReplyUnsupportedType(t *testing.T) {
	cli := &Client{Store: &store.Device{}}
	_, err := cli.BuildReply(
		&types.MessageInfo{ID: "QUOTED"},
		&waE2E.Message{Conversation: proto.String("original")},
		&waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{Text: proto.String("🐈️")}},
	)
	if !errors.Is(err, ErrUnsupportedReplyType) {
		t.Errorf("expected ErrUnsupportedReplyType, got %v", err)
	}
}
