package whatsmeow

import (
	"context"
	"fmt"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/voip"
)

// SetCallHandRaised sends this client's persistent raised-hand state.
func (cli *Client) SetCallHandRaised(ctx context.Context, callID string, raised bool) error {
	return cli.setCallHandRaisedWithDependencies(
		ctx,
		callID,
		raised,
		cli.generateRequestID,
		cli.sendNode,
	)
}

func (cli *Client) setCallHandRaisedWithDependencies(
	ctx context.Context,
	callID string,
	raised bool,
	requestID func() string,
	send func(context.Context, waBinary.Node) error,
) error {
	if cli == nil {
		return ErrClientIsNil
	}
	cs := cli.getCall(callID)
	if cs == nil {
		return fmt.Errorf("whatsmeow: unknown call %s", callID)
	}
	if requestID == nil || send == nil {
		return fmt.Errorf("whatsmeow: set call hand state: incomplete dependencies")
	}
	node := voip.BuildRaiseHand(
		callID,
		cs.signalingTarget(),
		cs.creator,
		requestID(),
		raised,
	)
	if err := send(ctx, node); err != nil {
		return fmt.Errorf("whatsmeow: send call hand state: %w", err)
	}
	participant := cs.selfLID.ToNonAD()
	cli.callsLock.Lock()
	if cs.raisedHands == nil {
		cs.raisedHands = make(map[types.JID]bool)
	}
	cs.raisedHands[participant] = raised
	cli.callsLock.Unlock()
	return nil
}

// SetCallScreenShare sends an independent version-2 screen-share transition.
func (cli *Client) SetCallScreenShare(
	ctx context.Context,
	callID string,
	state types.CallScreenShareState,
	screenShareID *uint32,
) error {
	if state != types.CallScreenShareStateStarted && state != types.CallScreenShareStateStopped {
		return fmt.Errorf("whatsmeow: unsupported screen-share state %d", state)
	}
	cs := cli.getCall(callID)
	if cs == nil {
		return fmt.Errorf("whatsmeow: unknown call %s", callID)
	}
	node := voip.BuildScreenShare(
		callID,
		cs.signalingTarget(),
		cs.creator,
		cli.generateRequestID(),
		state,
		screenShareID,
	)
	if err := cli.sendNode(ctx, node); err != nil {
		return fmt.Errorf("whatsmeow: send screen-share state: %w", err)
	}
	participant := cs.selfLID.ToNonAD()
	cli.callsLock.Lock()
	if cs.screenShares == nil {
		cs.screenShares = make(map[types.JID]types.CallScreenShare)
	}
	if state == types.CallScreenShareStateStopped {
		delete(cs.screenShares, participant)
	} else {
		cs.screenShares[participant] = types.CallScreenShare{
			State:            state,
			Version:          2,
			ScreenShareID:    valueOrZero(screenShareID),
			HasScreenShareID: screenShareID != nil,
		}
	}
	cli.callsLock.Unlock()
	return nil
}

func valueOrZero(value *uint32) uint32 {
	if value == nil {
		return 0
	}
	return *value
}

func (cli *Client) handleCallParticipantState(
	ctx context.Context,
	node *waBinary.Node,
	send func(context.Context, waBinary.Node) error,
) {
	children := node.GetChildren()
	if len(children) != 1 {
		cli.dispatchEvent(&events.UnknownCallEvent{Node: node})
		return
	}
	child := &children[0]
	attrs := node.AttrGetter()
	childAttrs := child.AttrGetter()
	meta := types.BasicCallMeta{
		From:        attrs.JID("from"),
		Timestamp:   attrs.UnixTime("t"),
		CallID:      childAttrs.String("call-id"),
		CallCreator: childAttrs.JID("call-creator"),
	}
	if ack, ok := voip.BuildCallControlAck(node, child.Tag); ok {
		if err := send(ctx, ack); err != nil {
			cli.Log.Warnf("Failed to send %s acknowledgement, call_id: %s: %v", child.Tag, meta.CallID, err)
		}
	}
	participant := meta.From.ToNonAD()
	switch child.Tag {
	case "user_action":
		raised, err := voip.ParseRaiseHand(child)
		if err != nil {
			cli.Log.Warnf("Failed to parse call hand state, call_id: %s: %v", meta.CallID, err)
			return
		}
		if cs := cli.getCall(meta.CallID); cs != nil {
			cli.callsLock.Lock()
			if cs.raisedHands == nil {
				cs.raisedHands = make(map[types.JID]bool)
			}
			cs.raisedHands[participant] = raised
			cli.callsLock.Unlock()
		}
		cli.dispatchEvent(&events.CallHandRaise{
			BasicCallMeta: meta,
			Participant:   participant,
			Raised:        raised,
			Data:          child,
		})
	case "screen_share":
		screenShare, err := voip.ParseScreenShare(child)
		if err != nil {
			cli.Log.Warnf("Failed to parse call screen share, call_id: %s: %v", meta.CallID, err)
			return
		}
		if cs := cli.getCall(meta.CallID); cs != nil {
			cli.callsLock.Lock()
			if cs.screenShares == nil {
				cs.screenShares = make(map[types.JID]types.CallScreenShare)
			}
			if screenShare.State == types.CallScreenShareStateStopped {
				delete(cs.screenShares, participant)
			} else {
				cs.screenShares[participant] = *screenShare
			}
			cli.callsLock.Unlock()
		}
		cli.dispatchEvent(&events.CallScreenShare{
			BasicCallMeta:   meta,
			CallScreenShare: *screenShare,
			Participant:     participant,
			Data:            child,
		})
	}
}

func (cli *Client) clearDepartedParticipantState(update types.GroupCallUpdate) {
	connected := make(map[types.JID]struct{}, len(update.Participants))
	for _, participant := range update.Participants {
		if participant.State == "connected" {
			connected[participant.JID.ToNonAD()] = struct{}{}
		}
	}
	cli.callsLock.Lock()
	cs := cli.calls[update.CallID]
	if cs == nil {
		cli.callsLock.Unlock()
		return
	}
	for participant := range cs.raisedHands {
		if _, ok := connected[participant]; !ok {
			delete(cs.raisedHands, participant)
		}
	}
	var stopped []types.JID
	for participant := range cs.screenShares {
		if _, ok := connected[participant]; !ok {
			delete(cs.screenShares, participant)
			stopped = append(stopped, participant)
		}
	}
	meta := cs.meta
	cli.callsLock.Unlock()
	for _, participant := range stopped {
		cli.dispatchEvent(&events.CallScreenShare{
			BasicCallMeta: meta,
			CallScreenShare: types.CallScreenShare{
				State:   types.CallScreenShareStateStopped,
				Version: 2,
			},
			Participant: participant,
			Synthetic:   true,
		})
	}
}
