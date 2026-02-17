package shared

import (
	"sync"
	"time"

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
	messageInfos     = make(map[string]*types.MessageInfo) // New: store full info for retries
	statsLock        sync.Mutex
)

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
	messageInfos[string(info.ID)] = &info // Save original info
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
