// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
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
	if children[0].Tag == "waiting_room_update" {
		cli.handleCallWaitingRoomUpdate(ctx, node, cli.sendNode)
		return
	} else if children[0].Tag == "user_action" || children[0].Tag == "screen_share" {
		cli.handleCallParticipantState(ctx, node, cli.sendNode)
		return
	} else if children[0].Tag == "video" {
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
		// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
		if err := cli.capturePeerInviteDevice(basicMeta.CallID, basicMeta.From, &child); err != nil {
			cli.Log.Warnf("Failed to capture peer invite device, call_id: %s: %v", basicMeta.CallID, err)
		}
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
	// Source of truth: https://github.com/purpshell/meowcaller/blob/48c2391ce9f7dcc2b3f223f72f1b5f0c627ad943/datasheets/voip-group-update-ingest.md#L112-L129
	case "group_update":
		cli.onCallGroupUpdate(ctx, &child, basicMeta)
	// Source of truth: https://github.com/purpshell/meowcaller/blob/747c6a1b8a0370358ef18bbaa5e029b960c2f836/datasheets/voip-group-enc-rekey-ingest.md#L39-L73
	case "enc_rekey":
		cli.onCallEncRekey(ctx, &child, basicMeta)
	default:
		cli.dispatchEvent(&events.UnknownCallEvent{Node: node})
	}
}

func (cli *Client) onCallOffer(ctx context.Context, child *waBinary.Node, meta types.BasicCallMeta, remote types.CallRemoteMeta) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L21-L54
	cag := child.AttrGetter()
	if cag.OptionalString("is_call_ended") == "1" || cag.OptionalString("terminate_reason") != "" {
		cli.Log.Debugf("Ignoring already-ended call offer, call_id: %s", meta.CallID)
		return
	}

	group, isGroupInvite, err := voip.ParseGroupInviteSnapshot(child)
	if err != nil {
		cli.Log.Warnf("Failed to parse group invite snapshot, call_id: %s: %v", meta.CallID, err)
		return
	}
	var callKey []byte
	if !isGroupInvite {
		callKey, err = cli.decryptIncomingCallKey(ctx, &events.CallOffer{BasicCallMeta: meta, CallRemoteMeta: remote, Data: child})
		if err != nil {
			cli.Log.Warnf("Failed to decrypt call key, call_id: %s: %v", meta.CallID, err)
			return
		}
	}
	cli.acceptInboundOffer(ctx, child, meta, remote, callKey, group)
}

func (cli *Client) acceptInboundOffer(
	ctx context.Context,
	child *waBinary.Node,
	meta types.BasicCallMeta,
	remote types.CallRemoteMeta,
	callKey []byte,
	group ...*types.GroupCallUpdate,
) *callState {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L56-L88
	peer := meta.CallCreator
	if peer.IsEmpty() {
		peer = meta.From
	}
	if relayPeer := voip.ParseRelayPeer(child); !relayPeer.IsEmpty() {
		peer = relayPeer
	}
	peerDevice := meta.From
	if peerDevice.IsEmpty() {
		peerDevice = peer
	}
	invitePeerDevice, invitePeerErr := parseCallInviteDevice(peerDevice, child)
	if invitePeerErr != nil {
		cli.Log.Warnf("Failed to capture inbound peer invite device, call_id: %s: %v", meta.CallID, invitePeerErr)
	}
	self := cli.getOwnLID()
	inviteSelfDevice := types.GroupCallDevice{
		JID:               self,
		CapabilityVersion: 1,
		Capability:        bytes.Clone(voip.CapabilityOffer),
	}
	relay := voip.ParseRelay(child, types.CallDirectionIncoming)
	isVideo := voip.OfferHasVideo(child)
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L190-L191
	var groupSnapshot *types.GroupCallUpdate
	if len(group) > 0 && group[0] != nil {
		cloned := cloneGroupCallUpdate(*group[0])
		groupSnapshot = &cloned
	}

	cs := cli.getCall(meta.CallID)
	isNew := cs == nil
	if isNew {
		cs = &callState{
			meta:             meta,
			selfLID:          self,
			peerLID:          peer,
			to:               meta.From,
			creator:          meta.CallCreator,
			callKey:          callKey,
			relay:            relay,
			localVideo:       isVideo,
			remoteVideo:      isVideo,
			inviteSelfDevice: inviteSelfDevice,
			invitePeerDevice: invitePeerDevice,
		}
		if groupSnapshot != nil {
			cs.group = &groupCallState{snapshot: *groupSnapshot}
		}
		cli.putCall(meta.CallID, cs)
	} else {
		cli.callsLock.Lock()
		if callKey != nil {
			cs.callKey = callKey
		}
		if relay != nil {
			cs.relay = relay
		}
		if !peer.IsEmpty() {
			cs.peerLID = preferQualifiedCallPeer(cs.peerLID, peer)
		}
		cs.inviteSelfDevice = inviteSelfDevice
		if invitePeerErr == nil {
			cs.invitePeerDevice = invitePeerDevice
		}
		if groupSnapshot != nil &&
			(cs.group == nil || groupSnapshot.TransactionID > cs.group.snapshot.TransactionID) {
			cs.group = &groupCallState{snapshot: *groupSnapshot}
		}
		cli.callsLock.Unlock()
	}
	cli.applyVoipSettingsCodec(cs, child, meta.CallID)

	pre := buildInboundPreaccept(cs, meta, cli.generateRequestID(), isVideo)
	if err := cli.sendNode(ctx, pre); err != nil {
		cli.Log.Warnf("Failed to send call preaccept, call_id: %s: %v", meta.CallID, err)
	}

	if isNew {
		var eventGroup *types.GroupCallUpdate
		if groupSnapshot != nil {
			cloned := cloneGroupCallUpdate(*groupSnapshot)
			eventGroup = &cloned
		}
		cli.dispatchEvent(&events.CallOffer{
			BasicCallMeta:  meta,
			CallRemoteMeta: remote,
			Data:           child,
			Video:          isVideo,
			Group:          eventGroup,
		})
	}
	cli.maybeEmitMediaReady(cs)
	return cs
}

func buildInboundPreaccept(cs *callState, meta types.BasicCallMeta, requestID string, video bool) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L35-L39
	target := meta.From
	if cs != nil {
		target = cs.signalingTarget()
	}
	return voip.BuildEagerPreaccept(meta.CallID, target, meta.CallCreator, requestID, video)
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
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
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

	accept := buildDeferredCallAccept(cs, meta, video)
	accept.Attrs["id"] = cli.generateRequestID()
	if err := cli.sendNode(ctx, accept); err != nil {
		cli.Log.Warnf("Failed to send call accept, call_id: %s: %v", meta.CallID, err)
		return
	}
	// NOT VALIDATED: live incoming-call E2E validates the successful deferred accept send boundary.
	cli.markCallConnected(meta.CallID, cs)
}

func buildDeferredCallAccept(cs *callState, meta types.BasicCallMeta, video bool) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L35-L39
	target := meta.From
	if cs != nil {
		target = cs.signalingTarget()
	}
	return voip.BuildAccept(&voip.AcceptParams{
		CallID:      meta.CallID,
		To:          target,
		CallCreator: meta.CallCreator,
		AudioRates:  []string{"16000"},
		Metadata:    waBinary.Attrs{"peer_abtest_bucket_id_list": "125208,94276"},
		Video:       video,
	})
}

func (cli *Client) markCallConnected(callID string, expected *callState) bool {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	cli.callsLock.Lock()
	defer cli.callsLock.Unlock()
	if cli.calls[callID] != expected {
		return false
	}
	expected.connected = true
	return true
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
	peer := voip.ParseRelayPeer(node)
	cli.callsLock.Lock()
	cs.relay = ep
	if !peer.IsEmpty() {
		cs.peerLID = preferQualifiedCallPeer(cs.peerLID, peer)
	}
	cli.callsLock.Unlock()
	cli.maybeEmitMediaReady(cs)
}

func (cli *Client) maybeEmitMediaReady(cs *callState) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L183-L189
	cli.callsLock.Lock()
	if cs.callKey == nil || cs.mediaReadySent {
		cli.callsLock.Unlock()
		return
	}
	peer := cs.peerLID
	var relay types.RelayEndpoint
	if cs.group == nil {
		if cs.relay == nil {
			cli.callsLock.Unlock()
			return
		}
		relay = *cs.relay
	} else {
		// NOT VALIDATED: clear when live group transport emits media readiness after its first installed key epoch.
		var ok bool
		if !cs.hasGroupKeyEpoch || len(cs.callKey) != 32 {
			cli.callsLock.Unlock()
			return
		}
		peer, relay, ok = groupMediaReadyFields(cs.selfLID, cs.group.snapshot)
		if !ok {
			cli.callsLock.Unlock()
			return
		}
	}
	cs.mediaReadySent = true
	meta, self, callKey, codec := cs.meta, cs.selfLID, bytes.Clone(cs.callKey), cs.codec
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
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L36-L72
	peer := meta.From
	if cs := cli.getCall(meta.CallID); cs != nil {
		cli.callsLock.Lock()
		// Source of truth: https://github.com/purpshell/meowcaller/blob/7cb6045001dafd2514f53e85cd8c3e419c13adbe/datasheets/voip-initial-group-call.md#L181-L182
		if cs.group == nil && !meta.From.IsEmpty() {
			cs.peerLID = preferQualifiedCallPeer(cs.peerLID, meta.From)
			cs.to = meta.From
		}
		cs.connected = true
		peer = cs.peerLID
		cli.callsLock.Unlock()
	}
	cli.dispatchEvent(&events.CallAccept{BasicCallMeta: meta, CallRemoteMeta: remote, Data: child, PeerLID: peer})
}

func preferQualifiedCallPeer(current, signaled types.JID) types.JID {
	if signaled.IsEmpty() {
		return current
	}
	if current.User == signaled.User &&
		current.Server == signaled.Server &&
		current.Device != 0 &&
		signaled.Device == 0 {
		return current
	}
	return signaled
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
