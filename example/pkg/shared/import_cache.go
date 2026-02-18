package shared

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var (
	importStats      = make(map[string]*ChatStatus)
	chatMessages     = make(map[string][]MessageData)
	mediaMessages    []MessageData
	downloadableMsgs = make(map[string]whatsmeow.DownloadableMessage)
	messageInfos     = make(map[string]*types.MessageInfo)
	statsLock        sync.Mutex
)

// MockDownloadableMessage implements whatsmeow.DownloadableMessage for restoration
type MockDownloadableMessage struct {
	DirectPath    string
	MediaKey      []byte
	FileSHA256    []byte
	FileEncSHA256 []byte
}

func (m *MockDownloadableMessage) GetDirectPath() string     { return m.DirectPath }
func (m *MockDownloadableMessage) GetMediaKey() []byte       { return m.MediaKey }
func (m *MockDownloadableMessage) GetFileSHA256() []byte     { return m.FileSHA256 }
func (m *MockDownloadableMessage) GetFileEncSHA256() []byte  { return m.FileEncSHA256 }
func (m *MockDownloadableMessage) GetFileLength() uint64     { return 0 } // Not needed usually
func (m *MockDownloadableMessage) GetMimetype() string       { return "" } // Not needed usually

// enableImportCache adds an event handler to the client that caches incoming messages in memory.
func EnableImportCache(cli *whatsmeow.Client) {
	cli.AddEventHandler(importCacheEventHandler)
}

func importCacheEventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		processImportMessage(v.Info, v.Message)
	case *events.HistorySync:
		for _, conv := range v.Data.GetConversations() {
			chatJID, _ := types.ParseJID(conv.GetID())
			status := getImportChatStatus(chatJID)
			if conv.GetName() != "" {
				status.Name = conv.GetName()
			}
			for _, msg := range conv.GetMessages() {
				webMsg := msg.GetMessage()
				if webMsg != nil && webMsg.GetMessage() != nil {
					info := types.MessageInfo{
						MessageSource: types.MessageSource{
							Chat:     chatJID,
							IsFromMe: webMsg.GetKey().GetFromMe(),
						},
						ID:        types.MessageID(webMsg.GetKey().GetID()),
						Timestamp: time.Unix(int64(webMsg.GetMessageTimestamp()), 0),
					}
					processImportMessage(info, webMsg.GetMessage())
				}
			}
		}
	}
}

func processImportMessage(info types.MessageInfo, msg *waE2E.Message) {
	if msg == nil {
		return
	}
	chatJID := info.Chat.String()
	status := getImportChatStatus(info.Chat)
	status.MessageCount++

	content := ""
	caption := ""
	msgType := "text"
	hasMedia := false
	mimeType := ""
	fileName := ""
	var downloadRef whatsmeow.DownloadableMessage

	if msg.GetConversation() != "" {
		content = msg.GetConversation()
	} else if msg.GetExtendedTextMessage() != nil {
		content = msg.GetExtendedTextMessage().GetText()
	} else if img := msg.GetImageMessage(); img != nil {
		msgType = "image"
		caption = img.GetCaption()
		hasMedia = true
		downloadRef = img
		mimeType = img.GetMimetype()
	} else if vid := msg.GetVideoMessage(); vid != nil {
		msgType = "video"
		caption = vid.GetCaption()
		hasMedia = true
		downloadRef = vid
		mimeType = vid.GetMimetype()
	} else if aud := msg.GetAudioMessage(); aud != nil {
		msgType = "audio"
		hasMedia = true
		downloadRef = aud
		mimeType = aud.GetMimetype()
	} else if doc := msg.GetDocumentMessage(); doc != nil {
		msgType = "document"
		caption = doc.GetCaption()
		hasMedia = true
		downloadRef = doc
		mimeType = doc.GetMimetype()
		fileName = doc.GetFileName()
	} else if sticker := msg.GetStickerMessage(); sticker != nil {
		msgType = "sticker"
		hasMedia = true
		downloadRef = sticker
		mimeType = sticker.GetMimetype()
	}

	statsLock.Lock()
	defer statsLock.Unlock()
	msgData := MessageData{
		ID: string(info.ID), ChatJID: chatJID, Text: content, Caption: caption, Type: msgType, Timestamp: info.Timestamp.Unix(), FromMe: info.IsFromMe, HasMedia: hasMedia,
		MimeType: mimeType, FileName: fileName,
	}
	if downloadRef != nil {
		msgData.DirectPath = downloadRef.GetDirectPath()
		msgData.MediaKey = downloadRef.GetMediaKey()
		msgData.FileSHA256 = downloadRef.GetFileSHA256()
		msgData.FileEncSHA256 = downloadRef.GetFileEncSHA256()
	}
	messageInfos[string(info.ID)] = &info
	chatMessages[chatJID] = append(chatMessages[chatJID], msgData)
	if hasMedia {
		status.MediaCount++
		downloadableMsgs[string(info.ID)] = downloadRef
		mediaMessages = append(mediaMessages, msgData)
	}
}

func getImportChatStatus(jid types.JID) *ChatStatus {
	jidStr := jid.String()
	statsLock.Lock()
	defer statsLock.Unlock()
	if s, ok := importStats[jidStr]; ok {
		return s
	}
	s := &ChatStatus{JID: jidStr, ImportStatus: "Partial", LastImported: time.Now(), IsGroup: jid.Server == types.GroupServer}
	importStats[jidStr] = s
	return s
}

func SaveState(filePath string) error {
	statsLock.Lock()
	defer statsLock.Unlock()

	state := StateData{
		ImportStats:   importStats,
		ChatMessages:  chatMessages,
		MediaMessages: mediaMessages,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

func LoadState(filePath string) error {
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return nil // Just start fresh
	}
	if err != nil {
		return err
	}

	var state StateData
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	statsLock.Lock()
	defer statsLock.Unlock()

	if state.ImportStats != nil {
		importStats = state.ImportStats
	}
	if state.ChatMessages != nil {
		chatMessages = state.ChatMessages
	}
	if state.MediaMessages != nil {
		mediaMessages = state.MediaMessages
	}

	// Rebuild downloadableMsgs map from MessageData
	for _, msg := range mediaMessages {
		if msg.HasMedia {
			downloadableMsgs[msg.ID] = &MockDownloadableMessage{
				DirectPath:    msg.DirectPath,
				MediaKey:      msg.MediaKey,
				FileSHA256:    msg.FileSHA256,
				FileEncSHA256: msg.FileEncSHA256,
			}
		}
	}
	fmt.Printf("Loaded state: %d chats, %d media items\n", len(importStats), len(mediaMessages))

	return nil
}
