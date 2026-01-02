// Package mobile provides gomobile bindings for whatsmeow
// This package exposes WhatsApp functionality for iOS/Android apps
package mobile

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"google.golang.org/protobuf/proto"

	_ "modernc.org/sqlite"
)

// EventCallback is the interface that iOS/Android apps implement to receive events
type EventCallback interface {
	OnQRCode(code string)
	OnConnected()
	OnDisconnected(reason string)
	OnLoggedOut(reason string)
	OnMessage(msg *Message)
	OnReceipt(receipt *Receipt)
	OnPresence(presence *Presence)
	OnHistorySync(progress int, total int)
	OnError(err string)
}

// Client wraps the whatsmeow client for mobile use
type Client struct {
	client       *whatsmeow.Client
	container    *sqlstore.Container
	callback     EventCallback
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.RWMutex
	isConnected  bool
	isLoggedIn   bool
	deviceJID    string
}

// Message represents a WhatsApp message for mobile
type Message struct {
	ID           string
	ChatJID      string
	SenderJID    string
	SenderName   string
	Text         string
	Timestamp    int64
	IsFromMe     bool
	IsGroup      bool
	MediaType    string // "image", "video", "audio", "document", "sticker", ""
	MediaURL     string
	MediaCaption string
	QuotedID     string
	QuotedText   string
}

// Receipt represents a message receipt
type Receipt struct {
	MessageID   string
	ChatJID     string
	SenderJID   string
	Type        string // "delivered", "read", "played"
	Timestamp   int64
}

// Presence represents user presence information
type Presence struct {
	JID        string
	Available  bool
	LastSeen   int64
}

// Contact represents a WhatsApp contact
type Contact struct {
	JID         string
	Name        string
	PushName    string
	PhoneNumber string
	IsGroup     bool
}

// GroupInfo represents group information
type GroupInfo struct {
	JID           string
	Name          string
	Topic         string
	ParticipantCount int
	CreatedAt     int64
	IsAdmin       bool
}

// NewClient creates a new WhatsApp client
// dbPath should be a path to the SQLite database file
func NewClient(dbPath string, callback EventCallback) (*Client, error) {
	if dbPath == "" {
		return nil, errors.New("database path is required")
	}

	ctx := context.Background()

	container, err := sqlstore.New(ctx, "sqlite", "file:"+dbPath+"?_pragma=foreign_keys(1)", nil)
	if err != nil {
		return nil, err
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, err
	}

	logger := waLog.Stdout("WhatsApp", "INFO", true)
	waClient := whatsmeow.NewClient(deviceStore, logger)

	clientCtx, cancel := context.WithCancel(context.Background())

	c := &Client{
		client:    waClient,
		container: container,
		callback:  callback,
		ctx:       clientCtx,
		cancel:    cancel,
	}

	// Set up event handler
	waClient.AddEventHandler(c.handleEvent)

	return c, nil
}

// Connect connects to WhatsApp servers
// If not logged in, this will start the QR code pairing process
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client.Store.ID == nil {
		// Not logged in, need to pair with QR code
		qrChan, err := c.client.GetQRChannel(c.ctx)
		if err != nil {
			return err
		}

		err = c.client.Connect()
		if err != nil {
			return err
		}

		// Handle QR codes in background
		go func() {
			for evt := range qrChan {
				if evt.Event == "code" {
					if c.callback != nil {
						c.callback.OnQRCode(evt.Code)
					}
				}
			}
		}()
	} else {
		// Already logged in, just connect
		err := c.client.Connect()
		if err != nil {
			return err
		}
	}

	return nil
}

// Disconnect disconnects from WhatsApp
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.client.Disconnect()
	c.isConnected = false
}

// Logout logs out and unpairs the device
func (c *Client) Logout() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.client.Logout(c.ctx)
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.IsConnected()
}

// IsLoggedIn returns whether the client is logged in
func (c *Client) IsLoggedIn() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client.IsLoggedIn()
}

// GetMyJID returns the JID of the logged-in user
func (c *Client) GetMyJID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.client.Store.ID == nil {
		return ""
	}
	return c.client.Store.ID.String()
}

// SendTextMessage sends a text message
func (c *Client) SendTextMessage(chatJID string, text string) (string, error) {
	jid, err := types.ParseJID(chatJID)
	if err != nil {
		return "", err
	}

	msg := &waE2E.Message{
		Conversation: proto.String(text),
	}

	resp, err := c.client.SendMessage(c.ctx, jid, msg)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

// SendImageMessage sends an image message
// imageData should be base64 encoded image data
func (c *Client) SendImageMessage(chatJID string, imageDataBase64 string, caption string, mimeType string) (string, error) {
	jid, err := types.ParseJID(chatJID)
	if err != nil {
		return "", err
	}

	imageData, err := base64.StdEncoding.DecodeString(imageDataBase64)
	if err != nil {
		return "", err
	}

	uploaded, err := c.client.Upload(c.ctx, imageData, whatsmeow.MediaImage)
	if err != nil {
		return "", err
	}

	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption:       proto.String(caption),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(imageData))),
		},
	}

	resp, err := c.client.SendMessage(c.ctx, jid, msg)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

// SendDocumentMessage sends a document
// documentData should be base64 encoded
func (c *Client) SendDocumentMessage(chatJID string, documentDataBase64 string, filename string, caption string, mimeType string) (string, error) {
	jid, err := types.ParseJID(chatJID)
	if err != nil {
		return "", err
	}

	docData, err := base64.StdEncoding.DecodeString(documentDataBase64)
	if err != nil {
		return "", err
	}

	uploaded, err := c.client.Upload(c.ctx, docData, whatsmeow.MediaDocument)
	if err != nil {
		return "", err
	}

	msg := &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			Caption:       proto.String(caption),
			Title:         proto.String(filename),
			FileName:      proto.String(filename),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(docData))),
		},
	}

	resp, err := c.client.SendMessage(c.ctx, jid, msg)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

// MarkAsRead marks messages as read
// messageIDs should be a JSON array of message IDs
func (c *Client) MarkAsRead(chatJID string, messageIDsJSON string) error {
	jid, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}

	var messageIDs []string
	if err := json.Unmarshal([]byte(messageIDsJSON), &messageIDs); err != nil {
		return err
	}

	ids := make([]types.MessageID, len(messageIDs))
	for i, id := range messageIDs {
		ids[i] = types.MessageID(id)
	}

	return c.client.MarkRead(c.ctx, ids, time.Now(), jid, jid)
}

// SendTyping sends a typing indicator
func (c *Client) SendTyping(chatJID string, typing bool) error {
	jid, err := types.ParseJID(chatJID)
	if err != nil {
		return err
	}

	presence := types.ChatPresenceComposing
	if !typing {
		presence = types.ChatPresencePaused
	}

	return c.client.SendChatPresence(c.ctx, jid, presence, types.ChatPresenceMediaText)
}

// SetPresence sets the user's presence (online/offline)
func (c *Client) SetPresence(available bool) error {
	presence := types.PresenceAvailable
	if !available {
		presence = types.PresenceUnavailable
	}
	return c.client.SendPresence(c.ctx, presence)
}

// GetContactInfo gets information about a contact
func (c *Client) GetContactInfo(jidStr string) (*Contact, error) {
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		return nil, err
	}

	contact := &Contact{
		JID:         jid.String(),
		PhoneNumber: jid.User,
		IsGroup:     jid.Server == types.GroupServer,
	}

	// Try to get stored contact name
	stored, err := c.client.Store.Contacts.GetContact(c.ctx, jid)
	if err == nil {
		contact.Name = stored.FullName
		contact.PushName = stored.PushName
		if contact.Name == "" {
			contact.Name = stored.PushName
		}
	}

	return contact, nil
}

// GetGroupInfo gets information about a group
func (c *Client) GetGroupInfo(groupJIDStr string) (*GroupInfo, error) {
	groupJID, err := types.ParseJID(groupJIDStr)
	if err != nil {
		return nil, err
	}

	info, err := c.client.GetGroupInfo(c.ctx, groupJID)
	if err != nil {
		return nil, err
	}

	isAdmin := false
	myJID := c.client.Store.ID
	if myJID != nil {
		for _, p := range info.Participants {
			if p.JID.User == myJID.User && (p.IsAdmin || p.IsSuperAdmin) {
				isAdmin = true
				break
			}
		}
	}

	return &GroupInfo{
		JID:              groupJID.String(),
		Name:             info.Name,
		Topic:            info.Topic,
		ParticipantCount: len(info.Participants),
		CreatedAt:        info.GroupCreated.Unix(),
		IsAdmin:          isAdmin,
	}, nil
}

// GetJoinedGroups returns all joined groups as JSON
func (c *Client) GetJoinedGroups() (string, error) {
	groups, err := c.client.GetJoinedGroups(c.ctx)
	if err != nil {
		return "", err
	}

	result := make([]GroupInfo, len(groups))
	myJID := c.client.Store.ID

	for i, g := range groups {
		isAdmin := false
		if myJID != nil {
			for _, p := range g.Participants {
				if p.JID.User == myJID.User && (p.IsAdmin || p.IsSuperAdmin) {
					isAdmin = true
					break
				}
			}
		}

		result[i] = GroupInfo{
			JID:              g.JID.String(),
			Name:             g.Name,
			Topic:            g.Topic,
			ParticipantCount: len(g.Participants),
			CreatedAt:        g.GroupCreated.Unix(),
			IsAdmin:          isAdmin,
		}
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// IsOnWhatsApp checks if a phone number is registered on WhatsApp
// phoneNumber should include country code (e.g., "+1234567890")
func (c *Client) IsOnWhatsApp(phoneNumber string) (bool, error) {
	resp, err := c.client.IsOnWhatsApp(c.ctx, []string{phoneNumber})
	if err != nil {
		return false, err
	}

	if len(resp) == 0 {
		return false, nil
	}

	return resp[0].IsIn, nil
}

// GetProfilePicture gets the profile picture URL for a JID
func (c *Client) GetProfilePicture(jidStr string) (string, error) {
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		return "", err
	}

	pic, err := c.client.GetProfilePictureInfo(c.ctx, jid, &whatsmeow.GetProfilePictureParams{})
	if err != nil {
		return "", err
	}

	if pic == nil {
		return "", nil
	}

	return pic.URL, nil
}

// DownloadMedia downloads media from a message and returns base64 encoded data
func (c *Client) DownloadMedia(mediaURL string, mediaKey string, mediaType string) (string, error) {
	// This is a simplified version - in practice you'd need the full message
	// to properly download media
	return "", errors.New("use DownloadMediaFromMessage instead")
}

// CreateGroup creates a new group
// participantsJSON should be a JSON array of JID strings
func (c *Client) CreateGroup(name string, participantsJSON string) (string, error) {
	var participantStrs []string
	if err := json.Unmarshal([]byte(participantsJSON), &participantStrs); err != nil {
		return "", err
	}

	participants := make([]types.JID, len(participantStrs))
	for i, p := range participantStrs {
		jid, err := types.ParseJID(p)
		if err != nil {
			return "", err
		}
		participants[i] = jid
	}

	resp, err := c.client.CreateGroup(c.ctx, whatsmeow.ReqCreateGroup{
		Name:         name,
		Participants: participants,
	})
	if err != nil {
		return "", err
	}

	return resp.JID.String(), nil
}

// LeaveGroup leaves a group
func (c *Client) LeaveGroup(groupJIDStr string) error {
	groupJID, err := types.ParseJID(groupJIDStr)
	if err != nil {
		return err
	}

	return c.client.LeaveGroup(c.ctx, groupJID)
}

// GetDeviceJID returns the device JID if logged in
func (c *Client) GetDeviceJID() string {
	if c.client.Store.ID == nil {
		return ""
	}
	return c.client.Store.ID.String()
}

// handleEvent processes incoming events and forwards them to the callback
func (c *Client) handleEvent(evt interface{}) {
	if c.callback == nil {
		return
	}

	switch v := evt.(type) {
	case *events.Connected:
		c.mu.Lock()
		c.isConnected = true
		c.isLoggedIn = true
		if c.client.Store.ID != nil {
			c.deviceJID = c.client.Store.ID.String()
		}
		c.mu.Unlock()
		c.callback.OnConnected()

	case *events.Disconnected:
		c.mu.Lock()
		c.isConnected = false
		c.mu.Unlock()
		c.callback.OnDisconnected("disconnected")

	case *events.LoggedOut:
		c.mu.Lock()
		c.isConnected = false
		c.isLoggedIn = false
		c.mu.Unlock()
		c.callback.OnLoggedOut(v.Reason.String())

	case *events.StreamReplaced:
		c.mu.Lock()
		c.isConnected = false
		c.mu.Unlock()
		c.callback.OnDisconnected("stream replaced by another connection")

	case *events.Message:
		msg := c.convertMessage(v)
		c.callback.OnMessage(msg)

	case *events.Receipt:
		receipt := &Receipt{
			MessageID: string(v.MessageIDs[0]),
			ChatJID:   v.Chat.String(),
			SenderJID: v.Sender.String(),
			Timestamp: v.Timestamp.Unix(),
		}
		switch v.Type {
		case types.ReceiptTypeDelivered:
			receipt.Type = "delivered"
		case types.ReceiptTypeRead:
			receipt.Type = "read"
		case types.ReceiptTypePlayed:
			receipt.Type = "played"
		default:
			receipt.Type = "unknown"
		}
		c.callback.OnReceipt(receipt)

	case *events.Presence:
		presence := &Presence{
			JID:       v.From.String(),
			Available: v.Unavailable == false,
			LastSeen:  v.LastSeen.Unix(),
		}
		c.callback.OnPresence(presence)

	case *events.HistorySync:
		// Process history sync messages
		c.processHistorySync(v)

	case *events.PairSuccess:
		c.mu.Lock()
		c.deviceJID = v.ID.String()
		c.mu.Unlock()
	}
}

// convertMessage converts a whatsmeow message event to our Message type
func (c *Client) convertMessage(evt *events.Message) *Message {
	msg := &Message{
		ID:        evt.Info.ID,
		ChatJID:   evt.Info.Chat.String(),
		SenderJID: evt.Info.Sender.String(),
		Timestamp: evt.Info.Timestamp.Unix(),
		IsFromMe:  evt.Info.IsFromMe,
		IsGroup:   evt.Info.IsGroup,
	}

	// Get sender name
	if evt.Info.PushName != "" {
		msg.SenderName = evt.Info.PushName
	}

	// Extract message content
	if evt.Message == nil {
		return msg
	}

	// Text message
	if evt.Message.Conversation != nil {
		msg.Text = *evt.Message.Conversation
	}

	// Extended text message
	if extText := evt.Message.ExtendedTextMessage; extText != nil {
		if extText.Text != nil {
			msg.Text = *extText.Text
		}
		// Handle quoted message
		if extText.ContextInfo != nil && extText.ContextInfo.QuotedMessage != nil {
			if extText.ContextInfo.StanzaID != nil {
				msg.QuotedID = *extText.ContextInfo.StanzaID
			}
			if extText.ContextInfo.QuotedMessage.Conversation != nil {
				msg.QuotedText = *extText.ContextInfo.QuotedMessage.Conversation
			}
		}
	}

	// Image message
	if img := evt.Message.ImageMessage; img != nil {
		msg.MediaType = "image"
		if img.URL != nil {
			msg.MediaURL = *img.URL
		}
		if img.Caption != nil {
			msg.MediaCaption = *img.Caption
			msg.Text = *img.Caption
		}
	}

	// Video message
	if vid := evt.Message.VideoMessage; vid != nil {
		msg.MediaType = "video"
		if vid.URL != nil {
			msg.MediaURL = *vid.URL
		}
		if vid.Caption != nil {
			msg.MediaCaption = *vid.Caption
			msg.Text = *vid.Caption
		}
	}

	// Audio message
	if audio := evt.Message.AudioMessage; audio != nil {
		msg.MediaType = "audio"
		if audio.URL != nil {
			msg.MediaURL = *audio.URL
		}
	}

	// Document message
	if doc := evt.Message.DocumentMessage; doc != nil {
		msg.MediaType = "document"
		if doc.URL != nil {
			msg.MediaURL = *doc.URL
		}
		if doc.Caption != nil {
			msg.MediaCaption = *doc.Caption
		}
		if doc.Title != nil {
			msg.Text = *doc.Title
		}
	}

	// Sticker message
	if sticker := evt.Message.StickerMessage; sticker != nil {
		msg.MediaType = "sticker"
		if sticker.URL != nil {
			msg.MediaURL = *sticker.URL
		}
	}

	return msg
}

// processHistorySync extracts messages from history sync and sends them to callback
func (c *Client) processHistorySync(evt *events.HistorySync) {
	if c.callback == nil || evt.Data == nil {
		return
	}

	// Report progress
	progress := 0
	total := 0
	if evt.Data.Progress != nil {
		progress = int(*evt.Data.Progress)
	}
	if evt.Data.Conversations != nil {
		total = len(evt.Data.Conversations)
	}
	c.callback.OnHistorySync(progress, total)

	// Process conversations
	for _, conv := range evt.Data.Conversations {
		if conv.ID == nil {
			continue
		}

		chatJID := *conv.ID

		// Process messages in this conversation
		for _, historyMsg := range conv.Messages {
			if historyMsg == nil || historyMsg.Message == nil || historyMsg.Message.Message == nil {
				continue
			}

			msgInfo := historyMsg.Message
			waMsg := msgInfo.Message

			// Parse chat JID
			parsedChatJID, err := types.ParseJID(chatJID)
			if err != nil {
				continue
			}

			// Build message
			msg := &Message{
				ChatJID:   chatJID,
				IsGroup:   parsedChatJID.Server == types.GroupServer,
				Timestamp: int64(msgInfo.GetMessageTimestamp()),
			}

			// Get message ID
			if msgInfo.Key != nil && msgInfo.Key.ID != nil {
				msg.ID = *msgInfo.Key.ID
			}

			// Determine sender
			if msgInfo.Key != nil {
				if msgInfo.Key.FromMe != nil && *msgInfo.Key.FromMe {
					msg.IsFromMe = true
					if c.client.Store.ID != nil {
						msg.SenderJID = c.client.Store.ID.String()
					}
				} else if msgInfo.Key.Participant != nil {
					msg.SenderJID = *msgInfo.Key.Participant
				} else if msgInfo.Key.RemoteJID != nil {
					msg.SenderJID = *msgInfo.Key.RemoteJID
				}
			}

			// Get push name
			if msgInfo.PushName != nil {
				msg.SenderName = *msgInfo.PushName
			}

			// Extract message content
			if waMsg.Conversation != nil {
				msg.Text = *waMsg.Conversation
			}

			if extText := waMsg.ExtendedTextMessage; extText != nil {
				if extText.Text != nil {
					msg.Text = *extText.Text
				}
			}

			if img := waMsg.ImageMessage; img != nil {
				msg.MediaType = "image"
				if img.URL != nil {
					msg.MediaURL = *img.URL
				}
				if img.Caption != nil {
					msg.MediaCaption = *img.Caption
					if msg.Text == "" {
						msg.Text = *img.Caption
					}
				}
			}

			if vid := waMsg.VideoMessage; vid != nil {
				msg.MediaType = "video"
				if vid.URL != nil {
					msg.MediaURL = *vid.URL
				}
				if vid.Caption != nil {
					msg.MediaCaption = *vid.Caption
				}
			}

			if audio := waMsg.AudioMessage; audio != nil {
				msg.MediaType = "audio"
				if audio.URL != nil {
					msg.MediaURL = *audio.URL
				}
			}

			if doc := waMsg.DocumentMessage; doc != nil {
				msg.MediaType = "document"
				if doc.URL != nil {
					msg.MediaURL = *doc.URL
				}
				if doc.Title != nil {
					msg.Text = *doc.Title
				}
			}

			if sticker := waMsg.StickerMessage; sticker != nil {
				msg.MediaType = "sticker"
				if sticker.URL != nil {
					msg.MediaURL = *sticker.URL
				}
			}

			// Only send if we have some content
			if msg.ID != "" && (msg.Text != "" || msg.MediaType != "") {
				c.callback.OnMessage(msg)
			}
		}
	}
}

// Close cleans up resources
func (c *Client) Close() {
	c.cancel()
	c.Disconnect()
}

// GetStoredContacts returns all stored contacts as JSON
func (c *Client) GetStoredContacts() (string, error) {
	contacts, err := c.client.Store.Contacts.GetAllContacts(c.ctx)
	if err != nil {
		return "", err
	}

	result := make([]Contact, 0, len(contacts))
	for jid, info := range contacts {
		contact := Contact{
			JID:         jid.String(),
			Name:        info.FullName,
			PushName:    info.PushName,
			PhoneNumber: jid.User,
			IsGroup:     jid.Server == types.GroupServer,
		}
		if contact.Name == "" {
			contact.Name = info.PushName
		}
		result = append(result, contact)
	}

	data, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// Utility function to create JID from phone number
func CreateJID(phoneNumber string) string {
	// Remove + and any spaces/dashes
	cleaned := ""
	for _, c := range phoneNumber {
		if c >= '0' && c <= '9' {
			cleaned += string(c)
		}
	}
	return cleaned + "@" + types.DefaultUserServer
}

// Utility function to create group JID
func CreateGroupJID(groupID string) string {
	return groupID + "@" + types.GroupServer
}
