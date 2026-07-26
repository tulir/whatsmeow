package voip

import (
	"fmt"
	"strconv"
	"strings"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

func validateCallLinkMedia(media types.CallLinkMedia) error {
	switch media {
	case types.CallLinkMediaAudio, types.CallLinkMediaVideo:
		return nil
	default:
		return fmt.Errorf("whatsmeow: invalid call-link media %q", media)
	}
}

func buildCallServiceRequest(requestID string, action waBinary.Node) (waBinary.Node, error) {
	if requestID == "" {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: call-link request ID is required")
	}
	return waBinary.Node{
		Tag:     "call",
		Attrs:   waBinary.Attrs{"to": types.NewJID("", "call"), "id": requestID},
		Content: []waBinary.Node{action},
	}, nil
}

// BuildCallLinkCreate builds the service request that allocates a reusable link.
func BuildCallLinkCreate(media types.CallLinkMedia, requestID string) (waBinary.Node, error) {
	if err := validateCallLinkMedia(media); err != nil {
		return waBinary.Node{}, err
	}
	return buildCallServiceRequest(requestID, waBinary.Node{
		Tag:   "link_create",
		Attrs: waBinary.Attrs{"media": string(media)},
	})
}

// BuildCallLinkQuery builds the service request that previews a reusable link.
func BuildCallLinkQuery(token string, media types.CallLinkMedia, requestID string) (waBinary.Node, error) {
	if strings.TrimSpace(token) == "" {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: call-link token is required")
	}
	if err := validateCallLinkMedia(media); err != nil {
		return waBinary.Node{}, err
	}
	return buildCallServiceRequest(requestID, waBinary.Node{
		Tag: "link_query",
		Attrs: waBinary.Attrs{
			"token": token,
			"media": string(media),
		},
	})
}

// BuildCallLinkJoin builds the media-capability request that joins a link.
func BuildCallLinkJoin(token string, media types.CallLinkMedia, requestID string) (waBinary.Node, error) {
	if strings.TrimSpace(token) == "" {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: call-link token is required")
	}
	if err := validateCallLinkMedia(media); err != nil {
		return waBinary.Node{}, err
	}
	children := []waBinary.Node{audioOpus("16000")}
	if media == types.CallLinkMediaVideo {
		children = append(children, waBinary.Node{
			Tag: "video",
			Attrs: waBinary.Attrs{
				"dec":                "H264",
				"device_orientation": "0",
			},
		})
	}
	children = append(children,
		waBinary.Node{Tag: "net", Attrs: waBinary.Attrs{"medium": "2"}},
		waBinary.Node{
			Tag:     "capability",
			Attrs:   waBinary.Attrs{"ver": "1"},
			Content: append([]byte(nil), capabilityOffer...),
		},
	)
	return buildCallServiceRequest(requestID, waBinary.Node{
		Tag: "link_join",
		Attrs: waBinary.Attrs{
			"token": token,
			"media": string(media),
		},
		Content: children,
	})
}

func buildWaitingRoomRequest(
	tag, callID string,
	creator types.JID,
	requestID string,
	extraAttrs waBinary.Attrs,
	children []waBinary.Node,
) waBinary.Node {
	attrs := waBinary.Attrs{
		"call-id":      callID,
		"call-creator": creator,
	}
	for key, value := range extraAttrs {
		attrs[key] = value
	}
	return waBinary.Node{
		Tag: "call",
		Attrs: waBinary.Attrs{
			"to": types.NewJID(callID, "call"),
			"id": requestID,
		},
		Content: []waBinary.Node{{
			Tag:     tag,
			Attrs:   attrs,
			Content: children,
		}},
	}
}

// BuildWaitingRoomToggle changes approval requirements for an active link call.
func BuildWaitingRoomToggle(
	callID string,
	creator types.JID,
	enabled bool,
	requestID string,
) waBinary.Node {
	value := "0"
	if enabled {
		value = "1"
	}
	return buildWaitingRoomRequest(
		"waiting_room_toggle",
		callID,
		creator,
		requestID,
		waBinary.Attrs{"enabled": value},
		nil,
	)
}

// BuildWaitingRoomHeartbeat keeps a pending link-join request alive.
func BuildWaitingRoomHeartbeat(callID string, creator types.JID, requestID string) waBinary.Node {
	return buildWaitingRoomRequest(
		"heartbeat",
		callID,
		creator,
		requestID,
		waBinary.Attrs{"type": "waiting_room"},
		nil,
	)
}

// BuildWaitingRoomAdmit admits one pending user to an active link call.
func BuildWaitingRoomAdmit(
	callID string,
	creator, user types.JID,
	requestID string,
) waBinary.Node {
	return buildWaitingRoomRequest(
		"waiting_room_admit",
		callID,
		creator,
		requestID,
		nil,
		[]waBinary.Node{{Tag: "user", Attrs: waBinary.Attrs{"jid": user}}},
	)
}

// BuildWaitingRoomDeny denies one pending user entry to an active link call.
func BuildWaitingRoomDeny(
	callID string,
	creator, user types.JID,
	requestID string,
) waBinary.Node {
	return buildWaitingRoomRequest(
		"waiting_room_deny",
		callID,
		creator,
		requestID,
		nil,
		[]waBinary.Node{{Tag: "user", Attrs: waBinary.Attrs{"jid": user}}},
	)
}

func validateCallLinkAckEnvelope(node *waBinary.Node, expectedType string) error {
	if node == nil {
		return fmt.Errorf("whatsmeow: parse %s ACK: nil node", expectedType)
	}
	attrs := node.AttrGetter()
	if node.Tag != "ack" || attrs.String("class") != "call" ||
		attrs.String("type") != expectedType {
		return fmt.Errorf(
			"whatsmeow: parse %s ACK: unexpected envelope tag=%q class=%q type=%q",
			expectedType,
			node.Tag,
			attrs.OptionalString("class"),
			attrs.OptionalString("type"),
		)
	}
	return nil
}

func validateCallLinkAck(node *waBinary.Node, expectedType string) (*waBinary.Node, error) {
	if err := validateCallLinkAckEnvelope(node, expectedType); err != nil {
		return nil, err
	}
	child, ok := node.GetOptionalChildByTag(expectedType)
	if !ok {
		return nil, fmt.Errorf("whatsmeow: parse %s ACK: missing %s", expectedType, expectedType)
	}
	return &child, nil
}

func parseMedia(value string) (types.CallLinkMedia, error) {
	media := types.CallLinkMedia(value)
	if err := validateCallLinkMedia(media); err != nil {
		return "", err
	}
	return media, nil
}

func parseBoolAttr(attrs *waBinary.AttrUtility, name string) bool {
	return attrs.OptionalString(name) == "1"
}

// ParseCallLinkCreateAck parses the token returned by link creation.
func ParseCallLinkCreateAck(node *waBinary.Node) (*types.CallLink, error) {
	child, err := validateCallLinkAck(node, "link_create")
	if err != nil {
		return nil, err
	}
	attrs := child.AttrGetter()
	media, err := parseMedia(attrs.String("media"))
	if err != nil {
		return nil, err
	}
	link := &types.CallLink{
		Token: attrs.String("token"),
		Media: media,
	}
	if attrErr := attrs.Error(); attrErr != nil {
		return nil, fmt.Errorf("whatsmeow: parse link_create ACK: %w", attrErr)
	}
	if link.Token == "" {
		return nil, fmt.Errorf("whatsmeow: parse link_create ACK: empty token")
	}
	return link, nil
}

// ParseCallLinkQueryAck parses the metadata returned by link preview.
func ParseCallLinkQueryAck(node *waBinary.Node) (*types.CallLinkPreview, error) {
	child, err := validateCallLinkAck(node, "link_query")
	if err != nil {
		return nil, err
	}
	attrs := child.AttrGetter()
	media, err := parseMedia(attrs.String("media"))
	if err != nil {
		return nil, err
	}
	preview := &types.CallLinkPreview{
		Token:     attrs.String("token"),
		Media:     media,
		Creator:   attrs.JID("link_creator"),
		CreatorPN: attrs.OptionalJIDOrEmpty("link_creator_pn"),
	}
	if attrErr := attrs.Error(); attrErr != nil {
		return nil, fmt.Errorf("whatsmeow: parse link_query ACK: %w", attrErr)
	}
	waitingRoom, ok := child.GetOptionalChildByTag("waiting_room")
	if !ok {
		return nil, fmt.Errorf("whatsmeow: parse link_query ACK: missing waiting_room")
	}
	wrAttrs := waitingRoom.AttrGetter()
	preview.WaitingRoomEnabled = parseBoolAttr(wrAttrs, "enabled")
	preview.IsAdmin = parseBoolAttr(wrAttrs, "is_admin")
	if attrErr := wrAttrs.Error(); attrErr != nil {
		return nil, fmt.Errorf("whatsmeow: parse link_query waiting room: %w", attrErr)
	}
	return preview, nil
}

func parseWaitingRoomNode(node *waBinary.Node) (*types.CallLinkWaitingRoom, error) {
	if node == nil || node.Tag != "waiting_room" {
		return nil, fmt.Errorf("whatsmeow: parse waiting room: unexpected node")
	}
	attrs := node.AttrGetter()
	media, err := parseMedia(attrs.String("media"))
	if err != nil {
		return nil, err
	}
	room := &types.CallLinkWaitingRoom{
		CallID:      attrs.String("call-id"),
		CallCreator: attrs.JID("call-creator"),
		LinkToken:   attrs.String("link-token"),
		Media:       media,
		Enabled:     parseBoolAttr(attrs, "enabled"),
		IsAdmin:     parseBoolAttr(attrs, "is_admin"),
	}
	if transactionID := attrs.OptionalString("transaction-id"); transactionID != "" {
		value, parseErr := strconv.ParseUint(transactionID, 10, 32)
		if parseErr != nil {
			return nil, fmt.Errorf("whatsmeow: parse waiting room transaction ID: %w", parseErr)
		}
		room.TransactionID = uint32(value)
	}
	if attrErr := attrs.Error(); attrErr != nil {
		return nil, fmt.Errorf("whatsmeow: parse waiting room attributes: %w", attrErr)
	}
	for _, child := range node.GetChildren() {
		if child.Tag != "user" {
			continue
		}
		userAttrs := child.AttrGetter()
		user := types.CallLinkWaitingRoomUser{
			JID:   userAttrs.JID("jid"),
			PN:    userAttrs.OptionalJIDOrEmpty("user_pn"),
			State: userAttrs.String("state"),
		}
		if attrErr := userAttrs.Error(); attrErr != nil {
			return nil, fmt.Errorf("whatsmeow: parse waiting room user: %w", attrErr)
		}
		room.Users = append(room.Users, user)
	}
	return room, nil
}

// ParseCallLinkJoinAck parses direct admission or pending waiting-room state.
func ParseCallLinkJoinAck(node *waBinary.Node) (*types.CallLinkJoin, error) {
	if err := validateCallLinkAckEnvelope(node, "link_join"); err != nil {
		return nil, err
	}
	var result types.CallLinkJoin
	waitingNode, hasWaiting := node.GetOptionalChildByTag("waiting_room")
	if hasWaiting {
		waiting, err := parseWaitingRoomNode(&waitingNode)
		if err != nil {
			return nil, err
		}
		result.Token = waiting.LinkToken
		result.Media = waiting.Media
		result.CallID = waiting.CallID
		result.CallCreator = waiting.CallCreator
		result.WaitingRoomEnabled = waiting.Enabled
		result.IsAdmin = waiting.IsAdmin
	}
	group, hasGroup, err := ParseInitialGroupCallAck(node)
	if err != nil {
		return nil, err
	}
	if hasGroup {
		if group == nil {
			return nil, fmt.Errorf("whatsmeow: parse link_join ACK: missing parsed group")
		}
		result.Group = group
		if result.CallID == "" {
			result.CallID = group.CallID
			result.CallCreator = group.CallCreator
			result.Media = types.CallLinkMedia(group.Media)
		} else if result.CallID != group.CallID || result.CallCreator != group.CallCreator {
			return nil, fmt.Errorf("whatsmeow: parse link_join ACK: waiting-room and group identity mismatch")
		}
	}
	if !hasWaiting && !hasGroup {
		return nil, fmt.Errorf("whatsmeow: parse link_join ACK: missing group_info and waiting_room")
	}
	result.InWaitingRoom = hasWaiting && result.WaitingRoomEnabled && !hasGroup
	return &result, nil
}

// ParseWaitingRoomUpdate parses one authoritative waiting-room roster update.
func ParseWaitingRoomUpdate(node *waBinary.Node) (*types.CallLinkWaitingRoom, error) {
	if node == nil || node.Tag != "waiting_room_update" {
		return nil, fmt.Errorf("whatsmeow: parse waiting room update: unexpected node")
	}
	attrs := node.AttrGetter()
	callID := attrs.String("call-id")
	creator := attrs.JID("call-creator")
	if attrErr := attrs.Error(); attrErr != nil {
		return nil, fmt.Errorf("whatsmeow: parse waiting room update identity: %w", attrErr)
	}
	child, ok := node.GetOptionalChildByTag("waiting_room")
	if !ok {
		return nil, fmt.Errorf("whatsmeow: parse waiting room update: missing waiting_room")
	}
	room, err := parseWaitingRoomNode(&child)
	if err != nil {
		return nil, err
	}
	if room.CallID != callID || room.CallCreator != creator {
		return nil, fmt.Errorf("whatsmeow: parse waiting room update: identity mismatch")
	}
	return room, nil
}

// BuildWaitingRoomUpdateAck builds the typed acknowledgement required by call signaling.
func BuildWaitingRoomUpdateAck(node *waBinary.Node) (waBinary.Node, error) {
	if node == nil || node.Tag != "call" {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build waiting room ACK: unexpected node")
	}
	children := node.GetChildren()
	if len(children) != 1 || children[0].Tag != "waiting_room_update" {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build waiting room ACK: missing waiting_room_update")
	}
	attrs := node.AttrGetter()
	to := attrs.JID("from")
	id := attrs.String("id")
	if err := attrs.Error(); err != nil {
		return waBinary.Node{}, fmt.Errorf("whatsmeow: build waiting room ACK: %w", err)
	}
	return waBinary.Node{
		Tag: "ack",
		Attrs: waBinary.Attrs{
			"to":    to,
			"id":    id,
			"class": "call",
			"type":  "waiting_room_update",
		},
	}, nil
}
