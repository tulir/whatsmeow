// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/voip"
)

func (cli *Client) handleCallEvent(ctx context.Context, node *waBinary.Node) {
	children := node.GetChildren()
	if len(children) != 1 {
		defer cli.maybeDeferredAck(ctx, node)()
		cli.dispatchEvent(&events.UnknownCallEvent{Node: node})
		return
	}
	if children[0].Tag == "video" {
		cli.sendCallVideoAck(ctx, node)
	} else {
		defer cli.maybeDeferredAck(ctx, node)()
	}
	ag := node.AttrGetter()
	child := children[0]
	cag := child.AttrGetter()
	basicMeta := types.BasicCallMeta{
		From:        ag.JID("from"),
		Timestamp:   ag.UnixTime("t"),
		CallCreator: cag.JID("call-creator"),
		CallID:      cag.String("call-id"),
		GroupJID:    cag.OptionalJIDOrEmpty("group-jid"),
	}
	if basicMeta.CallCreator.Server == types.HiddenUserServer {
		basicMeta.CallCreatorAlt = cag.OptionalJIDOrEmpty("caller_pn")
	} else {
		basicMeta.CallCreatorAlt = cag.OptionalJIDOrEmpty("caller_lid")
	}
	switch child.Tag {
	case "offer":
		cli.onCallOffer(ctx, &child, basicMeta, types.CallRemoteMeta{
			RemotePlatform: ag.String("platform"),
			RemoteVersion:  ag.String("version"),
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
		cli.onRelayLatency(ctx, basicMeta, &child)
	case "accept":
		cli.onCallAccept(basicMeta, types.CallRemoteMeta{
			RemotePlatform: ag.String("platform"),
			RemoteVersion:  ag.String("version"),
		}, &child)
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
		if cs := cli.getCall(basicMeta.CallID); cs != nil {
			cli.captureCallRelay(cs, &child)
		}
	case "terminate":
		cli.onCallTerminate(&child, basicMeta, cag.String("reason"))
	case "reject":
		cli.onCallReject(&child, basicMeta)
	case "mute_v2":
		cli.onCallMuteV2(ctx, basicMeta, cag)
	case "video":
		cli.onCallVideo(basicMeta, &child)
	default:
		cli.dispatchEvent(&events.UnknownCallEvent{Node: node})
	}
}

func (cli *Client) onCallOffer(ctx context.Context, child *waBinary.Node, meta types.BasicCallMeta, remote types.CallRemoteMeta) {
	cag := child.AttrGetter()
	if cag.OptionalString("is_call_ended") == "1" || cag.OptionalString("terminate_reason") != "" {
		cli.Log.Debugf("Ignoring already-ended call offer, call_id: %s", meta.CallID)
		return
	}

	callKey, err := cli.decryptIncomingCallKey(ctx, &events.CallOffer{BasicCallMeta: meta, CallRemoteMeta: remote, Data: child})
	if err != nil {
		cli.Log.Warnf("Failed to decrypt call key, call_id: %s: %v", meta.CallID, err)
		return
	}
	cli.acceptInboundOffer(ctx, child, meta, remote, callKey)
}

func (cli *Client) acceptInboundOffer(ctx context.Context, child *waBinary.Node, meta types.BasicCallMeta, remote types.CallRemoteMeta, callKey []byte) *callState {
	peer := meta.CallCreator
	if peer.IsEmpty() {
		peer = meta.From
	}
	relay := voip.ParseRelay(child, types.CallDirectionIncoming)
	isVideo := voip.OfferHasVideo(child)

	cs := cli.getCall(meta.CallID)
	isNew := cs == nil
	if isNew {
		cs = &callState{
			meta:        meta,
			selfLID:     cli.getOwnLID(),
			peerLID:     peer,
			to:          meta.From,
			creator:     meta.CallCreator,
			callKey:     callKey,
			relay:       relay,
			localVideo:  isVideo,
			remoteVideo: isVideo,
		}
		cli.putCall(meta.CallID, cs)
	} else {
		cli.callsLock.Lock()
		cs.callKey = callKey
		if relay != nil {
			cs.relay = relay
		}
		cli.callsLock.Unlock()
	}
	cli.applyVoipSettingsCodec(cs, child, meta.CallID)

	pre := voip.BuildEagerPreaccept(meta.CallID, meta.From, meta.CallCreator, cli.generateRequestID(), isVideo)
	if err := cli.sendNode(ctx, pre); err != nil {
		cli.Log.Warnf("Failed to send call preaccept, call_id: %s: %v", meta.CallID, err)
	}

	if isNew {
		cli.dispatchEvent(&events.CallOffer{BasicCallMeta: meta, CallRemoteMeta: remote, Data: child, Video: isVideo})
	}
	cli.maybeEmitMediaReady(cs)
	return cs
}

func (cli *Client) applyVoipSettingsCodec(cs *callState, node *waBinary.Node, callID string) {
	vsNode := voip.FindChild(node, "voip_settings")
	if vsNode == nil {
		return
	}
	codec, err := voip.ParseCodec(voip.NodeBytes(vsNode))
	if err != nil {
		cli.Log.Debugf("Failed to parse voip_settings, call_id: %s: %v", callID, err)
		return
	}
	cli.callsLock.Lock()
	cs.codec = codec
	cli.callsLock.Unlock()
	cli.Log.Debugf("Selected call codec, call_id: %s, codec: %s", callID, codec)
}

func (cli *Client) onRelayLatency(ctx context.Context, meta types.BasicCallMeta, child *waBinary.Node) {
	cs := cli.getCall(meta.CallID)
	if cs == nil {
		return
	}
	cli.captureCallRelay(cs, child)
	if cs.outgoing {
		return
	}

	kids := child.GetChildren()
	for i := range kids {
		te := &kids[i]
		if te.Tag != "te" {
			continue
		}
		tag := te.AttrGetter()
		resp := voip.BuildRelayLatency(&voip.RelayLatencyParams{
			CallID:       meta.CallID,
			To:           meta.From,
			CallCreator:  meta.CallCreator,
			LatencyMs:    voip.DecodeLatency(tag.String("latency")),
			RelayName:    tag.String("relay_name"),
			AddressBytes: voip.NodeBytes(te),
		})
		resp.Attrs["id"] = cli.GenerateMessageID()
		if err := cli.sendNode(ctx, resp); err != nil {
			cli.Log.Warnf("Failed to send relaylatency response, call_id: %s: %v", meta.CallID, err)
			return
		}
	}
}

func (cli *Client) onCallTerminate(child *waBinary.Node, meta types.BasicCallMeta, reason string) {
	cli.dropCall(meta.CallID)
	cli.dispatchEvent(&events.CallMediaStop{BasicCallMeta: meta, Reason: reason})
	cli.dispatchEvent(&events.CallTerminate{BasicCallMeta: meta, Reason: reason, Data: child})
}

func (cli *Client) onCallReject(child *waBinary.Node, meta types.BasicCallMeta) {
	cs := cli.getCall(meta.CallID)
	cli.dropCall(meta.CallID)
	cli.dispatchEvent(&events.CallReject{BasicCallMeta: meta, Data: child})
	if cs != nil {
		cli.dispatchEvent(&events.CallMediaStop{BasicCallMeta: cs.meta, Reason: "rejected"})
	}
}

func (cli *Client) onCallMuteV2(ctx context.Context, meta types.BasicCallMeta, mv *waBinary.AttrUtility) {
	cli.dispatchEvent(&events.CallMute{BasicCallMeta: meta, Muted: mv.String("mute-state") == "1"})

	cs := cli.getCall(meta.CallID)
	if cs == nil {
		return
	}
	cli.callsLock.Lock()
	pending := cs.acceptPending
	cs.acceptPending = false
	video := cs.localVideo || cs.remoteVideo
	cli.callsLock.Unlock()
	if !pending {
		return
	}

	accept := voip.BuildAccept(&voip.AcceptParams{
		CallID: meta.CallID, To: meta.From, CallCreator: meta.CallCreator,
		AudioRates: []string{"16000"},
		Metadata:   waBinary.Attrs{"peer_abtest_bucket_id_list": "125208,94276"},
		Video:      video,
	})
	accept.Attrs["id"] = cli.generateRequestID()
	if err := cli.sendNode(ctx, accept); err != nil {
		cli.Log.Warnf("Failed to send call accept, call_id: %s: %v", meta.CallID, err)
	}
}

func (cli *Client) captureCallRelay(cs *callState, node *waBinary.Node) {
	direction := types.CallDirectionIncoming
	if cs.outgoing {
		direction = types.CallDirectionOutgoing
	}
	ep := voip.ParseRelay(node, direction)
	if ep == nil {
		return
	}
	cli.callsLock.Lock()
	cs.relay = ep
	cli.callsLock.Unlock()
	cli.maybeEmitMediaReady(cs)
}

func (cli *Client) maybeEmitMediaReady(cs *callState) {
	cli.callsLock.Lock()
	if cs.callKey == nil || cs.relay == nil || cs.mediaReadySent {
		cli.callsLock.Unlock()
		return
	}
	cs.mediaReadySent = true
	meta, self, peer, callKey, relay, codec := cs.meta, cs.selfLID, cs.peerLID, cs.callKey, *cs.relay, cs.codec
	direction := types.CallDirectionIncoming
	if cs.outgoing {
		direction = types.CallDirectionOutgoing
	}
	video := cs.localVideo || cs.remoteVideo
	cli.callsLock.Unlock()

	cli.dispatchEvent(&events.CallMediaReady{
		BasicCallMeta: meta,
		SelfLID:       self,
		PeerLID:       peer,
		CallKey:       callKey,
		Relay:         relay,
		Codec:         codec,
		Direction:     direction,
		Video:         video,
	})
	cli.Log.Debugf("Call media ready, call_id: %s, codec: %s", meta.CallID, codec)
}

func (cli *Client) onCallAccept(meta types.BasicCallMeta, remote types.CallRemoteMeta, child *waBinary.Node) {
	if cs := cli.getCall(meta.CallID); cs != nil {
		cli.callsLock.Lock()
		if !meta.From.IsEmpty() {
			cs.peerLID = meta.From
			cs.to = meta.From
		}
		cli.callsLock.Unlock()
	}
	cli.dispatchEvent(&events.CallAccept{BasicCallMeta: meta, CallRemoteMeta: remote, Data: child})
}

// RejectCall reject an incoming call.
func (cli *Client) RejectCall(ctx context.Context, callFrom types.JID, callID string) error {
	ownID := cli.getOwnID()
	if ownID.IsEmpty() {
		return ErrNotLoggedIn
	}
	ownID, callFrom = ownID.ToNonAD(), callFrom.ToNonAD()
	cs := cli.getCall(callID)
	rejectNode := waBinary.Node{
		Tag:     "reject",
		Attrs:   waBinary.Attrs{"call-id": callID, "call-creator": callFrom, "count": "0"},
		Content: nil,
	}
	if token, err := cli.ensureTCToken(ctx, callFrom); err != nil {
		cli.Log.Warnf("Failed to get privacy token for call reject to %s: %v", callFrom, err)
	} else if len(token) > 0 {
		rejectNode.Content = []waBinary.Node{{
			Tag:     "tctoken",
			Content: token,
		}}
	}
	sendErr := cli.sendNode(ctx, waBinary.Node{
		Tag:     "call",
		Attrs:   waBinary.Attrs{"id": cli.GenerateMessageID(), "from": ownID, "to": callFrom},
		Content: []waBinary.Node{rejectNode},
	})
	if cs != nil {
		cli.dispatchEvent(&events.CallMediaStop{BasicCallMeta: cs.meta, Reason: "rejected"})
	}
	if sendErr != nil {
		return sendErr
	}
	cli.dropCall(callID)
	return nil
}
