package whatsmeow

import (
	"bytes"
	"context"
	"fmt"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/voip"
)

type callLinkRequest func(context.Context, waBinary.Node, string) (*waBinary.Node, error)

func createCallLinkWithDependencies(
	ctx context.Context,
	media types.CallLinkMedia,
	requestID func() string,
	request callLinkRequest,
) (*types.CallLink, error) {
	if requestID == nil || request == nil {
		return nil, fmt.Errorf("whatsmeow: create call link: incomplete dependencies")
	}
	id := requestID()
	node, err := voip.BuildCallLinkCreate(media, id)
	if err != nil {
		return nil, err
	}
	reply, err := request(ctx, node, id)
	if err != nil {
		return nil, err
	}
	if err = validateCallLinkResponse(reply, id, "link_create"); err != nil {
		return nil, err
	}
	return voip.ParseCallLinkCreateAck(reply)
}

// CreateCallLink creates a reusable WhatsApp call-link token.
func (cli *Client) CreateCallLink(
	ctx context.Context,
	media types.CallLinkMedia,
) (*types.CallLink, error) {
	if cli == nil {
		return nil, ErrClientIsNil
	}
	return createCallLinkWithDependencies(ctx, media, cli.generateRequestID, cli.sendCallLinkRequest)
}

// PreviewCallLink queries current metadata for a reusable call-link token.
func (cli *Client) PreviewCallLink(
	ctx context.Context,
	token string,
	media types.CallLinkMedia,
) (*types.CallLinkPreview, error) {
	if cli == nil {
		return nil, ErrClientIsNil
	}
	id := cli.generateRequestID()
	node, err := voip.BuildCallLinkQuery(token, media, id)
	if err != nil {
		return nil, err
	}
	reply, err := cli.sendCallLinkRequest(ctx, node, id)
	if err != nil {
		return nil, err
	}
	if err = validateCallLinkResponse(reply, id, "link_query"); err != nil {
		return nil, err
	}
	return voip.ParseCallLinkQueryAck(reply)
}

// JoinCallLink joins immediately or enters the current active call's waiting room.
func (cli *Client) JoinCallLink(
	ctx context.Context,
	token string,
	media types.CallLinkMedia,
) (*types.CallLinkJoin, error) {
	if cli == nil {
		return nil, ErrClientIsNil
	}
	self := cli.getOwnLID()
	if self.IsEmpty() {
		return nil, ErrNotLoggedIn
	}
	id := cli.generateRequestID()
	node, err := voip.BuildCallLinkJoin(token, media, id)
	if err != nil {
		return nil, err
	}
	reply, err := cli.sendCallLinkRequest(ctx, node, id)
	if err != nil {
		return nil, err
	}
	if err = validateCallLinkResponse(reply, id, "link_join"); err != nil {
		return nil, err
	}
	join, err := voip.ParseCallLinkJoinAck(reply)
	if err != nil {
		return nil, err
	}
	state := newCallLinkState(self, join)
	cli.putCall(join.CallID, state)
	cli.applyVoipSettingsCodec(state, reply, join.CallID)

	if join.Group != nil {
		update := cloneGroupCallUpdate(*join.Group)
		meta := state.meta
		meta.GroupJID = update.GroupJID
		cli.dispatchEvent(&events.CallGroupUpdate{
			BasicCallMeta: meta,
			Update:        update,
			Data:          reply,
		})
		if update.RekeyRequested {
			if err = cli.distributeRequestedGroupEpoch(ctx, meta, update); err != nil {
				cli.dropCall(join.CallID)
				return nil, fmt.Errorf("whatsmeow: distribute call-link group key epoch: %w", err)
			}
		}
		cli.maybeEmitMediaReady(state)
		return join, nil
	}

	cancel, err := startWaitingRoomHeartbeat(
		ctx,
		state,
		10*time.Second,
		cli.generateRequestID,
		cli.sendNode,
	)
	if err != nil {
		cli.dropCall(join.CallID)
		return nil, err
	}
	cli.callsLock.Lock()
	if cli.calls[join.CallID] == state && state.inWaitingRoom {
		state.waitingRoomCancel = cancel
		cancel = nil
	}
	cli.callsLock.Unlock()
	if cancel != nil {
		cancel()
	}
	room := cloneCallLinkWaitingRoom(*state.waitingRoom)
	cli.dispatchEvent(&events.CallWaitingRoomUpdate{
		BasicCallMeta: state.meta,
		WaitingRoom:   room,
		Data:          reply,
	})
	return join, nil
}

func newCallLinkState(self types.JID, join *types.CallLinkJoin) *callState {
	video := join.Media == types.CallLinkMediaVideo
	state := &callState{
		meta: types.BasicCallMeta{
			From:        join.CallCreator,
			Timestamp:   time.Now(),
			CallCreator: join.CallCreator,
			CallID:      join.CallID,
		},
		selfLID:       self,
		to:            join.CallCreator,
		creator:       join.CallCreator,
		outgoing:      true,
		connected:     !join.InWaitingRoom,
		localVideo:    video,
		remoteVideo:   video,
		linkToken:     join.Token,
		inWaitingRoom: join.InWaitingRoom,
		raisedHands:   make(map[types.JID]bool),
		screenShares:  make(map[types.JID]types.CallScreenShare),
		inviteSelfDevice: types.GroupCallDevice{
			JID:               self,
			CapabilityVersion: 1,
			Capability:        bytes.Clone(voip.CapabilityOffer),
		},
		waitingRoom: &types.CallLinkWaitingRoom{
			CallID:      join.CallID,
			CallCreator: join.CallCreator,
			LinkToken:   join.Token,
			Media:       join.Media,
			Enabled:     join.WaitingRoomEnabled,
			IsAdmin:     join.IsAdmin,
		},
	}
	if join.Group != nil {
		group := cloneGroupCallUpdate(*join.Group)
		state.group = &groupCallState{snapshot: group}
		state.peerLID = firstGroupPeer(self, group)
	}
	return state
}

func firstGroupPeer(self types.JID, update types.GroupCallUpdate) types.JID {
	self = self.ToNonAD()
	for _, participant := range update.Participants {
		if participant.State != "connected" || participant.JID.ToNonAD() == self {
			continue
		}
		for _, device := range participant.Devices {
			if !device.JID.IsEmpty() {
				return device.JID
			}
		}
		return participant.JID
	}
	return types.EmptyJID
}

func startWaitingRoomHeartbeat(
	ctx context.Context,
	state *callState,
	interval time.Duration,
	requestID func() string,
	send func(context.Context, waBinary.Node) error,
) (context.CancelFunc, error) {
	if state == nil || interval <= 0 || requestID == nil || send == nil {
		return nil, fmt.Errorf("whatsmeow: start waiting-room heartbeat: incomplete dependencies")
	}
	sendHeartbeat := func(sendCtx context.Context) error {
		node := voip.BuildWaitingRoomHeartbeat(state.meta.CallID, state.creator, requestID())
		if err := send(sendCtx, node); err != nil {
			return fmt.Errorf("whatsmeow: send waiting-room heartbeat: %w", err)
		}
		return nil
	}
	if err := sendHeartbeat(ctx); err != nil {
		return nil, err
	}
	heartbeatCtx, rawCancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				if heartbeatCtx.Err() != nil {
					return
				}
				if err := sendHeartbeat(heartbeatCtx); err != nil {
					return
				}
			}
		}
	}()
	cancel := func() {
		rawCancel()
		<-done
	}
	return cancel, nil
}

func (cli *Client) sendCallLinkRequest(
	ctx context.Context,
	node waBinary.Node,
	requestID string,
) (*waBinary.Node, error) {
	if requestID == "" {
		return nil, fmt.Errorf("whatsmeow: call-link request ID is required")
	}
	waiter := cli.waitResponse(requestID)
	remove := func() {
		cli.responseWaitersLock.Lock()
		if current, ok := cli.responseWaiters[requestID]; ok && current == waiter {
			delete(cli.responseWaiters, requestID)
		}
		cli.responseWaitersLock.Unlock()
	}
	if err := cli.sendNode(ctx, node); err != nil {
		remove()
		return nil, fmt.Errorf("whatsmeow: send call-link request: %w", err)
	}
	timer := time.NewTimer(defaultRequestTimeout)
	defer timer.Stop()
	select {
	case response := <-waiter:
		if response == nil {
			return nil, fmt.Errorf("whatsmeow: call-link request disconnected")
		}
		return response, nil
	case <-ctx.Done():
		remove()
		return nil, ctx.Err()
	case <-timer.C:
		remove()
		return nil, fmt.Errorf("whatsmeow: call-link request timed out")
	}
}

func validateCallLinkResponse(node *waBinary.Node, requestID, expectedType string) error {
	if node == nil {
		return fmt.Errorf("whatsmeow: %s response is nil", expectedType)
	}
	attrs := node.AttrGetter()
	if node.Tag != "ack" ||
		attrs.OptionalString("id") != requestID ||
		attrs.OptionalString("class") != "call" ||
		attrs.OptionalString("type") != expectedType {
		return fmt.Errorf("whatsmeow: unexpected %s response", expectedType)
	}
	if code := attrs.OptionalString("error"); code != "" {
		return fmt.Errorf("whatsmeow: %s rejected with error %s", expectedType, code)
	}
	return nil
}

// SetCallLinkApproval changes whether the active call requires administrator approval.
func (cli *Client) SetCallLinkApproval(ctx context.Context, callID string, enabled bool) error {
	return cli.sendCallLinkAdminControl(ctx, callID, "waiting_room_toggle", types.EmptyJID, enabled)
}

// AdmitCallLinkParticipant admits one waiting user.
func (cli *Client) AdmitCallLinkParticipant(ctx context.Context, callID string, user types.JID) error {
	return cli.sendCallLinkAdminControl(ctx, callID, "waiting_room_admit", user, false)
}

// DenyCallLinkParticipant denies one waiting user.
func (cli *Client) DenyCallLinkParticipant(ctx context.Context, callID string, user types.JID) error {
	return cli.sendCallLinkAdminControl(ctx, callID, "waiting_room_deny", user, false)
}

func (cli *Client) sendCallLinkAdminControl(
	ctx context.Context,
	callID, action string,
	user types.JID,
	enabled bool,
) error {
	if cli == nil {
		return ErrClientIsNil
	}
	cli.callsLock.Lock()
	state := cli.calls[callID]
	if state == nil || state.waitingRoom == nil {
		cli.callsLock.Unlock()
		return fmt.Errorf("whatsmeow: unknown call-link call %s", callID)
	}
	if !state.waitingRoom.IsAdmin {
		cli.callsLock.Unlock()
		return fmt.Errorf("whatsmeow: call-link control requires an administrator")
	}
	creator := state.creator
	cli.callsLock.Unlock()

	id := cli.generateRequestID()
	var node waBinary.Node
	switch action {
	case "waiting_room_toggle":
		node = voip.BuildWaitingRoomToggle(callID, creator, enabled, id)
	case "waiting_room_admit":
		if user.IsEmpty() {
			return fmt.Errorf("whatsmeow: waiting-room user is required")
		}
		node = voip.BuildWaitingRoomAdmit(callID, creator, user.ToNonAD(), id)
	case "waiting_room_deny":
		if user.IsEmpty() {
			return fmt.Errorf("whatsmeow: waiting-room user is required")
		}
		node = voip.BuildWaitingRoomDeny(callID, creator, user.ToNonAD(), id)
	default:
		return fmt.Errorf("whatsmeow: unsupported call-link control %q", action)
	}
	reply, err := cli.sendCallLinkRequest(ctx, node, id)
	if err != nil {
		return err
	}
	if err = validateCallLinkResponse(reply, id, action); err != nil {
		return err
	}
	if action == "waiting_room_toggle" {
		cli.callsLock.Lock()
		if current := cli.calls[callID]; current == state && current.waitingRoom != nil {
			current.waitingRoom.Enabled = enabled
		}
		cli.callsLock.Unlock()
	}
	return nil
}

func (cli *Client) handleCallWaitingRoomUpdate(
	ctx context.Context,
	node *waBinary.Node,
	send func(context.Context, waBinary.Node) error,
) {
	children := node.GetChildren()
	if len(children) != 1 {
		cli.dispatchEvent(&events.UnknownCallEvent{Node: node})
		return
	}
	room, err := voip.ParseWaitingRoomUpdate(&children[0])
	if err != nil {
		cli.Log.Warnf("Failed to parse call waiting-room update: %v", err)
		return
	}
	ack, ackErr := voip.BuildWaitingRoomUpdateAck(node)
	if ackErr != nil {
		cli.Log.Warnf("Failed to build call waiting-room acknowledgement, call_id: %s: %v", room.CallID, ackErr)
	} else if err = send(ctx, ack); err != nil {
		cli.Log.Warnf("Failed to send call waiting-room acknowledgement, call_id: %s: %v", room.CallID, err)
	}
	copyRoom := cloneCallLinkWaitingRoom(*room)
	cli.callsLock.Lock()
	state := cli.calls[room.CallID]
	if state != nil {
		if state.waitingRoom != nil &&
			state.waitingRoom.TransactionID != 0 &&
			room.TransactionID <= state.waitingRoom.TransactionID {
			cli.callsLock.Unlock()
			return
		}
		state.waitingRoom = &copyRoom
	}
	cli.callsLock.Unlock()
	meta := types.BasicCallMeta{
		From:        node.AttrGetter().OptionalJIDOrEmpty("from"),
		Timestamp:   node.AttrGetter().UnixTime("t"),
		CallCreator: room.CallCreator,
		CallID:      room.CallID,
	}
	if state != nil {
		meta = state.meta
	}
	cli.dispatchEvent(&events.CallWaitingRoomUpdate{
		BasicCallMeta: meta,
		WaitingRoom:   cloneCallLinkWaitingRoom(copyRoom),
		Data:          &children[0],
	})
}

func cloneCallLinkWaitingRoom(room types.CallLinkWaitingRoom) types.CallLinkWaitingRoom {
	room.Users = append([]types.CallLinkWaitingRoomUser(nil), room.Users...)
	return room
}
