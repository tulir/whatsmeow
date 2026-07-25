// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/voip"
)

type callState struct {
	meta                  types.BasicCallMeta
	selfLID, peerLID      types.JID
	to, creator           types.JID
	outgoing              bool
	callKey               []byte
	relay                 *types.RelayEndpoint
	codec                 types.CallCodec
	acceptPending         bool
	acceptInFlight        bool
	mediaReadySent        bool
	localVideo            bool
	remoteVideo           bool
	group                 *groupCallState
	groupKeyTransactionID uint32
	hasGroupKeyEpoch      bool
	connected             bool
	inviteSelfDevice      types.GroupCallDevice
	invitePeerDevice      types.GroupCallDevice
}

// CallOfferOptions configures a new outgoing 1:1 call.
type CallOfferOptions struct {
	Video bool
}

func (cli *Client) getCall(callID string) *callState {
	cli.callsLock.Lock()
	defer cli.callsLock.Unlock()
	return cli.calls[callID]
}

func (cli *Client) putCall(callID string, cs *callState) {
	cli.callsLock.Lock()
	defer cli.callsLock.Unlock()
	cli.calls[callID] = cs
}

func (cli *Client) dropCall(callID string) {
	cli.callsLock.Lock()
	defer cli.callsLock.Unlock()
	delete(cli.calls, callID)
}

func newCallID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return strings.ToUpper(hex.EncodeToString(b[:]))
}

func (cli *Client) resolvePeerCallLID(ctx context.Context, target types.JID) (types.JID, error) {
	if target.Server == types.HiddenUserServer {
		return target, nil
	}
	if lid, err := cli.Store.LIDs.GetLIDForPN(ctx, target); err == nil && !lid.IsEmpty() {
		return lid, nil
	}
	info, err := cli.GetUserInfo(ctx, []types.JID{target})
	if err != nil {
		return types.EmptyJID, fmt.Errorf("whatsmeow: usync %s: %w", target.User, err)
	}
	for _, ui := range info {
		if !ui.LID.IsEmpty() {
			return ui.LID, nil
		}
	}
	if lid, err := cli.Store.LIDs.GetLIDForPN(ctx, target); err == nil && !lid.IsEmpty() {
		return lid, nil
	}
	return types.EmptyJID, fmt.Errorf("whatsmeow: usync returned no LID for %s (peer unreachable or not on WhatsApp)", target.User)
}

func (cli *Client) composeOffer(callID string, self, peer types.JID, deviceKeys []voip.DeviceKey, privacyToken, deviceIdentity []byte, video bool) waBinary.Node {
	offer := voip.BuildOffer(&voip.OfferParams{
		CallID:         callID,
		To:             peer,
		CallCreator:    self,
		DeviceKeys:     deviceKeys,
		PrivacyToken:   privacyToken,
		Capability:     voip.CapabilityOffer,
		DeviceIdentity: deviceIdentity,
		Video:          video,
	})
	offer.Attrs["id"] = cli.GenerateMessageID()
	return offer
}

func (cli *Client) newOutgoingCallState(callID string, self, peer types.JID, callKey []byte, video bool) *callState {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/1ebd064663ac336ff3d1fc65d9baa974148fe73e/datasheets/voip-group-participant-invite.md#L117-L121
	return &callState{
		meta: types.BasicCallMeta{
			From:        self,
			Timestamp:   time.Now(),
			CallCreator: self,
			CallID:      callID,
		},
		selfLID:     self,
		peerLID:     peer,
		to:          peer,
		creator:     self,
		outgoing:    true,
		callKey:     callKey,
		localVideo:  video,
		remoteVideo: video,
		inviteSelfDevice: types.GroupCallDevice{
			JID:               self,
			CapabilityVersion: 1,
			Capability:        bytes.Clone(voip.CapabilityOffer),
		},
	}
}

// OfferCall places a 1:1 call to target.
func (cli *Client) OfferCall(ctx context.Context, target types.JID, options ...CallOfferOptions) (callID string, err error) {
	var opts CallOfferOptions
	if len(options) > 0 {
		opts = options[0]
	}
	self := cli.getOwnLID()
	if self.IsEmpty() {
		return "", ErrNotLoggedIn
	}
	peer, err := cli.resolvePeerCallLID(ctx, target)
	if err != nil {
		return "", err
	}
	devices, err := cli.GetUserDevices(ctx, []types.JID{peer})
	if err != nil {
		return "", fmt.Errorf("whatsmeow: call device discovery: %w", err)
	}
	if len(devices) == 0 {
		return "", fmt.Errorf("whatsmeow: peer %s has no devices (unreachable / not on WhatsApp)", peer)
	}

	callKey := make([]byte, 32)
	if _, err = rand.Read(callKey); err != nil {
		return "", err
	}
	deviceKeys, deviceIdentity, err := cli.encryptCallKeyForDevices(ctx, devices, callKey)
	if err != nil {
		return "", err
	}

	var privacyToken []byte
	if pt, ptErr := cli.Store.PrivacyTokens.GetPrivacyToken(ctx, peer); ptErr == nil && pt != nil {
		privacyToken = pt.Token
	}

	callID = newCallID()
	offer := cli.composeOffer(callID, self, peer, deviceKeys, privacyToken, deviceIdentity, opts.Video)

	cli.putCall(callID, cli.newOutgoingCallState(callID, self, peer, callKey, opts.Video))

	if err = cli.sendNode(ctx, offer); err != nil {
		return callID, fmt.Errorf("whatsmeow: send call offer: %w", err)
	}
	cli.Log.Debugf("Sent call offer, call_id: %s, peer_lid: %s, device_count: %d", callID, peer, len(devices))
	return callID, nil
}

// AcceptCall accepts an inbound call.
func (cli *Client) AcceptCall(ctx context.Context, callID string) error {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/676ebee3eca513b5348fab36cae5c560cc791238/datasheets/voip-group-invite-accept.md#L74-L104
	return cli.acceptCallWithDependencies(ctx, callID, cli.generateRequestID, cli.sendNode)
}

func (cli *Client) acceptCallWithDependencies(
	ctx context.Context,
	callID string,
	requestID func() string,
	send func(context.Context, waBinary.Node) error,
) error {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/676ebee3eca513b5348fab36cae5c560cc791238/datasheets/voip-group-invite-accept.md#L74-L104
	if cli == nil {
		return ErrClientIsNil
	}
	cli.callsLock.Lock()
	cs := cli.calls[callID]
	if cs == nil {
		cli.callsLock.Unlock()
		return fmt.Errorf("whatsmeow: unknown call %s", callID)
	}
	if cs.group == nil {
		cs.acceptPending = true
		cli.callsLock.Unlock()
		return nil
	}
	if cs.connected || cs.acceptInFlight {
		cli.callsLock.Unlock()
		return nil
	}
	cs.acceptPending = false
	cs.acceptInFlight = true
	creator := cs.creator
	cli.callsLock.Unlock()

	if requestID == nil || send == nil {
		cli.clearCallAcceptInFlight(callID, cs)
		return fmt.Errorf("whatsmeow: accept active group call: incomplete dependencies")
	}
	accept := buildImmediateGroupCallAccept(callID, creator, requestID())
	if err := send(ctx, accept); err != nil {
		cli.clearCallAcceptInFlight(callID, cs)
		return fmt.Errorf("whatsmeow: send active group call accept: %w", err)
	}
	if !cli.markCallConnected(callID, cs) {
		return fmt.Errorf("whatsmeow: active group call state changed while sending accept")
	}
	return nil
}

func (cli *Client) clearCallAcceptInFlight(callID string, expected *callState) {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/676ebee3eca513b5348fab36cae5c560cc791238/datasheets/voip-group-invite-accept.md#L74-L104
	cli.callsLock.Lock()
	defer cli.callsLock.Unlock()
	if cli.calls[callID] == expected {
		expected.acceptInFlight = false
	}
}

func buildImmediateGroupCallAccept(callID string, creator types.JID, requestID string) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/676ebee3eca513b5348fab36cae5c560cc791238/datasheets/voip-group-invite-accept.md#L26-L45
	accept := voip.BuildAccept(&voip.AcceptParams{
		CallID:      callID,
		To:          types.NewJID(callID, "call"),
		CallCreator: creator,
		AudioRates:  []string{"16000"},
	})
	accept.Attrs["id"] = requestID
	return accept
}

// HangupCall ends callID (either call direction).
func (cli *Client) HangupCall(ctx context.Context, callID string) error {
	cs := cli.getCall(callID)
	if cs == nil {
		return fmt.Errorf("whatsmeow: unknown call %s", callID)
	}
	term := buildCallTerminate(cs, callID, cli.GenerateMessageID())
	sendErr := cli.sendNode(ctx, term)
	cli.dispatchEvent(&events.CallMediaStop{BasicCallMeta: cs.meta, Reason: "hangup"})
	if sendErr != nil {
		return fmt.Errorf("whatsmeow: send call terminate: %w", sendErr)
	}
	cli.dropCall(callID)
	cli.Log.Debugf("Sent call terminate, call_id: %s", callID)
	return nil
}

func buildCallTerminate(cs *callState, callID, requestID string) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L35-L39
	term := voip.BuildTerminate(&voip.TerminateParams{
		CallID: callID, To: cs.signalingTarget(), CallCreator: cs.creator,
	})
	term.Attrs["id"] = requestID
	return term
}

// SetCallMute sends our local mute-state change for callID.
func (cli *Client) SetCallMute(ctx context.Context, callID string, muted bool) error {
	cs := cli.getCall(callID)
	if cs == nil {
		return fmt.Errorf("whatsmeow: unknown call %s", callID)
	}
	state := "0"
	if muted {
		state = "1"
	}
	mute := buildCallMute(cs, callID, cli.GenerateMessageID(), state)
	if err := cli.sendNode(ctx, mute); err != nil {
		return fmt.Errorf("whatsmeow: send call mute: %w", err)
	}
	cli.Log.Debugf("Sent call mute, call_id: %s, muted: %t", callID, muted)
	return nil
}

func buildCallMute(cs *callState, callID, requestID, state string) waBinary.Node {
	// Source of truth: https://github.com/purpshell/meowcaller/blob/33854919e64bdd4b053054ac9764d8fc63027b57/datasheets/voip-group-invite-accept.md#L35-L39
	mute := voip.BuildMute(callID, cs.signalingTarget(), cs.creator, state)
	mute.Attrs["id"] = requestID
	return mute
}
