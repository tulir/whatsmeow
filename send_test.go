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
