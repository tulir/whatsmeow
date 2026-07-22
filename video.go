// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"fmt"
	"strconv"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/voip"
)

func (cli *Client) sendCallVideoAck(ctx context.Context, node *waBinary.Node) {
	ack, ok := voip.BuildVideoAck(node)
	if !ok {
		cli.Log.Warnf("Failed to build call video acknowledgement")
		return
	}
	if err := cli.sendNode(ctx, ack); err != nil {
		cli.Log.Warnf("Failed to send call video acknowledgement: %v", err)
	}
}

func (cli *Client) onCallVideo(meta types.BasicCallMeta, node *waBinary.Node) {
	ag := node.AttrGetter()
	stateValue, err := strconv.Atoi(ag.String("state"))
	if err != nil {
		cli.Log.Warnf("Invalid call video state, call_id: %s: %v", meta.CallID, err)
		return
	}
	orientation, orientationErr := strconv.Atoi(ag.String("device_orientation"))
	hasOrientation := orientationErr == nil
	state := types.CallVideoState(stateValue)

	if cs := cli.getCall(meta.CallID); cs != nil {
		cli.callsLock.Lock()
		switch state {
		case types.CallVideoStateEnabled:
			cs.remoteVideo = true
		case types.CallVideoStateDisabled, types.CallVideoStateStopped:
			cs.remoteVideo = false
		}
		cli.callsLock.Unlock()
	}

	cli.dispatchEvent(&events.CallVideo{
		BasicCallMeta:  meta,
		State:          state,
		Orientation:    orientation,
		HasOrientation: hasOrientation,
		Data:           node,
	})
}

// SetCallVideo sends one independent local video-flow transition for callID.
func (cli *Client) SetCallVideo(ctx context.Context, callID string, state types.CallVideoState, orientation *int) error {
	if orientation != nil && (*orientation < 0 || *orientation > 3) {
		return fmt.Errorf("whatsmeow: call video orientation %d is outside 0..3", *orientation)
	}
	cs := cli.getCall(callID)
	if cs == nil {
		return fmt.Errorf("whatsmeow: unknown call %s", callID)
	}
	node := voip.BuildVideoState(callID, cs.to, cs.creator, cli.generateRequestID(), state, orientation)
	if err := cli.sendNode(ctx, node); err != nil {
		return fmt.Errorf("whatsmeow: send call video state: %w", err)
	}
	cli.callsLock.Lock()
	switch state {
	case types.CallVideoStateUpgradeRequest, types.CallVideoStateUpgradeRequestV2, types.CallVideoStateEnabled:
		cs.localVideo = true
	case types.CallVideoStateDisabled, types.CallVideoStateStopped, types.CallVideoStateUpgradeCancel:
		cs.localVideo = false
	}
	cli.callsLock.Unlock()
	return nil
}
