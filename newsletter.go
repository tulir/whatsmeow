// Copyright (c) 2023 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

// NewsletterSubscribeLiveUpdates subscribes to receive live updates from a WhatsApp channel temporarily (for the duration returned).
func (cli *Client) NewsletterSubscribeLiveUpdates(ctx context.Context, jid types.JID) (time.Duration, error) {
	resp, err := cli.sendIQ(infoQuery{
		Context:   ctx,
		Namespace: "newsletter",
		Type:      iqSet,
		To:        jid,
		Content: []waBinary.Node{{
			Tag: "live_updates",
		}},
	})
	if err != nil {
		return 0, err
	}
	child := resp.GetChildByTag("live_updates")
	dur := child.AttrGetter().Int("duration")
	return time.Duration(dur) * time.Second, nil
}

// NewsletterMarkViewed marks a channel message as viewed, incrementing the view counter.
//
// This is not the same as marking the channel as read on your other devices, use the usual MarkRead function for that.
func (cli *Client) NewsletterMarkViewed(jid types.JID, serverIDs []types.MessageServerID) error {
	items := make([]waBinary.Node, len(serverIDs))
	for i, id := range serverIDs {
		items[i] = waBinary.Node{
			Tag: "item",
			Attrs: waBinary.Attrs{
				"server_id": id,
			},
		}
	}
	reqID := cli.generateRequestID()
	resp := cli.waitResponse(reqID)
	err := cli.sendNode(waBinary.Node{
		Tag: "receipt",
		Attrs: waBinary.Attrs{
			"to":   jid,
			"type": "view",
			"id":   reqID,
		},
		Content: []waBinary.Node{{
			Tag:     "list",
			Content: items,
		}},
	})
	if err != nil {
		cli.cancelResponse(reqID, resp)
		return err
	}
	// TODO handle response?
	<-resp
	return nil
}

// NewsletterSendReaction sends a reaction to a channel message.
// To remove a reaction sent earlier, set reaction to an empty string.
//
// The last parameter is the message ID of the reaction itself. It can be left empty to let whatsmeow generate a random one.
func (cli *Client) NewsletterSendReaction(jid types.JID, serverID types.MessageServerID, reaction string, messageID types.MessageID) error {
	if messageID == "" {
		messageID = cli.GenerateMessageID()
	}
	reactionAttrs := waBinary.Attrs{}
	messageAttrs := waBinary.Attrs{
		"to":        jid,
		"id":        messageID,
		"server_id": serverID,
		"type":      "reaction",
	}
	if reaction != "" {
		reactionAttrs["code"] = reaction
	} else {
		messageAttrs["edit"] = string(types.EditAttributeSenderRevoke)
	}
	return cli.sendNode(waBinary.Node{
		Tag:   "message",
		Attrs: messageAttrs,
		Content: []waBinary.Node{{
			Tag:   "reaction",
			Attrs: reactionAttrs,
		}},
	})
}

const (
	queryFetchNewsletter           = "6563316087068696"
	queryFetchNewsletterDehydrated = "7272540469429201"
	queryRecommendedNewsletters    = "7263823273662354" //variables -> input -> {limit: 20, country_codes: [string]}, output: xwa2_newsletters_recommended
	queryNewslettersDirectory      = "6190824427689257" // variables -> input -> {view: "RECOMMENDED", limit: 50, start_cursor: base64, filters: {country_codes: [string]}}
	querySubscribedNewsletters     = "6388546374527196" // variables -> empty, output: xwa2_newsletter_subscribed
	queryNewsletterSubscribers     = "9800646650009898" //variables -> input -> {newsletter_id, count}, output: xwa2_newsletter_subscribers -> subscribers -> edges
	mutationMuteNewsletter         = "6274038279359549" //variables -> {newsletter_id, updates->{description, settings}}, output: xwa2_newsletter_update -> NewsletterMetadata without viewer meta
	mutationUnmuteNewsletter       = "6068417879924485"
	mutationUpdateNewsletter       = "7150902998257522"
	mutationCreateNewsletter       = "6234210096708695"
	mutationUnfollowNewsletter     = "6392786840836363"
	mutationFollowNewsletter       = "9926858900719341"
)

func (cli *Client) sendMexIQ(ctx context.Context, queryID string, variables any) (json.RawMessage, error) {
	payload, err := json.Marshal(map[string]any{
		"variables": variables,
	})
	if err != nil {
		return nil, err
	}
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:mex",
		Type:      iqGet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "query",
			Attrs: waBinary.Attrs{
				"query_id": queryID,
			},
			Content: payload,
		}},
		Context: ctx,
	})
	if err != nil {
		return nil, err
	}
	result, ok := resp.GetOptionalChildByTag("result")
	if !ok {
		return nil, &ElementMissingError{Tag: "result", In: "mex response"}
	}
	resultContent, ok := result.Content.([]byte)
	if !ok {
		return nil, fmt.Errorf("unexpected content type %T in mex response", result.Content)
	}
	var gqlResp types.GraphQLResponse
	err = json.Unmarshal(resultContent, &gqlResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal graphql response: %w", err)
	} else if len(gqlResp.Errors) > 0 {
		return gqlResp.Data, fmt.Errorf("graphql error: %w", gqlResp.Errors)
	}
	return gqlResp.Data, nil
}

type respGetNewsletterInfo struct {
	Newsletter *types.NewsletterMetadata `json:"xwa2_newsletter"`
}

func (cli *Client) getNewsletterInfo(input map[string]any, fetchViewerMeta bool) (*types.NewsletterMetadata, error) {
	data, err := cli.sendMexIQ(context.TODO(), queryFetchNewsletter, map[string]any{
		"fetch_creation_time":   true,
		"fetch_full_image":      true,
		"fetch_viewer_metadata": fetchViewerMeta,
		"input":                 input,
	})
	var respData respGetNewsletterInfo
	if data != nil {
		jsonErr := json.Unmarshal(data, &respData)
		if err == nil && jsonErr != nil {
			err = jsonErr
		}
	}
	return respData.Newsletter, err
}

// GetNewsletterInfo gets the info of a newsletter that you're joined to.
func (cli *Client) GetNewsletterInfo(jid types.JID) (*types.NewsletterMetadata, error) {
	return cli.getNewsletterInfo(map[string]any{
		"key":  jid.String(),
		"type": types.NewsletterKeyTypeJID,
	}, true)
}

// GetNewsletterInfoWithInvite gets the info of a newsletter with an invite link.
//
// You can either pass the full link (https://whatsapp.com/channel/...) or just the `...` part.
//
// Note that the ViewerMeta field of the returned NewsletterMetadata will be nil.
func (cli *Client) GetNewsletterInfoWithInvite(key string) (*types.NewsletterMetadata, error) {
	return cli.getNewsletterInfo(map[string]any{
		"key":  strings.TrimPrefix(key, NewsletterLinkPrefix),
		"type": types.NewsletterKeyTypeInvite,
	}, false)
}

type respGetSubscribedNewsletters struct {
	Newsletters []*types.NewsletterMetadata `json:"xwa2_newsletter_subscribed"`
}

// GetSubscribedNewsletters gets the info of all newsletters that you're joined to.
func (cli *Client) GetSubscribedNewsletters() ([]*types.NewsletterMetadata, error) {
	data, err := cli.sendMexIQ(context.TODO(), querySubscribedNewsletters, map[string]any{})
	var respData respGetSubscribedNewsletters
	if data != nil {
		jsonErr := json.Unmarshal(data, &respData)
		if err == nil && jsonErr != nil {
			err = jsonErr
		}
	}
	return respData.Newsletters, err
}

type CreateNewsletterParams struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Picture     []byte `json:"picture,omitempty"`
}

type respCreateNewsletter struct {
	Newsletter *types.NewsletterMetadata `json:"xwa2_newsletter_create"`
}

// CreateNewsletter creates a new WhatsApp channel.
func (cli *Client) CreateNewsletter(params CreateNewsletterParams) (*types.NewsletterMetadata, error) {
	resp, err := cli.sendMexIQ(context.TODO(), mutationCreateNewsletter, map[string]any{
		"newsletter_input": &params,
	})
	if err != nil {
		return nil, err
	}
	var respData respCreateNewsletter
	err = json.Unmarshal(resp, &respData)
	if err != nil {
		return nil, err
	}
	return respData.Newsletter, nil
}

// AcceptTOSNotice accepts a ToS notice.
//
// To accept the terms for creating newsletters, use
//
//	cli.AcceptTOSNotice("20601218", "5")
func (cli *Client) AcceptTOSNotice(noticeID, stage string) error {
	_, err := cli.sendIQ(infoQuery{
		Namespace: "tos",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "notice",
			Attrs: waBinary.Attrs{
				"id":    noticeID,
				"stage": stage,
			},
		}},
	})
	return err
}

// NewsletterToggleMute changes the mute status of a newsletter.
func (cli *Client) NewsletterToggleMute(jid types.JID, mute bool) error {
	query := mutationUnmuteNewsletter
	if mute {
		query = mutationMuteNewsletter
	}
	_, err := cli.sendMexIQ(context.TODO(), query, map[string]any{
		"newsletter_id": jid.String(),
	})
	return err
}

// FollowNewsletter makes the user follow (join) a WhatsApp channel.
func (cli *Client) FollowNewsletter(jid types.JID) error {
	_, err := cli.sendMexIQ(context.TODO(), mutationFollowNewsletter, map[string]any{
		"newsletter_id": jid.String(),
	})
	return err
}

// UnfollowNewsletter makes the user unfollow (leave) a WhatsApp channel.
func (cli *Client) UnfollowNewsletter(jid types.JID) error {
	_, err := cli.sendMexIQ(context.TODO(), mutationUnfollowNewsletter, map[string]any{
		"newsletter_id": jid.String(),
	})
	return err
}

type GetNewsletterMessagesParams struct {
	Count  int
	Before types.MessageServerID
}

// GetNewsletterMessages gets messages in a WhatsApp channel.
func (cli *Client) GetNewsletterMessages(jid types.JID, params *GetNewsletterMessagesParams) ([]*types.NewsletterMessage, error) {
	attrs := waBinary.Attrs{
		"type": "jid",
		"jid":  jid,
	}
	if params != nil {
		if params.Count != 0 {
			attrs["count"] = params.Count
		}
		if params.Before != 0 {
			attrs["before"] = params.Before
		}
	}
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "newsletter",
		Type:      iqGet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag:   "messages",
			Attrs: attrs,
		}},
		Context: context.TODO(),
	})
	if err != nil {
		return nil, err
	}
	messages, ok := resp.GetOptionalChildByTag("messages")
	if !ok {
		return nil, &ElementMissingError{Tag: "messages", In: "newsletter messages response"}
	}
	return cli.parseNewsletterMessages(&messages), nil
}

type GetNewsletterUpdatesParams struct {
	Count int
	Since time.Time
	After types.MessageServerID
}

// GetNewsletterMessageUpdates gets updates in a WhatsApp channel.
//
// These are the same kind of updates that NewsletterSubscribeLiveUpdates triggers (reaction and view counts).
func (cli *Client) GetNewsletterMessageUpdates(jid types.JID, params *GetNewsletterUpdatesParams) ([]*types.NewsletterMessage, error) {
	attrs := waBinary.Attrs{}
	if params != nil {
		if params.Count != 0 {
			attrs["count"] = params.Count
		}
		if !params.Since.IsZero() {
			attrs["since"] = params.Since.Unix()
		}
		if params.After != 0 {
			attrs["after"] = params.After
		}
	}
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "newsletter",
		Type:      iqGet,
		To:        jid,
		Content: []waBinary.Node{{
			Tag:   "message_updates",
			Attrs: attrs,
		}},
		Context: context.TODO(),
	})
	if err != nil {
		return nil, err
	}
	messages, ok := resp.GetOptionalChildByTag("message_updates", "messages")
	if !ok {
		return nil, &ElementMissingError{Tag: "messages", In: "newsletter messages response"}
	}
	return cli.parseNewsletterMessages(&messages), nil
}
