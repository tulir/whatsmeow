// Copyright (c) 2024 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"encoding/base64"

	"go.mau.fi/util/random"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

type PushConfig interface {
	GetPushConfigAttrs() waBinary.Attrs
}

type FCMPushConfig struct {
	Token string `json:"token"`
}

func (fpc *FCMPushConfig) GetPushConfigAttrs() waBinary.Attrs {
	return waBinary.Attrs{
		"id":       fpc.Token,
		"num_acc":  1,
		"platform": "gcm",
	}
}

type APNsPushConfig struct {
	Token       string `json:"token"`
	VoIPToken   string `json:"voip_token"`
	MsgIDEncKey []byte `json:"msg_id_enc_key"`
}

func (apc *APNsPushConfig) GetPushConfigAttrs() waBinary.Attrs {
	if len(apc.MsgIDEncKey) != 32 {
		apc.MsgIDEncKey = random.Bytes(32)
	}
	attrs := waBinary.Attrs{
		"id":                  apc.Token,
		"platform":            "apple",
		"version":             2,
		"reg_push":            1,
		"preview":             1,
		"pkey":                base64.RawURLEncoding.EncodeToString(apc.MsgIDEncKey),
		"background_location": 1, // or 0
		"call":                "Opening.m4r",
		"default":             "note.m4r",
		"groups":              "note.m4r",
		"lg":                  "en",
		"lc":                  "US",
		"nse_call":            0,
		"nse_ver":             2,
		"nse_read":            0,
		"voip_payload_type":   2,
	}
	if apc.VoIPToken != "" {
		attrs["voip"] = apc.VoIPToken
	}
	return attrs
}

type WebPushConfig struct {
	Endpoint string `json:"endpoint"`
	Auth     []byte `json:"auth"`
	P256DH   []byte `json:"p256dh"`
}

func (wpc *WebPushConfig) GetPushConfigAttrs() waBinary.Attrs {
	return waBinary.Attrs{
		"platform": "web",
		"endpoint": wpc.Endpoint,
		"auth":     base64.StdEncoding.EncodeToString(wpc.Auth),
		"p256dh":   base64.StdEncoding.EncodeToString(wpc.P256DH),
	}
}

func (cli *Client) GetServerPushNotificationConfig(ctx context.Context) (*waBinary.Node, error) {
	resp, err := cli.sendIQ(ctx, infoQuery{
		Namespace: "urn:xmpp:whatsapp:push",
		Type:      iqGet,
		To:        types.ServerJID,
		Content:   []waBinary.Node{{Tag: "settings"}},
	})
	return resp, err
}

// RegisterForPushNotifications registers a token to receive push notifications for new WhatsApp messages.
//
// This is generally not necessary for anything. Don't use this if you don't know what you're doing.
func (cli *Client) RegisterForPushNotifications(ctx context.Context, pc PushConfig) error {
	_, err := cli.sendIQ(ctx, infoQuery{
		Namespace: "urn:xmpp:whatsapp:push",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag:   "config",
			Attrs: pc.GetPushConfigAttrs(),
		}},
	})
	return err
}
