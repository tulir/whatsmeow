//go:build js && wasm

package shared

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"syscall/js"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
	"go.mau.fi/whatsmeow/util/cbcutil"
	"go.mau.fi/whatsmeow/util/hkdfutil"
)

var wasmBridgeClient *whatsmeow.Client

// pendingRetries stores callbacks for media downloads waiting for a phone re-upload
var (
	pendingRetries     = make(map[string]js.Value)
	pendingRetriesLock sync.Mutex
)

func jsLog(format string, v ...any) {
	msg := fmt.Sprintf("[WASM LOG] "+format, v...)
	js.Global().Get("console").Call("log", msg)
}

// RegisterWASMBridge registers JS functions for interacting with the client from a browser environment.
func RegisterWASMBridge(cli *whatsmeow.Client) {
	wasmBridgeClient = cli
	EnableImportCache(cli)

	// Add event handler for media retries
	cli.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.MediaRetry:
			handleMediaRetry(v)
		}
	})

	js.Global().Set("getImportSummary", js.FuncOf(jsGetImportSummary))
	js.Global().Set("getContacts", js.FuncOf(jsGetContacts))
	js.Global().Set("getChatMessages", js.FuncOf(jsGetChatMessages))
	js.Global().Set("getAllMediaMessages", js.FuncOf(jsGetAllMediaMessages))
	js.Global().Set("getMediaInfo", js.FuncOf(jsGetMediaInfo))
	js.Global().Set("getMediaStreaming", js.FuncOf(jsGetMediaStreaming))
	js.Global().Set("decryptMedia", js.FuncOf(jsDecryptMedia))
	
	jsLog("WASM Bridge registered")
}

func handleMediaRetry(evt *events.MediaRetry) {
	msgID := string(evt.MessageID)
	jsLog("handleMediaRetry: Received retry response for %s", msgID)

	pendingRetriesLock.Lock()
	callback, ok := pendingRetries[msgID]
	delete(pendingRetries, msgID)
	pendingRetriesLock.Unlock()

	if !ok {
		jsLog("handleMediaRetry: No pending callback for %s", msgID)
		return
	}

	statsLock.Lock()
	msgRef, hasRef := downloadableMsgs[msgID]
	statsLock.Unlock()

	if !hasRef {
		sendJSStreamingError(callback, "Media reference lost during retry")
		return
	}

	// Decrypt the retry notification
	retryData, err := whatsmeow.DecryptMediaRetryNotification(evt, msgRef.GetMediaKey())
	if err != nil {
		sendJSStreamingError(callback, fmt.Sprintf("Failed to decrypt retry: %v", err))
		return
	}

	if retryData.GetDirectPath() == "" {
		sendJSStreamingError(callback, "Phone failed to provide a new download path")
		return
	}

	jsLog("handleMediaRetry: SUCCESS! Got new path, re-starting download for %s", msgID)
	
	// Re-run the streaming download with the new path
	go streamDownload(msgID, msgRef, retryData.GetDirectPath(), callback)
}

func jsGetMediaStreaming(this js.Value, args []js.Value) any {
	if len(args) < 2 || args[1].Type() != js.TypeFunction {
		return nil
	}
	msgID := args[0].String()
	callback := args[1]
	
	go func() {
		statsLock.Lock()
		msgRef, hasRef := downloadableMsgs[msgID]
		msgInfo, hasInfo := messageInfos[msgID]
		statsLock.Unlock()

		if !hasRef || wasmBridgeClient == nil {
			sendJSStreamingError(callback, "Media reference not found")
			return
		}

		jsLog("jsGetMediaStreaming: Initial download attempt for %s (type: %s)", msgID, string(whatsmeow.GetMediaType(msgRef)))
		
		data, err := wasmBridgeClient.Download(context.Background(), msgRef)

		if err != nil {
			jsLog("jsGetMediaStreaming: Initial FAILED: %v", err)
			
			// If it's an expiration error (403, 404, 410), request re-upload from phone
			errStr := err.Error()
			if hasInfo && (errStr == "download failed with status code 403" || errStr == "download failed with status code 404" || errStr == "download failed with status code 410") {
				jsLog("jsGetMediaStreaming: Media EXPIRED. Requesting re-upload from phone for %s...", msgID)
				
				pendingRetriesLock.Lock()
				pendingRetries[msgID] = callback
				pendingRetriesLock.Unlock()

				err = wasmBridgeClient.SendMediaRetryReceipt(context.Background(), msgInfo, msgRef.GetMediaKey())
				if err != nil {
					jsLog("jsGetMediaStreaming: Failed to send retry request: %v", err)
					pendingRetriesLock.Lock()
					delete(pendingRetries, msgID)
					pendingRetriesLock.Unlock()
					sendJSStreamingError(callback, err.Error())
				} else {
					jsLog("jsGetMediaStreaming: Retry request SENT for %s. Waiting for phone...", msgID)
					// We don't call callback yet, wait for handleMediaRetry
				}
				return
			}
			
			sendJSStreamingError(callback, err.Error())
			return
		}

		// Success on first try
		streamChunks(data, callback)
	}()

	return nil
}

// streamDownload is a helper to download media with a specific path (used after retry)
func streamDownload(msgID string, msgRef whatsmeow.DownloadableMessage, directPath string, callback js.Value) {
	jsLog("streamDownload: Downloading %s with new path: %s", msgID, directPath)
	
	// We use DownloadMediaWithPath to override the expired path from the original message
	data, err := wasmBridgeClient.DownloadMediaWithPath(
		context.Background(),
		directPath,
		msgRef.GetFileEncSHA256(),
		msgRef.GetFileSHA256(),
		msgRef.GetMediaKey(),
		-1, // length unknown
		whatsmeow.GetMediaType(msgRef),
		"", // auto mmsType
	)

	if err != nil {
		jsLog("streamDownload: FAILED after retry for %s: %v", msgID, err)
		sendJSStreamingError(callback, err.Error())
		return
	}

	streamChunks(data, callback)
}

func streamChunks(data []byte, callback js.Value) {
	total := len(data)
	const chunkSize = 64 * 1024
	for i := 0; i < total; i += chunkSize {
		end := i + chunkSize
		if end > total {
			end = total
		}
		chunk := data[i:end]
		percentage := float64(end) / float64(total) * 100

		obj := js.Global().Get("Object").New()
		obj.Set("event", "chunk")
		obj.Set("data", base64.StdEncoding.EncodeToString(chunk))
		obj.Set("progress", percentage)
		callback.Invoke(obj)
	}

	obj := js.Global().Get("Object").New()
	obj.Set("event", "complete")
	callback.Invoke(obj)
}

func sendJSStreamingError(callback js.Value, msg string) {
	obj := js.Global().Get("Object").New()
	obj.Set("event", "error")
	obj.Set("message", msg)
	callback.Invoke(obj)
}

func jsGetImportSummary(this js.Value, args []js.Value) any {
	statsLock.Lock()
	defer statsLock.Unlock()
	data, _ := json.Marshal(importStats)
	return string(data)
}

func jsGetChatMessages(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return "[]"
	}
	jid := args[0].String()
	count := 100
	if len(args) > 1 {
		count = args[1].Int()
	}
	statsLock.Lock()
	defer statsLock.Unlock()
	msgs, ok := chatMessages[jid]
	if !ok {
		return "[]"
	}
	start := len(msgs) - count
	if start < 0 {
		start = 0
	}
	data, _ := json.Marshal(msgs[start:])
	return string(data)
}

func jsGetAllMediaMessages(this js.Value, args []js.Value) any {
	statsLock.Lock()
	defer statsLock.Unlock()
	data, _ := json.Marshal(mediaMessages)
	return string(data)
}

func jsGetMediaInfo(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return ""
	}
	msgID := args[0].String()
	statsLock.Lock()
	msgRef, hasRef := downloadableMsgs[msgID]
	statsLock.Unlock()

	if !hasRef {
		return ""
	}

	res := map[string]any{
		"direct_path":     msgRef.GetDirectPath(),
		"media_key":       base64.StdEncoding.EncodeToString(msgRef.GetMediaKey()),
		"file_sha256":     base64.StdEncoding.EncodeToString(msgRef.GetFileSHA256()),
		"file_enc_sha256": base64.StdEncoding.EncodeToString(msgRef.GetFileEncSHA256()),
		"media_type":      string(whatsmeow.GetMediaType(msgRef)),
	}
	data, _ := json.Marshal(res)
	return string(data)
}

func jsDecryptMedia(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		return ""
	}
	encData, _ := base64.StdEncoding.DecodeString(args[0].String())
	mediaKey, _ := base64.StdEncoding.DecodeString(args[1].String())
	mediaType := whatsmeow.MediaType(args[2].String())

	if len(encData) <= 10 {
		return ""
	}

	mediaKeyExpanded := hkdfutil.SHA256(mediaKey, nil, []byte(mediaType), 112)
	iv := mediaKeyExpanded[:16]
	cipherKey := mediaKeyExpanded[16:48]
	ciphertext := encData[:len(encData)-10]

	data, err := cbcutil.Decrypt(cipherKey, iv, ciphertext)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

func jsGetContacts(this js.Value, args []js.Value) any {
	if wasmBridgeClient == nil || wasmBridgeClient.Store == nil || wasmBridgeClient.Store.Contacts == nil {
		return "[]"
	}
	contacts, _ := wasmBridgeClient.Store.Contacts.GetAllContacts(context.Background())
	data, _ := json.Marshal(contacts)
	return string(data)
}
