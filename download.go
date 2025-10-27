// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"go.mau.fi/util/retryafter"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/proto/waMediaTransport"
	"go.mau.fi/whatsmeow/proto/waServerSync"
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

	MediaStickerPack   MediaType = "WhatsApp Sticker Pack Keys"
	MediaLinkThumbnail MediaType = "WhatsApp Link Thumbnail Keys"
)

// DownloadableMessage represents a protobuf message that contains attachment info.
//
// All of the downloadable messages inside a Message struct implement this interface
// (ImageMessage, VideoMessage, AudioMessage, DocumentMessage, StickerMessage).
type DownloadableMessage interface {
	GetDirectPath() string
	GetMediaKey() []byte
	GetFileSHA256() []byte
	GetFileEncSHA256() []byte
}

type MediaTypeable interface {
	GetMediaType() MediaType
}

// DownloadableThumbnail represents a protobuf message that contains a thumbnail attachment.
//
// This is primarily meant for link preview thumbnails in ExtendedTextMessage.
type DownloadableThumbnail interface {
	proto.Message
	GetThumbnailDirectPath() string
	GetThumbnailSHA256() []byte
	GetThumbnailEncSHA256() []byte
	GetMediaKey() []byte
}

// All the message types that are intended to be downloadable
var (
	_ DownloadableMessage   = (*waE2E.ImageMessage)(nil)
	_ DownloadableMessage   = (*waE2E.AudioMessage)(nil)
	_ DownloadableMessage   = (*waE2E.VideoMessage)(nil)
	_ DownloadableMessage   = (*waE2E.DocumentMessage)(nil)
	_ DownloadableMessage   = (*waE2E.StickerMessage)(nil)
	_ DownloadableMessage   = (*waE2E.StickerPackMessage)(nil)
	_ DownloadableMessage   = (*waHistorySync.StickerMetadata)(nil)
	_ DownloadableMessage   = (*waE2E.HistorySyncNotification)(nil)
	_ DownloadableMessage   = (*waServerSync.ExternalBlobReference)(nil)
	_ DownloadableThumbnail = (*waE2E.ExtendedTextMessage)(nil)
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
	GetURL() string
}

var classToMediaType = map[protoreflect.Name]MediaType{
	"ImageMessage":    MediaImage,
	"AudioMessage":    MediaAudio,
	"VideoMessage":    MediaVideo,
	"DocumentMessage": MediaDocument,
	"StickerMessage":  MediaImage,
	"StickerMetadata": MediaImage,

	"StickerPackMessage":      MediaStickerPack,
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

	MediaStickerPack:   "sticker-pack",
	MediaLinkThumbnail: "thumbnail-link",
}

// DownloadAny loops through the downloadable parts of the given message and downloads the first non-nil item.
//
// Deprecated: it's recommended to find the specific message type you want to download manually and use the Download method instead.
func (cli *Client) DownloadAny(ctx context.Context, msg *waE2E.Message) (data []byte, err error) {
	if msg == nil {
		return nil, ErrNothingDownloadableFound
	}
	switch {
	case msg.ImageMessage != nil:
		return cli.Download(ctx, msg.ImageMessage)
	case msg.VideoMessage != nil:
		return cli.Download(ctx, msg.VideoMessage)
	case msg.AudioMessage != nil:
		return cli.Download(ctx, msg.AudioMessage)
	case msg.DocumentMessage != nil:
		return cli.Download(ctx, msg.DocumentMessage)
	case msg.StickerMessage != nil:
		return cli.Download(ctx, msg.StickerMessage)
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

// ReturnDownloadWarnings controls whether the Download function returns non-fatal validation warnings.
// Currently, these include [ErrFileLengthMismatch] and [ErrInvalidMediaSHA256].
var ReturnDownloadWarnings = true

// DownloadThumbnail downloads a thumbnail from a message.
//
// This is primarily intended for downloading link preview thumbnails, which are in ExtendedTextMessage:
//
//	var msg *waE2E.Message
//	...
//	thumbnailImageBytes, err := cli.DownloadThumbnail(msg.GetExtendedTextMessage())
func (cli *Client) DownloadThumbnail(ctx context.Context, msg DownloadableThumbnail) ([]byte, error) {
	mediaType, ok := classToThumbnailMediaType[msg.ProtoReflect().Descriptor().Name()]
	if !ok {
		return nil, fmt.Errorf("%w '%s'", ErrUnknownMediaType, string(msg.ProtoReflect().Descriptor().Name()))
	} else if len(msg.GetThumbnailDirectPath()) > 0 {
		return cli.DownloadMediaWithPath(ctx, msg.GetThumbnailDirectPath(), msg.GetThumbnailEncSHA256(), msg.GetThumbnailSHA256(), msg.GetMediaKey(), -1, mediaType, mediaTypeToMMSType[mediaType])
	} else {
		return nil, ErrNoURLPresent
	}
}

// GetMediaType returns the MediaType value corresponding to the given protobuf message.
func GetMediaType(msg DownloadableMessage) MediaType {
	protoReflecter, ok := msg.(proto.Message)
	if !ok {
		mediaTypeable, ok := msg.(MediaTypeable)
		if !ok {
			return ""
		}
		return mediaTypeable.GetMediaType()
	}
	return classToMediaType[protoReflecter.ProtoReflect().Descriptor().Name()]
}

// Download downloads the attachment from the given protobuf message.
//
// The attachment is a specific part of a Message protobuf struct, not the message itself, e.g.
//
//	var msg *waE2E.Message
//	...
//	imageData, err := cli.Download(msg.GetImageMessage())
//
// You can also use DownloadAny to download the first non-nil sub-message.
func (cli *Client) Download(ctx context.Context, msg DownloadableMessage) ([]byte, error) {
	if cli == nil {
		return nil, ErrClientIsNil
	}
	mediaType := GetMediaType(msg)
	if mediaType == "" {
		return nil, fmt.Errorf("%w %T", ErrUnknownMediaType, msg)
	}
	urlable, ok := msg.(downloadableMessageWithURL)
	var url string
	var isWebWhatsappNetURL bool
	if ok {
		url = urlable.GetURL()
		isWebWhatsappNetURL = strings.HasPrefix(url, "https://web.whatsapp.net")
	}
	if len(url) > 0 && !isWebWhatsappNetURL {
		return cli.downloadAndDecrypt(ctx, url, msg.GetMediaKey(), mediaType, getSize(msg), msg.GetFileEncSHA256(), msg.GetFileSHA256())
	} else if len(msg.GetDirectPath()) > 0 {
		return cli.DownloadMediaWithPath(ctx, msg.GetDirectPath(), msg.GetFileEncSHA256(), msg.GetFileSHA256(), msg.GetMediaKey(), getSize(msg), mediaType, mediaTypeToMMSType[mediaType])
	} else {
		if isWebWhatsappNetURL {
			cli.Log.Warnf("Got a media message with a web.whatsapp.net URL (%s) and no direct path", url)
		}
		return nil, ErrNoURLPresent
	}
}

func (cli *Client) DownloadFB(
	ctx context.Context,
	transport *waMediaTransport.WAMediaTransport_Integral,
	mediaType MediaType,
) ([]byte, error) {
	return cli.DownloadMediaWithPath(ctx, transport.GetDirectPath(), transport.GetFileEncSHA256(), transport.GetFileSHA256(), transport.GetMediaKey(), -1, mediaType, mediaTypeToMMSType[mediaType])
}

// DownloadMediaWithPath downloads an attachment by manually specifying the path and encryption details.
func (cli *Client) DownloadMediaWithPath(
	ctx context.Context,
	directPath string,
	encFileHash, fileHash, mediaKey []byte,
	fileLength int,
	mediaType MediaType,
	mmsType string,
) (data []byte, err error) {
	if !strings.HasPrefix(directPath, "/") {
		return nil, fmt.Errorf("media download path does not start with slash: %s", directPath)
	}
	var mediaConn *MediaConn
	mediaConn, err = cli.refreshMediaConn(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh media connections: %w", err)
	}
	if len(mmsType) == 0 {
		mmsType = mediaTypeToMMSType[mediaType]
	}
	for i, host := range mediaConn.Hosts {
		// TODO omit hash for unencrypted media?
		mediaURL := fmt.Sprintf("https://%s%s&hash=%s&mms-type=%s&__wa-mms=", host.Hostname, directPath, base64.URLEncoding.EncodeToString(encFileHash), mmsType)
		data, err = cli.downloadAndDecrypt(ctx, mediaURL, mediaKey, mediaType, fileLength, encFileHash, fileHash)
		if err == nil ||
			errors.Is(err, ErrFileLengthMismatch) ||
			errors.Is(err, ErrInvalidMediaSHA256) ||
			errors.Is(err, ErrMediaDownloadFailedWith403) ||
			errors.Is(err, ErrMediaDownloadFailedWith404) ||
			errors.Is(err, ErrMediaDownloadFailedWith410) ||
			errors.Is(err, context.Canceled) {
			return
		} else if i >= len(mediaConn.Hosts)-1 {
			return nil, fmt.Errorf("failed to download media from last host: %w", err)
		}
		cli.Log.Warnf("Failed to download media: %s, trying with next host...", err)
	}
	return
}

func (cli *Client) downloadAndDecrypt(
	ctx context.Context,
	url string,
	mediaKey []byte,
	appInfo MediaType,
	fileLength int,
	fileEncSHA256,
	fileSHA256 []byte,
) (data []byte, err error) {
	iv, cipherKey, macKey, _ := getMediaKeys(mediaKey, appInfo)
	var ciphertext, mac []byte
	if ciphertext, mac, err = cli.downloadPossiblyEncryptedMediaWithRetries(ctx, url, fileEncSHA256); err != nil {

	} else if mediaKey == nil && fileEncSHA256 == nil && mac == nil {
		// Unencrypted media, just return the downloaded data
		data = ciphertext
	} else if err = validateMedia(iv, ciphertext, macKey, mac); err != nil {

	} else if data, err = cbcutil.Decrypt(cipherKey, iv, ciphertext); err != nil {
		err = fmt.Errorf("failed to decrypt file: %w", err)
	} else if ReturnDownloadWarnings {
		if fileLength >= 0 && len(data) != fileLength {
			err = fmt.Errorf("%w: expected %d, got %d", ErrFileLengthMismatch, fileLength, len(data))
		} else if len(fileSHA256) == 32 && sha256.Sum256(data) != *(*[32]byte)(fileSHA256) {
			err = ErrInvalidMediaSHA256
		}
	}
	return
}

func getMediaKeys(mediaKey []byte, appInfo MediaType) (iv, cipherKey, macKey, refKey []byte) {
	mediaKeyExpanded := hkdfutil.SHA256(mediaKey, nil, []byte(appInfo), 112)
	return mediaKeyExpanded[:16], mediaKeyExpanded[16:48], mediaKeyExpanded[48:80], mediaKeyExpanded[80:]
}

func shouldRetryMediaDownload(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	var netErr net.Error
	var httpErr DownloadHTTPError
	return errors.As(err, &netErr) ||
		strings.HasPrefix(err.Error(), "stream error:") || // hacky check for http2 errors
		(errors.As(err, &httpErr) && retryafter.Should(httpErr.StatusCode, true))
}

func (cli *Client) downloadPossiblyEncryptedMediaWithRetries(ctx context.Context, url string, checksum []byte) (file, mac []byte, err error) {
	for retryNum := 0; retryNum < 5; retryNum++ {
		if checksum == nil {
			file, err = cli.downloadMedia(ctx, url)
		} else {
			file, mac, err = cli.downloadEncryptedMedia(ctx, url, checksum)
		}
		if err == nil || !shouldRetryMediaDownload(err) {
			return
		}
		retryDuration := time.Duration(retryNum+1) * time.Second
		var httpErr DownloadHTTPError
		if errors.As(err, &httpErr) {
			retryDuration = retryafter.Parse(httpErr.Response.Header.Get("Retry-After"), retryDuration)
		}
		cli.Log.Warnf("Failed to download media due to network error: %v, retrying in %s...", err, retryDuration)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(retryDuration):
		}
	}
	return
}

func (cli *Client) doMediaDownloadRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare request: %w", err)
	}
	req.Header.Set("Origin", socket.Origin)
	req.Header.Set("Referer", socket.Origin+"/")
	if cli.MessengerConfig != nil {
		req.Header.Set("User-Agent", cli.MessengerConfig.UserAgent)
	}
	// TODO user agent for whatsapp downloads?
	resp, err := cli.mediaHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, DownloadHTTPError{Response: resp}
	}
	return resp, nil
}

func (cli *Client) downloadMedia(ctx context.Context, url string) ([]byte, error) {
	resp, err := cli.doMediaDownloadRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return data, err
}

const mediaHMACLength = 10

func (cli *Client) downloadEncryptedMedia(ctx context.Context, url string, checksum []byte) (file, mac []byte, err error) {
	data, err := cli.downloadMedia(ctx, url)
	if err != nil {
		return
	} else if len(data) <= mediaHMACLength {
		err = ErrTooShortFile
		return
	}
	file, mac = data[:len(data)-mediaHMACLength], data[len(data)-mediaHMACLength:]
	if len(checksum) == 32 && sha256.Sum256(data) != *(*[32]byte)(checksum) {
		err = ErrInvalidMediaEncSHA256
	}
	return
}

func validateMedia(iv, file, macKey, mac []byte) error {
	h := hmac.New(sha256.New, macKey)
	h.Write(iv)
	h.Write(file)
	if !hmac.Equal(h.Sum(nil)[:mediaHMACLength], mac) {
		return ErrInvalidMediaHMAC
	}
	return nil
}
