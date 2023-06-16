// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/socket"
	"go.mau.fi/whatsmeow/util/cbcutil"
	"go.mau.fi/whatsmeow/util/hkdfutil"
)

// MediaType represents a type of uploaded file on WhatsApp.
// The value is the key which is used as a part of generating the encryption keys.
type MediaType string

// The known media types
const (
	MediaImage    MediaType = "WhatsApp Image Keys"
	MediaVideo    MediaType = "WhatsApp Video Keys"
	MediaAudio    MediaType = "WhatsApp Audio Keys"
	MediaDocument MediaType = "WhatsApp Document Keys"
	MediaHistory  MediaType = "WhatsApp History Keys"
	MediaAppState MediaType = "WhatsApp App State Keys"

	MediaLinkThumbnail MediaType = "WhatsApp Link Thumbnail Keys"
)

// DownloadableMessage represents a protobuf message that contains attachment info.
//
// All of the downloadable messages inside a Message struct implement this interface
// (ImageMessage, VideoMessage, AudioMessage, DocumentMessage, StickerMessage).
type DownloadableMessage interface {
	proto.Message
	GetDirectPath() string
	GetMediaKey() []byte
	GetFileSha256() []byte
	GetFileEncSha256() []byte
}

// DownloadableThumbnail represents a protobuf message that contains a thumbnail attachment.
//
// This is primarily meant for link preview thumbnails in ExtendedTextMessage.
type DownloadableThumbnail interface {
	proto.Message
	GetThumbnailDirectPath() string
	GetThumbnailSha256() []byte
	GetThumbnailEncSha256() []byte
	GetMediaKey() []byte
}

// All the message types that are intended to be downloadable
var (
	_ DownloadableMessage   = (*waProto.ImageMessage)(nil)
	_ DownloadableMessage   = (*waProto.AudioMessage)(nil)
	_ DownloadableMessage   = (*waProto.VideoMessage)(nil)
	_ DownloadableMessage   = (*waProto.DocumentMessage)(nil)
	_ DownloadableMessage   = (*waProto.StickerMessage)(nil)
	_ DownloadableMessage   = (*waProto.StickerMetadata)(nil)
	_ DownloadableMessage   = (*waProto.HistorySyncNotification)(nil)
	_ DownloadableMessage   = (*waProto.ExternalBlobReference)(nil)
	_ DownloadableThumbnail = (*waProto.ExtendedTextMessage)(nil)
)

type downloadableMessageWithLength interface {
	DownloadableMessage
	GetFileLength() uint64
}

type downloadableMessageWithSizeBytes interface {
	DownloadableMessage
	GetFileSizeBytes() uint64
}

type downloadableMessageWithURL interface {
	DownloadableMessage
	GetUrl() string
}

var classToMediaType = map[protoreflect.Name]MediaType{
	"ImageMessage":    MediaImage,
	"AudioMessage":    MediaAudio,
	"VideoMessage":    MediaVideo,
	"DocumentMessage": MediaDocument,
	"StickerMessage":  MediaImage,
	"StickerMetadata": MediaImage,

	"HistorySyncNotification": MediaHistory,
	"ExternalBlobReference":   MediaAppState,
}

var classToThumbnailMediaType = map[protoreflect.Name]MediaType{
	"ExtendedTextMessage": MediaLinkThumbnail,
}

var mediaTypeToMMSType = map[MediaType]string{
	MediaImage:    "image",
	MediaAudio:    "audio",
	MediaVideo:    "video",
	MediaDocument: "document",
	MediaHistory:  "md-msg-hist",
	MediaAppState: "md-app-state",

	MediaLinkThumbnail: "thumbnail-link",
}

// DownloadAny loops through the downloadable parts of the given message and downloads the first non-nil item.
func (cli *Client) DownloadAny(msg *waProto.Message) (data []byte, err error) {
	if msg == nil {
		return nil, ErrNothingDownloadableFound
	}
	switch {
	case msg.ImageMessage != nil:
		return cli.Download(msg.ImageMessage)
	case msg.VideoMessage != nil:
		return cli.Download(msg.VideoMessage)
	case msg.AudioMessage != nil:
		return cli.Download(msg.AudioMessage)
	case msg.DocumentMessage != nil:
		return cli.Download(msg.DocumentMessage)
	case msg.StickerMessage != nil:
		return cli.Download(msg.StickerMessage)
	default:
		return nil, ErrNothingDownloadableFound
	}
}

func getSize(msg DownloadableMessage) int {
	switch sized := msg.(type) {
	case downloadableMessageWithLength:
		return int(sized.GetFileLength())
	case downloadableMessageWithSizeBytes:
		return int(sized.GetFileSizeBytes())
	default:
		return -1
	}
}

// DownloadThumbnail downloads a thumbnail from a message.
//
// This is primarily intended for downloading link preview thumbnails, which are in ExtendedTextMessage:
//
//	var msg *waProto.Message
//	...
//	thumbnailImageBytes, err := cli.DownloadThumbnail(msg.GetExtendedTextMessage())
func (cli *Client) DownloadThumbnail(msg DownloadableThumbnail) ([]byte, error) {
	mediaType, ok := classToThumbnailMediaType[msg.ProtoReflect().Descriptor().Name()]
	if !ok {
		return nil, fmt.Errorf("%w '%s'", ErrUnknownMediaType, string(msg.ProtoReflect().Descriptor().Name()))
	} else if len(msg.GetThumbnailDirectPath()) > 0 {
		return cli.DownloadMediaWithPath(msg.GetThumbnailDirectPath(), msg.GetThumbnailEncSha256(), msg.GetThumbnailSha256(), msg.GetMediaKey(), -1, mediaType, mediaTypeToMMSType[mediaType])
	} else {
		return nil, ErrNoURLPresent
	}
}

// GetMediaType returns the MediaType value corresponding to the given protobuf message.
func GetMediaType(msg DownloadableMessage) MediaType {
	return classToMediaType[msg.ProtoReflect().Descriptor().Name()]
}

// Download downloads the attachment from the given protobuf message.
//
// The attachment is a specific part of a Message protobuf struct, not the message itself, e.g.
//
//	var msg *waProto.Message
//	...
//	imageData, err := cli.Download(msg.GetImageMessage())
//
// You can also use DownloadAny to download the first non-nil sub-message.
func (cli *Client) Download(msg DownloadableMessage) ([]byte, error) {
	mediaType, ok := classToMediaType[msg.ProtoReflect().Descriptor().Name()]
	if !ok {
		return nil, fmt.Errorf("%w '%s'", ErrUnknownMediaType, string(msg.ProtoReflect().Descriptor().Name()))
	}
	urlable, ok := msg.(downloadableMessageWithURL)
	var url string
	var isWebWhatsappNetURL bool
	if ok {
		url = urlable.GetUrl()
		isWebWhatsappNetURL = strings.HasPrefix(url, "https://web.whatsapp.net")
	}
	if len(url) > 0 && !isWebWhatsappNetURL {
		return cli.downloadAndDecrypt(url, msg.GetMediaKey(), mediaType, getSize(msg), msg.GetFileEncSha256(), msg.GetFileSha256())
	} else if len(msg.GetDirectPath()) > 0 {
		return cli.DownloadMediaWithPath(msg.GetDirectPath(), msg.GetFileEncSha256(), msg.GetFileSha256(), msg.GetMediaKey(), getSize(msg), mediaType, mediaTypeToMMSType[mediaType])
	} else {
		if isWebWhatsappNetURL {
			cli.Log.Warnf("Got a media message with a web.whatsapp.net URL (%s) and no direct path", url)
		}
		return nil, ErrNoURLPresent
	}
}

// DownloadMediaWithPath downloads an attachment by manually specifying the path and encryption details.
func (cli *Client) DownloadMediaWithPath(directPath string, encFileHash, fileHash, mediaKey []byte, fileLength int, mediaType MediaType, mmsType string) (data []byte, err error) {
	var mediaConn *MediaConn
	mediaConn, err = cli.refreshMediaConn(false)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh media connections: %w", err)
	}
	if len(mmsType) == 0 {
		mmsType = mediaTypeToMMSType[mediaType]
	}
	for i, host := range mediaConn.Hosts {
		mediaURL := fmt.Sprintf("https://%s%s&hash=%s&mms-type=%s&__wa-mms=", host.Hostname, directPath, base64.URLEncoding.EncodeToString(encFileHash), mmsType)
		data, err = cli.downloadAndDecrypt(mediaURL, mediaKey, mediaType, fileLength, encFileHash, fileHash)
		// TODO there are probably some errors that shouldn't retry
		if err != nil {
			if i >= len(mediaConn.Hosts)-1 {
				return nil, fmt.Errorf("failed to download media from last host: %w", err)
			}
			cli.Log.Warnf("Failed to download media: %s, trying with next host...", err)
		}
	}
	return
}

func (cli *Client) downloadAndDecrypt(url string, mediaKey []byte, appInfo MediaType, fileLength int, fileEncSha256, fileSha256 []byte) (data []byte, err error) {
	iv, cipherKey, macKey, _ := getMediaKeys(mediaKey, appInfo)
	var ciphertext, mac []byte
	if ciphertext, mac, err = cli.downloadEncryptedMediaWithRetries(url, fileEncSha256); err != nil {

	} else if err = validateMedia(iv, ciphertext, macKey, mac); err != nil {

	} else if data, err = cbcutil.Decrypt(cipherKey, iv, ciphertext); err != nil {
		err = fmt.Errorf("failed to decrypt file: %w", err)
	} else if fileLength >= 0 && len(data) != fileLength {
		err = fmt.Errorf("%w: expected %d, got %d", ErrFileLengthMismatch, fileLength, len(data))
	} else if len(fileSha256) == 32 && sha256.Sum256(data) != *(*[32]byte)(fileSha256) {
		err = ErrInvalidMediaSHA256
	}
	return
}

func getMediaKeys(mediaKey []byte, appInfo MediaType) (iv, cipherKey, macKey, refKey []byte) {
	mediaKeyExpanded := hkdfutil.SHA256(mediaKey, nil, []byte(appInfo), 112)
	return mediaKeyExpanded[:16], mediaKeyExpanded[16:48], mediaKeyExpanded[48:80], mediaKeyExpanded[80:]
}

func (cli *Client) downloadEncryptedMediaWithRetries(url string, checksum []byte) (file, mac []byte, err error) {
	for retryNum := 0; retryNum < 5; retryNum++ {
		file, mac, err = cli.downloadEncryptedMedia(url, checksum)
		if err == nil {
			return
		}
		netErr, ok := err.(net.Error)
		if !ok {
			// Not a network error, don't retry
			return
		}
		cli.Log.Warnf("Failed to download media due to network error: %w, retrying...", netErr)
		time.Sleep(time.Duration(retryNum+1) * time.Second)
	}
	return
}

func (cli *Client) downloadEncryptedMedia(url string, checksum []byte) (file, mac []byte, err error) {
	var req *http.Request
	req, err = http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		err = fmt.Errorf("failed to prepare request: %w", err)
		return
	}
	req.Header.Set("Origin", socket.Origin)
	req.Header.Set("Referer", socket.Origin+"/")
	var resp *http.Response
	resp, err = cli.http.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden {
			err = ErrMediaDownloadFailedWith403
		} else if resp.StatusCode == http.StatusNotFound {
			err = ErrMediaDownloadFailedWith404
		} else if resp.StatusCode == http.StatusGone {
			err = ErrMediaDownloadFailedWith410
		} else {
			err = fmt.Errorf("download failed with status code %d", resp.StatusCode)
		}
		return
	}
	var data []byte
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return
	} else if len(data) <= 10 {
		err = ErrTooShortFile
		return
	}
	file, mac = data[:len(data)-10], data[len(data)-10:]
	if len(checksum) == 32 && sha256.Sum256(data) != *(*[32]byte)(checksum) {
		err = ErrInvalidMediaEncSHA256
	}
	return
}

func validateMedia(iv, file, macKey, mac []byte) error {
	h := hmac.New(sha256.New, macKey)
	h.Write(iv)
	h.Write(file)
	if !hmac.Equal(h.Sum(nil)[:10], mac) {
		return ErrInvalidMediaHMAC
	}
	return nil
}
