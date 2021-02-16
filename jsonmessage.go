package whatsapp

import (
	"encoding/json"
	"fmt"
	"strings"
)

type JSONMessage []json.RawMessage

type JSONMessageType string

const (
	MessageMsgInfo  JSONMessageType = "MsgInfo"
	MessageMsg      JSONMessageType = "Msg"
	MessagePresence JSONMessageType = "Presence"
	MessageStream   JSONMessageType = "Stream"
	MessageConn     JSONMessageType = "Conn"
	MessageProps    JSONMessageType = "Props"
	MessageCmd      JSONMessageType = "Cmd"
	MessageChat     JSONMessageType = "Chat"
	MessageCall     JSONMessageType = "Call"
)

type StreamType string

const (
	StreamUpdate = "update"
	StreamSleep  = "asleep"
)

type StreamEvent struct {
	Type StreamType

	IsOutdated bool
	Version    string

	Extra []json.RawMessage
}

type ProtocolProps struct {
	WebPresence            bool   `json:"webPresence"`
	NotificationQuery      bool   `json:"notificationQuery"`
	FacebookCrashLog       bool   `json:"fbCrashlog"`
	Bucket                 string `json:"bucket"`
	GIFSearch              string `json:"gifSearch"`
	Spam                   bool   `json:"SPAM"`
	SetBlock               bool   `json:"SET_BLOCK"`
	MessageInfo            bool   `json:"MESSAGE_INFO"`
	MaxFileSize            int    `json:"maxFileSize"`
	Media                  int    `json:"media"`
	GroupNameLength        int    `json:"maxSubject"`
	GroupDescriptionLength int    `json:"groupDescLength"`
	MaxParticipants        int    `json:"maxParticipants"`
	VideoMaxEdge           int    `json:"videoMaxEdge"`
	ImageMaxEdge           int    `json:"imageMaxEdge"`
	ImageMaxKilobytes      int    `json:"imageMaxKBytes"`
	Edit                   int    `json:"edit"`
	FwdUIStartTimestamp    int    `json:"fwdUiStartTs"`
	GroupsV3               int    `json:"groupsV3"`
	RestrictGroups         int    `json:"restrictGroups"`
	AnnounceGroups         int    `json:"announceGroups"`
}

type PresenceEvent struct {
	JID       string   `json:"id"`
	SenderJID string   `json:"participant"`
	Status    Presence `json:"type"`
	Timestamp int64    `json:"t"`
	Deny      bool     `json:"deny"`
}

type MsgInfoCommand string

const (
	MsgInfoCommandAck  MsgInfoCommand = "ack"
	MsgInfoCommandAcks MsgInfoCommand = "acks"
)

type Acknowledgement int

const (
	AckMessageSent      Acknowledgement = 1
	AckMessageDelivered Acknowledgement = 2
	AckMessageRead      Acknowledgement = 3
)

type JSONStringOrArray []string

func (jsoa *JSONStringOrArray) UnmarshalJSON(data []byte) error {
	var str string
	if json.Unmarshal(data, &str) == nil {
		*jsoa = []string{str}
		return nil
	}
	var strs []string
	json.Unmarshal(data, &strs)
	*jsoa = strs
	return nil
}

type JSONMsgInfo struct {
	Command         MsgInfoCommand    `json:"cmd"`
	IDs             JSONStringOrArray `json:"id"`
	Acknowledgement Acknowledgement   `json:"ack"`
	MessageFromJID  string            `json:"from"`
	SenderJID       string            `json:"participant"`
	ToJID           string            `json:"to"`
	Timestamp       int64             `json:"t"`
}

type ConnInfo struct {
	ProtocolVersion []int `json:"protoVersion"`
	BinaryVersion   int   `json:"binVersion"`
	Phone           struct {
		WhatsAppVersion    string `json:"wa_version"`
		MCC                string `json:"mcc"`
		MNC                string `json:"mnc"`
		OSVersion          string `json:"os_version"`
		DeviceManufacturer string `json:"device_manufacturer"`
		DeviceModel        string `json:"device_model"`
		OSBuildNumber      string `json:"os_build_number"`
	} `json:"phone"`
	Features map[string]interface{} `json:"features"`
	PushName string                 `json:"pushname"`
}

type CommandType string

const (
	CommandPicture    CommandType = "picture"
	CommandDisconnect CommandType = "disconnect"
)

type JSONCommand struct {
	Type CommandType `json:"type"`
	JID  string      `json:"jid"`

	*ProfilePicInfo
	Kind string `json:"kind"`

	Raw json.RawMessage `json:"-"`
}

type ChatUpdateCommand string

const (
	ChatUpdateCommandAction ChatUpdateCommand = "action"
)

type ChatUpdate struct {
	JID     string            `json:"id"`
	Command ChatUpdateCommand `json:"cmd"`
	Data    ChatUpdateData    `json:"data"`
}

type ChatActionType string

const (
	ChatActionNameChange  ChatActionType = "subject"
	ChatActionAddTopic    ChatActionType = "desc_add"
	ChatActionRemoveTopic ChatActionType = "desc_remove"
	ChatActionRestrict    ChatActionType = "restrict"
	ChatActionAnnounce    ChatActionType = "announce"
	ChatActionPromote     ChatActionType = "promote"
	ChatActionDemote      ChatActionType = "demote"
	ChatActionIntroduce   ChatActionType = "introduce"
	ChatActionCreate      ChatActionType = "create"
	ChatActionRemove      ChatActionType = "remove"
	ChatActionAdd         ChatActionType = "add"
)

type ChatUpdateData struct {
	Action    ChatActionType
	SenderJID string

	NameChange struct {
		Name  string `json:"subject"`
		SetAt int64  `json:"s_t"`
		SetBy string `json:"s_o"`
	}

	AddTopic struct {
		Topic string `json:"desc"`
		ID    string `json:"descId"`
		SetAt int64  `json:"descTime"`
		SetBy string `json:"descOwner"`
	}

	RemoveTopic struct {
		ID string `json:"descId"`
	}

	Introduce struct {
		CreationTime int64    `json:"creation"`
		Admins       []string `json:"admins"`
		SuperAdmins  []string `json:"superadmins"`
		Regulars     []string `json:"regulars"`
	}

	Restrict bool

	Announce bool

	UserChange struct {
		JIDs []string `json:"participants"`
	}
}

func (cud *ChatUpdateData) UnmarshalJSON(data []byte) error {
	var arr []json.RawMessage
	err := json.Unmarshal(data, &arr)
	if err != nil {
		return err
	} else if len(arr) < 3 {
		return nil
	}

	err = json.Unmarshal(arr[0], &cud.Action)
	if err != nil {
		return err
	}

	err = json.Unmarshal(arr[1], &cud.SenderJID)
	if err != nil {
		return err
	}
	cud.SenderJID = strings.Replace(cud.SenderJID, OldUserSuffix, NewUserSuffix, 1)

	var unmarshalTo interface{}
	switch cud.Action {
	case ChatActionIntroduce, ChatActionCreate:
		err = json.Unmarshal(arr[2], &cud.NameChange)
		if err != nil {
			return err
		}
		err = json.Unmarshal(arr[2], &cud.AddTopic)
		if err != nil {
			return err
		}
		unmarshalTo = &cud.Introduce
	case ChatActionNameChange:
		unmarshalTo = &cud.NameChange
	case ChatActionAddTopic:
		unmarshalTo = &cud.AddTopic
	case ChatActionRemoveTopic:
		unmarshalTo = &cud.RemoveTopic
	case ChatActionRestrict:
		unmarshalTo = &cud.Restrict
	case ChatActionAnnounce:
		unmarshalTo = &cud.Announce
	case ChatActionPromote, ChatActionDemote, ChatActionRemove, ChatActionAdd:
		unmarshalTo = &cud.UserChange
	default:
		return nil
	}
	err = json.Unmarshal(arr[2], unmarshalTo)
	if err != nil {
		return err
	}
	cud.NameChange.SetBy = strings.Replace(cud.NameChange.SetBy, OldUserSuffix, NewUserSuffix, 1)
	for index, jid := range cud.UserChange.JIDs {
		cud.UserChange.JIDs[index] = strings.Replace(jid, OldUserSuffix, NewUserSuffix, 1)
	}
	for index, jid := range cud.Introduce.SuperAdmins {
		cud.Introduce.SuperAdmins[index] = strings.Replace(jid, OldUserSuffix, NewUserSuffix, 1)
	}
	for index, jid := range cud.Introduce.Admins {
		cud.Introduce.Admins[index] = strings.Replace(jid, OldUserSuffix, NewUserSuffix, 1)
	}
	for index, jid := range cud.Introduce.Regulars {
		cud.Introduce.Regulars[index] = strings.Replace(jid, OldUserSuffix, NewUserSuffix, 1)
	}
	return nil
}

type CallInfoType string

const (
	CallOffer        CallInfoType = "offer"
	CallOfferVideo   CallInfoType = "offer_video"
	CallTransport    CallInfoType = "transport"
	CallRelayLatency CallInfoType = "relaylatency"
	CallTerminate    CallInfoType = "terminate"
)

type CallInfo struct {
	ID   string       `json:"id"`
	Type CallInfoType `json:"type"`
	From string       `json:"from"`

	Platform string `json:"platform"`
	Version  []int  `json:"version"`

	Data [][]interface{} `json:"data"`
}

func (wac *Conn) handleJSONMessage(message string) {
	msg := JSONMessage{}
	err := json.Unmarshal([]byte(message), &msg)
	if err != nil || len(msg) < 2 {
		wac.handle(fmt.Errorf("WhatsApp JSON parse error: %w", err))
		return
	}

	var msgType JSONMessageType
	err = json.Unmarshal(msg[0], &msgType)
	if err != nil {
		wac.handle(fmt.Errorf("WhatsApp JSON parse error parsing message type: %w", err))
		return
	}

	switch msgType {
	case MessagePresence:
		wac.handleMessagePresence(msg[1])
	case MessageStream:
		wac.handleMessageStream(msg[1:])
	case MessageConn:
		wac.handleMessageConn(msg[1])
	case MessageProps:
		wac.handleMessageProps(msg[1])
	case MessageMsgInfo, MessageMsg:
		wac.handleMessageMsgInfo(msgType, msg[1])
	case MessageCmd:
		wac.handleMessageCommand(msg[1])
	case MessageChat:
		wac.handleMessageChatUpdate(msg[1])
	case MessageCall:
		wac.handleMessageCall(msg[1])
	}
}

func (wac *Conn) handleMessageStream(message []json.RawMessage) {
	var event StreamEvent
	err := json.Unmarshal(message[0], &event.Type)
	if err != nil {
		wac.handle(fmt.Errorf("WhatsApp JSON parse error parsing StreamEvent: %w", err))
		return
	}

	if event.Type == StreamUpdate && len(message) >= 3 {
		_ = json.Unmarshal(message[1], &event.IsOutdated)
		_ = json.Unmarshal(message[2], &event.Version)
		if len(message) >= 4 {
			event.Extra = message[3:]
		}
	} else if len(message) >= 2 {
		event.Extra = message[1:]
	}

	wac.handle(event)
}

func (wac *Conn) handleMessageProps(message []byte) {
	var event ProtocolProps
	err := json.Unmarshal(message, &event)
	if err != nil {
		wac.handle(fmt.Errorf("WhatsApp JSON parse error parsing ProtocolProps: %w", err))
		return
	}
	wac.handle(event)
}

func (wac *Conn) handleMessagePresence(message []byte) {
	var event PresenceEvent
	err := json.Unmarshal(message, &event)
	if err != nil {
		wac.handle(fmt.Errorf("WhatsApp JSON parse error parsing PresenceEvent: %w", err))
		return
	}
	event.JID = strings.Replace(event.JID, OldUserSuffix, NewUserSuffix, 1)
	if len(event.SenderJID) == 0 {
		event.SenderJID = event.JID
	} else {
		event.SenderJID = strings.Replace(event.SenderJID, OldUserSuffix, NewUserSuffix, 1)
	}
	wac.handle(event)
}

func (wac *Conn) handleMessageMsgInfo(msgType JSONMessageType, message []byte) {
	var event JSONMsgInfo
	err := json.Unmarshal(message, &event)
	if err != nil {
		wac.handle(fmt.Errorf("WhatsApp JSON parse error parsing MsgInfo: %w", err))
		return
	}
	event.MessageFromJID = strings.Replace(event.MessageFromJID, OldUserSuffix, NewUserSuffix, 1)
	event.SenderJID = strings.Replace(event.SenderJID, OldUserSuffix, NewUserSuffix, 1)
	event.ToJID = strings.Replace(event.ToJID, OldUserSuffix, NewUserSuffix, 1)
	if msgType == MessageMsg {
		event.SenderJID = event.ToJID
	}
	wac.handle(event)
}

func (wac *Conn) handleMessageConn(message []byte) {
	var event ConnInfo
	err := json.Unmarshal(message, &event)
	if err != nil {
		wac.handle(fmt.Errorf("WhatsApp JSON parse error parsing ConnInfo: %w", err))
		return
	}
	wac.handle(event)
}

func (wac *Conn) handleMessageCommand(message []byte) {
	var event JSONCommand
	err := json.Unmarshal(message, &event)
	if err != nil {
		wac.handle(fmt.Errorf("WhatsApp JSON parse error parsing JSONCommand: %w", err))
		return
	}
	event.Raw = message
	event.JID = strings.Replace(event.JID, OldUserSuffix, NewUserSuffix, 1)
	wac.handle(event)
}

func (wac *Conn) handleMessageChatUpdate(message []byte) {
	var event ChatUpdate
	err := json.Unmarshal(message, &event)
	if err != nil {
		wac.handle(fmt.Errorf("WhatsApp JSON parse error parsing ChatUpdate: %w", err))
		return
	}
	event.JID = strings.Replace(event.JID, OldUserSuffix, NewUserSuffix, 1)
	wac.handle(event)
}

func (wac *Conn) handleMessageCall(message []byte) {
	var event CallInfo
	err := json.Unmarshal(message, &event)
	if err != nil {
		wac.handle(fmt.Errorf("WhatsApp JSON parse error parsing CallInfo: %w", err))
		return
	}
	event.From = strings.Replace(event.From, OldUserSuffix, NewUserSuffix, 1)
	wac.handle(event)
}
