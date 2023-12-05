// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/go-whatsapp/go-util/random"

	waBinary "github.com/go-whatsapp/whatsmeow/binary"
	waProto "github.com/go-whatsapp/whatsmeow/binary/proto"
	"github.com/go-whatsapp/whatsmeow/types"
	"github.com/go-whatsapp/whatsmeow/types/events"
)

func (cli *Client) handleCallEvent(node *waBinary.Node) {
	go cli.sendAck(node)

	if len(node.GetChildren()) != 1 {
		cli.dispatchEvent(&events.UnknownCallEvent{Node: node})
		return
	}
	ag := node.AttrGetter()
	child := node.GetChildren()[0]
	cag := child.AttrGetter()
	basicMeta := types.BasicCallMeta{
		From:        ag.JID("from"),
		Timestamp:   ag.UnixTime("t"),
		CallCreator: cag.JID("call-creator"),
		CallID:      cag.String("call-id"),
	}
	switch child.Tag {
	case "offer":
		cli.dispatchEvent(&events.CallOffer{
			BasicCallMeta: basicMeta,
			CallRemoteMeta: types.CallRemoteMeta{
				RemotePlatform: ag.String("platform"),
				RemoteVersion:  ag.String("version"),
			},
			Data: &child,
		})
	case "offer_notice":
		cli.dispatchEvent(&events.CallOfferNotice{
			BasicCallMeta: basicMeta,
			Media:         cag.String("media"),
			Type:          cag.String("type"),
			Data:          &child,
		})
	case "relaylatency":
		cli.dispatchEvent(&events.CallRelayLatency{
			BasicCallMeta: basicMeta,
			Data:          &child,
		})
	case "accept":
		cli.dispatchEvent(&events.CallAccept{
			BasicCallMeta: basicMeta,
			CallRemoteMeta: types.CallRemoteMeta{
				RemotePlatform: ag.String("platform"),
				RemoteVersion:  ag.String("version"),
			},
			Data: &child,
		})
	case "preaccept":
		cli.dispatchEvent(&events.CallPreAccept{
			BasicCallMeta: basicMeta,
			CallRemoteMeta: types.CallRemoteMeta{
				RemotePlatform: ag.String("platform"),
				RemoteVersion:  ag.String("version"),
			},
			Data: &child,
		})
	case "transport":
		cli.dispatchEvent(&events.CallTransport{
			BasicCallMeta: basicMeta,
			CallRemoteMeta: types.CallRemoteMeta{
				RemotePlatform: ag.String("platform"),
				RemoteVersion:  ag.String("version"),
			},
			Data: &child,
		})
	case "terminate":
		cli.dispatchEvent(&events.CallTerminate{
			BasicCallMeta: basicMeta,
			Reason:        cag.String("reason"),
			Data:          &child,
		})
	default:
		cli.dispatchEvent(&events.UnknownCallEvent{Node: node})
	}
}

// OfferCall offers a call to a user.
func (cli *Client) OfferCall(callTo types.JID, video bool) error {
	clientID := cli.getOwnJID()
	if clientID.IsEmpty() {
		return ErrNotLoggedIn
	}
	var offerLen uint8 = 6
	if video {
		offerLen++
	}
	callID := strings.ToUpper(hex.EncodeToString(random.Bytes(16)))
	plaintext, dsmPlaintext, err := marshalMessage(callTo, &waProto.Message{Call: &waProto.Call{CallKey: random.Bytes(32)}})
	if err != nil {
		return fmt.Errorf("failed to marshal call: %w", err)
	}
	destinationNode, includeIdentity := cli.encryptMessageForDevices(context.TODO(), []types.JID{clientID, callTo}, clientID, callID, plaintext, dsmPlaintext, nil)
	if includeIdentity {
		destinationNode = append(destinationNode, cli.makeDeviceIdentityNode())
	}
	offerContent := make([]waBinary.Node, 0, offerLen)
	offerContent = append(offerContent,
		waBinary.Node{Tag: "audio", Attrs: waBinary.Attrs{"enc": "opus", "rate": "16000"}},
		waBinary.Node{Tag: "audio", Attrs: waBinary.Attrs{"enc": "opus", "rate": "8000"}},
	)
	if video {
		offerContent = append(offerContent,
			waBinary.Node{Tag: "video", Attrs: waBinary.Attrs{"orientation": "0", "screen_width": "1080", "screen_height": "2340", "device_orientation": "0", "enc": "vp8", "dec": "vp8"}},
		)
	}
	offerContent = append(offerContent,
		waBinary.Node{Tag: "capability", Attrs: waBinary.Attrs{"ver": "1"}, Content: []byte{1, 4, 255, 131, 207, 4}},
		waBinary.Node{Tag: "destination", Content: destinationNode},
		waBinary.Node{Tag: "encopt", Attrs: waBinary.Attrs{"keygen": "2"}},
		waBinary.Node{Tag: "net", Attrs: waBinary.Attrs{"medium": "3"}},
	)
	return cli.sendNode(waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"id": cli.GenerateMessageID(), "to": callTo},
		Content: []waBinary.Node{{
			Tag:     "offer",
			Attrs:   waBinary.Attrs{"call-id": callID, "call-creator": clientID},
			Content: offerContent,
		}},
	})
}

// RejectCall rejects an incoming call.
func (cli *Client) RejectCall(callFrom types.JID, callID string) error {
	ownID := cli.getOwnJID()
	if ownID.IsEmpty() {
		return ErrNotLoggedIn
	}
	ownID, callFrom = ownID.ToNonAD(), callFrom.ToNonAD()
	return cli.sendNode(waBinary.Node{
		Tag:   "call",
		Attrs: waBinary.Attrs{"id": cli.GenerateMessageID(), "from": ownID, "to": callFrom},
		Content: []waBinary.Node{{
			Tag:     "reject",
			Attrs:   waBinary.Attrs{"call-id": callID, "call-creator": callFrom, "count": "0"},
			Content: nil,
		}},
	})
}
