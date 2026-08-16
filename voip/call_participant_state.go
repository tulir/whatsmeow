package voip

import (
	"fmt"
	"strconv"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

// BuildRaiseHand builds one persistent participant hand-state transition.
func BuildRaiseHand(
	callID string,
	to, creator types.JID,
	requestID string,
	raised bool,
) waBinary.Node {
	state := "0"
	if raised {
		state = "1"
	}
	return callWrap(to, &requestID, waBinary.Node{
		Tag: "user_action",
		Attrs: waBinary.Attrs{
			"call-id":      callID,
			"call-creator": creator,
			"action":       "raise_hand",
		},
		Content: []waBinary.Node{{
			Tag:   "raise_hand",
			Attrs: waBinary.Attrs{"raise-hand-state": state},
		}},
	})
}

// ParseRaiseHand parses one captured raise/lower action.
func ParseRaiseHand(node *waBinary.Node) (bool, error) {
	if node == nil || node.Tag != "user_action" {
		return false, fmt.Errorf("whatsmeow: parse raise hand: unexpected node")
	}
	attrs := node.AttrGetter()
	if attrs.String("action") != "raise_hand" {
		return false, fmt.Errorf("whatsmeow: parse raise hand: unsupported action %q", attrs.OptionalString("action"))
	}
	if attrs.String("call-id") == "" || attrs.JID("call-creator").IsEmpty() {
		return false, fmt.Errorf("whatsmeow: parse raise hand: %w", attrs.Error())
	}
	child, ok := node.GetOptionalChildByTag("raise_hand")
	if !ok {
		return false, fmt.Errorf("whatsmeow: parse raise hand: missing raise_hand")
	}
	switch child.AttrGetter().String("raise-hand-state") {
	case "1":
		return true, nil
	case "0":
		return false, nil
	default:
		return false, fmt.Errorf("whatsmeow: parse raise hand: invalid state")
	}
}

// BuildScreenShare builds one version-2 screen-share transition.
func BuildScreenShare(
	callID string,
	to, creator types.JID,
	requestID string,
	state types.CallScreenShareState,
	screenShareID *uint32,
) waBinary.Node {
	attrs := waBinary.Attrs{
		"call-id":           callID,
		"call-creator":      creator,
		"screenshare_state": strconv.Itoa(int(state)),
		"version":           "2",
	}
	if screenShareID != nil {
		attrs["screen_share_id"] = strconv.FormatUint(uint64(*screenShareID), 10)
	}
	return callWrap(to, &requestID, waBinary.Node{Tag: "screen_share", Attrs: attrs})
}

// ParseScreenShare parses one versioned screen-share transition.
func ParseScreenShare(node *waBinary.Node) (*types.CallScreenShare, error) {
	if node == nil || node.Tag != "screen_share" {
		return nil, fmt.Errorf("whatsmeow: parse screen share: unexpected node")
	}
	attrs := node.AttrGetter()
	stateValue, err := strconv.Atoi(attrs.String("screenshare_state"))
	if err != nil {
		return nil, fmt.Errorf("whatsmeow: parse screen share state: %w", err)
	}
	state := types.CallScreenShareState(stateValue)
	if state != types.CallScreenShareStateStarted && state != types.CallScreenShareStateStopped {
		return nil, fmt.Errorf("whatsmeow: parse screen share: unsupported state %d", state)
	}
	version, err := strconv.ParseUint(attrs.String("version"), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("whatsmeow: parse screen share version: %w", err)
	}
	result := &types.CallScreenShare{State: state, Version: uint32(version)}
	if rawID := attrs.OptionalString("screen_share_id"); rawID != "" {
		id, parseErr := strconv.ParseUint(rawID, 10, 32)
		if parseErr != nil {
			return nil, fmt.Errorf("whatsmeow: parse screen share ID: %w", parseErr)
		}
		result.ScreenShareID = uint32(id)
		result.HasScreenShareID = true
	}
	if err = attrs.Error(); err != nil {
		return nil, fmt.Errorf("whatsmeow: parse screen share: %w", err)
	}
	return result, nil
}

// BuildCallControlAck builds a call-class ACK whose type comes from the child tag.
func BuildCallControlAck(original *waBinary.Node, childTag string) (waBinary.Node, bool) {
	if original == nil || childTag == "" {
		return waBinary.Node{}, false
	}
	attrs := original.AttrGetter()
	id := attrs.String("id")
	from := attrs.JID("from")
	if id == "" || from.IsEmpty() {
		return waBinary.Node{}, false
	}
	ackAttrs := waBinary.Attrs{
		"class": "call",
		"id":    id,
		"to":    from,
		"type":  childTag,
	}
	if participant := attrs.OptionalJIDOrEmpty("participant"); !participant.IsEmpty() && participant != from {
		ackAttrs["participant"] = participant
	}
	if recipient := attrs.OptionalJIDOrEmpty("recipient"); !recipient.IsEmpty() {
		ackAttrs["recipient"] = recipient
	}
	return waBinary.Node{Tag: "ack", Attrs: ackAttrs}, true
}
