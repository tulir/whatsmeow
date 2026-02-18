package shared

import (
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// GetChats returns a copy of the current chat status map
func GetChats() map[string]ChatStatus {
	statsLock.Lock()
	defer statsLock.Unlock()
	
	// Return a copy to avoid race conditions
	result := make(map[string]ChatStatus)
	for k, v := range importStats {
		result[k] = *v
	}
	return result
}

// GetChatMessages returns a copy of messages for a specific chat
func GetChatMessages(jid string) []MessageData {
	statsLock.Lock()
	defer statsLock.Unlock()
	
	if msgs, ok := chatMessages[jid]; ok {
		// Return a copy
		result := make([]MessageData, len(msgs))
		copy(result, msgs)
		return result
	}
	return nil
}

// GetAllMedia returns a copy of all discovered media messages
func GetAllMedia() []MessageData {
	statsLock.Lock()
	defer statsLock.Unlock()
	
	// Return a copy
	result := make([]MessageData, len(mediaMessages))
	copy(result, mediaMessages)
	return result
}

// GetDownloadableMessage retrieves the internal reference for a media message
func GetDownloadableMessage(msgID string) (whatsmeow.DownloadableMessage, bool) {
	statsLock.Lock()
	defer statsLock.Unlock()
	msg, ok := downloadableMsgs[msgID]
	return msg, ok
}

// GetMessageInfo retrieves the full message info (needed for retries)
func GetMessageInfo(msgID string) (*types.MessageInfo, bool) {
	statsLock.Lock()
	defer statsLock.Unlock()
	info, ok := messageInfos[msgID]
	return info, ok
}
