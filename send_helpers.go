// Copyright (c) 2024 André Marques
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/proto/waE2E"
)

// BuildButtonsMessage creates a message with normal buttons.
// Buttons are displayed below the message content.
//
// Example:
//
//	buttons := []*waE2E.ButtonsMessage_Button{
//		{
//			ButtonID: proto.String("1"),
//			ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{DisplayText: proto.String("Yes")},
//			Type: waE2E.ButtonsMessage_Button_RESPONSE.Enum(),
//		},
//		{
//			ButtonID: proto.String("2"),
//			ButtonText: &waE2E.ButtonsMessage_Button_ButtonText{DisplayText: proto.String("No")},
//			Type: waE2E.ButtonsMessage_Button_RESPONSE.Enum(),
//		},
//	}
//	msg := cli.BuildButtonsMessage("Do you agree?", "Choose an option", buttons, nil)
//	cli.SendMessage(ctx, targetJID, msg)
func (cli *Client) BuildButtonsMessage(
	content string,
	footer string,
	buttons []*waE2E.ButtonsMessage_Button,
	contextInfo *waE2E.ContextInfo,
) *waE2E.Message {
	buttonsMsg := &waE2E.ButtonsMessage{
		ContentText: proto.String(content),
		Buttons:     buttons,
		HeaderType:  waE2E.ButtonsMessage_EMPTY.Enum(),
	}

	if footer != "" {
		buttonsMsg.FooterText = proto.String(footer)
	}

	if contextInfo != nil {
		buttonsMsg.ContextInfo = contextInfo
	}

	return &waE2E.Message{
		ButtonsMessage: buttonsMsg,
	}
}

// BuildTemplateButtonsMessage creates a message with template buttons (hydrated buttons).
// Supports buttons with actions like calls, URLs, etc.
//
// Example:
//
//	buttons := []*waE2E.HydratedTemplateButton{
//		{
//			HydratedButton: &waE2E.HydratedTemplateButton_QuickReplyButton{
//				QuickReplyButton: &waE2E.HydratedTemplateButton_HydratedQuickReplyButton{
//					DisplayText: proto.String("Quick Reply"),
//					ID: proto.String("id1"),
//				},
//			},
//		},
//	}
//	msg := cli.BuildTemplateButtonsMessage("Content", "Footer", buttons, nil)
//	cli.SendMessage(ctx, targetJID, msg)
func (cli *Client) BuildTemplateButtonsMessage(
	content string,
	footer string,
	buttons []*waE2E.HydratedTemplateButton,
	contextInfo *waE2E.ContextInfo,
) *waE2E.Message {
	templateMsg := &waE2E.TemplateMessage{
		HydratedTemplate: &waE2E.TemplateMessage_HydratedFourRowTemplate{
			HydratedContentText: proto.String(content),
			HydratedButtons:     buttons,
		},
	}

	if footer != "" {
		templateMsg.HydratedTemplate.HydratedFooterText = proto.String(footer)
	}

	if contextInfo != nil {
		templateMsg.ContextInfo = contextInfo
	}

	return &waE2E.Message{
		TemplateMessage: templateMsg,
	}
}

// BuildListMessage creates a message with a list of options.
// The user can select one option from the list.
//
// Example:
//
//	sections := []*waE2E.ListMessage_Section{
//		{
//			Title: proto.String("Section 1"),
//			Rows: []*waE2E.ListMessage_Row{
//				{RowID: proto.String("1"), Title: proto.String("Option 1"), Description: proto.String("Description 1")},
//				{RowID: proto.String("2"), Title: proto.String("Option 2"), Description: proto.String("Description 2")},
//			},
//		},
//	}
//	msg := cli.BuildListMessage("Choose an option", "List description", "View options", sections, "Footer")
//	cli.SendMessage(ctx, targetJID, msg)
func (cli *Client) BuildListMessage(
	title string,
	description string,
	buttonText string,
	sections []*waE2E.ListMessage_Section,
	footer string,
) *waE2E.Message {
	listMsg := &waE2E.ListMessage{
		Title:       proto.String(title),
		Description: proto.String(description),
		ButtonText:  proto.String(buttonText),
		Sections:    sections,
		ListType:    waE2E.ListMessage_SINGLE_SELECT.Enum(),
	}

	if footer != "" {
		listMsg.FooterText = proto.String(footer)
	}

	return &waE2E.Message{
		ListMessage: listMsg,
	}
}

// BuildInteractiveMessage creates an interactive message with NativeFlowMessage.
// Used to create advanced messages with native buttons.
//
// Example:
//
//	header := &waE2E.InteractiveMessage_Header{Title: proto.String("Title")}
//	body := &waE2E.InteractiveMessage_Body{Text: proto.String("Message body")}
//	footer := &waE2E.InteractiveMessage_Footer{Text: proto.String("Footer")}
//	nativeFlow := &waE2E.InteractiveMessage_NativeFlowMessage{
//		Buttons: []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton{
//			{
//				Name: proto.String("cta_url"),
//				ButtonParamsJSON: proto.String(`{"display_text":"Visit Website","url":"https://example.com"}`),
//			},
//		},
//		MessageParamsJSON: proto.String("{}"),
//		MessageVersion: proto.Int32(3),
//	}
//	msg := cli.BuildInteractiveMessage(header, body, footer, nativeFlow)
//	cli.SendMessage(ctx, targetJID, msg)
func (cli *Client) BuildInteractiveMessage(
	header *waE2E.InteractiveMessage_Header,
	body *waE2E.InteractiveMessage_Body,
	footer *waE2E.InteractiveMessage_Footer,
	nativeFlow *waE2E.InteractiveMessage_NativeFlowMessage,
) *waE2E.Message {
	interactiveMsg := &waE2E.InteractiveMessage{
		Header: header,
		Body:   body,
		Footer: footer,
	}

	if nativeFlow != nil {
		interactiveMsg.InteractiveMessage = &waE2E.InteractiveMessage_NativeFlowMessage_{
			NativeFlowMessage: nativeFlow,
		}
	}

	return &waE2E.Message{
		InteractiveMessage: interactiveMsg,
	}
}

// BuildCarouselMessage creates a message with a carousel of cards.
// Each card is an InteractiveMessage.
//
// Example:
//
//	cards := []*waE2E.InteractiveMessage{
//		{
//			Header: &waE2E.InteractiveMessage_Header{Title: proto.String("Card 1")},
//			Body:   &waE2E.InteractiveMessage_Body{Text: proto.String("First card content")},
//		},
//		{
//			Header: &waE2E.InteractiveMessage_Header{Title: proto.String("Card 2")},
//			Body:   &waE2E.InteractiveMessage_Body{Text: proto.String("Second card content")},
//		},
//	}
//	msg := cli.BuildCarouselMessage(cards, waE2E.InteractiveMessage_CarouselMessage_HSCROLL_CARDS)
//	cli.SendMessage(ctx, targetJID, msg)
func (cli *Client) BuildCarouselMessage(
	cards []*waE2E.InteractiveMessage,
	cardType waE2E.InteractiveMessage_CarouselMessage_CarouselCardType,
) *waE2E.Message {
	carousel := &waE2E.InteractiveMessage_CarouselMessage{
		Cards:            cards,
		MessageVersion:   proto.Int32(1),
		CarouselCardType: cardType.Enum(),
	}

	return &waE2E.Message{
		InteractiveMessage: &waE2E.InteractiveMessage{
			InteractiveMessage: &waE2E.InteractiveMessage_CarouselMessage_{
				CarouselMessage: carousel,
			},
		},
	}
}
