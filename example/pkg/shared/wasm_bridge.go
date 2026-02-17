//go:build js && wasm

package shared

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"syscall/js"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/util/cbcutil"
	"go.mau.fi/whatsmeow/util/hkdfutil"
)

var wasmBridgeClient *whatsmeow.Client

func jsLog(format string, v ...any) {
	msg := fmt.Sprintf("[WASM LOG] "+format, v...)
	js.Global().Get("console").Call("log", msg)
}

// registerWASMBridge registers JS functions for interacting with the client from a browser environment.
func RegisterWASMBridge(cli *whatsmeow.Client) {
	wasmBridgeClient = cli
	EnableImportCache(cli)

	js.Global().Set("getImportSummary", js.FuncOf(jsGetImportSummary))
	js.Global().Set("getContacts", js.FuncOf(jsGetContacts))
	js.Global().Set("getChatMessages", js.FuncOf(jsGetChatMessages))
	js.Global().Set("getAllMediaMessages", js.FuncOf(jsGetAllMediaMessages))
	js.Global().Set("getMediaInfo", js.FuncOf(jsGetMediaInfo))
	js.Global().Set("getMediaStreaming", js.FuncOf(jsGetMediaStreaming))
	js.Global().Set("decryptMedia", js.FuncOf(jsDecryptMedia))
	
	jsLog("WASM Bridge registered")
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
	jsLog("jsGetMediaInfo: Request for ID %s", msgID)

	statsLock.Lock()
	msgRef, hasRef := downloadableMsgs[msgID]
	statsLock.Unlock()

	if !hasRef {
		jsLog("jsGetMediaInfo: NO REFERENCE found for %s", msgID)
		return ""
	}

	mediaType := whatsmeow.GetMediaType(msgRef)
	
	res := map[string]any{
		"direct_path":     msgRef.GetDirectPath(),
		"media_key":       base64.StdEncoding.EncodeToString(msgRef.GetMediaKey()),
		"file_sha256":     base64.StdEncoding.EncodeToString(msgRef.GetFileSHA256()),
		"file_enc_sha256": base64.StdEncoding.EncodeToString(msgRef.GetFileEncSHA256()),
		"media_type":      string(mediaType),
	}

	jsLog("jsGetMediaInfo: Returning public metadata for %s", msgID)
	data, _ := json.Marshal(res)
	return string(data)
}

func jsGetMediaStreaming(this js.Value, args []js.Value) any {
	if len(args) < 2 || args[1].Type() != js.TypeFunction {
		return nil
	}
	msgID := args[0].String()
	callback := args[1]
	jsLog("jsGetMediaStreaming: Starting stream for %s", msgID)

	go func() {
		statsLock.Lock()
		msgRef, hasRef := downloadableMsgs[msgID]
		statsLock.Unlock()

		if !hasRef || wasmBridgeClient == nil {
			jsLog("jsGetMediaStreaming: FAILED - RefFound=%v, ClientReady=%v", hasRef, wasmBridgeClient != nil)
			obj := js.Global().Get("Object").New()
			obj.Set("event", "error")
			obj.Set("message", "Media reference not found")
			callback.Invoke(obj)
			return
		}

		data, err := wasmBridgeClient.Download(context.Background(), msgRef)

		if err != nil {
			jsLog("jsGetMediaStreaming: Download FAILED for %s: %v", msgID, err)
			obj := js.Global().Get("Object").New()
			obj.Set("event", "error")
			obj.Set("message", err.Error())
			callback.Invoke(obj)
			return
		}

		total := len(data)
		jsLog("jsGetMediaStreaming: Download SUCCESS for %s (%d bytes). Streaming chunks...", msgID, total)
		
		const chunkSize = 64 * 1024 // 64KB chunks
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
			obj.Set("current", end)
			obj.Set("total", total)
			callback.Invoke(obj)
		}

		jsLog("jsGetMediaStreaming: Stream COMPLETE for %s", msgID)
		obj := js.Global().Get("Object").New()
		obj.Set("event", "complete")
		callback.Invoke(obj)
	}()

	return nil
}

func jsDecryptMedia(this js.Value, args []js.Value) any {
	if len(args) < 3 {
		jsLog("decryptMedia: Missing arguments")
		return ""
	}
	
	encDataB64 := args[0].String()
	mediaKeyB64 := args[1].String()
	mediaTypeStr := args[2].String()

	encData, _ := base64.StdEncoding.DecodeString(encDataB64)
	mediaKey, _ := base64.StdEncoding.DecodeString(mediaKeyB64)
	mediaType := whatsmeow.MediaType(mediaTypeStr)

	if len(encData) <= 10 {
		jsLog("decryptMedia: Data too short")
		return ""
	}

	mediaKeyExpanded := hkdfutil.SHA256(mediaKey, nil, []byte(mediaType), 112)
	iv := mediaKeyExpanded[:16]
	cipherKey := mediaKeyExpanded[16:48]

	ciphertext := encData[:len(encData)-10]

	data, err := cbcutil.Decrypt(cipherKey, iv, ciphertext)
	if err != nil {
		jsLog("decryptMedia: Decryption FAILED: %v", err)
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
