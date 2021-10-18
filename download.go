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
	"io/ioutil"
	"net/http"

	"golang.org/x/crypto/hkdf"
	"google.golang.org/protobuf/reflect/protoreflect"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/cbcutil"
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
)

// DownloadableMessage represents a protobuf message that contains attachment info.
type DownloadableMessage interface {
	GetDirectPath() string
	GetMediaKey() []byte
	GetFileSha256() []byte
	GetFileEncSha256() []byte
	GetFileLength() uint64
	ProtoReflect() protoreflect.Message
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

	"HistorySyncNotification": MediaHistory,
}

var mediaTypeToMMSType = map[MediaType]string{
	MediaHistory: "md-msg-hist",
}

func (cli *Client) DownloadAny(msg *waProto.Message) (data []byte, err error) {
	downloadables := []DownloadableMessage{msg.GetImageMessage(), msg.GetAudioMessage(), msg.GetVideoMessage(), msg.GetDocumentMessage(), msg.GetStickerMessage()}
	for _, downloadable := range downloadables {
		if downloadable != nil {
			return cli.Download(downloadable)
		}
	}
	return nil, ErrNothingDownloadableFound
}

// Download downloads the attachment from the given protobuf message.
func (cli *Client) Download(msg DownloadableMessage) (data []byte, err error) {
	mediaType, ok := classToMediaType[msg.ProtoReflect().Descriptor().Name()]
	if !ok {
		return nil, fmt.Errorf("%w '%s'", ErrUnknownMediaType, string(msg.ProtoReflect().Descriptor().Name()))
	}
	urlable, ok := msg.(downloadableMessageWithURL)
	if ok && len(urlable.GetUrl()) > 0 {
		return downloadAndDecrypt(urlable.GetUrl(), msg.GetMediaKey(), mediaType, int(msg.GetFileLength()), msg.GetFileEncSha256(), msg.GetFileSha256())
	} else if len(msg.GetDirectPath()) > 0 {
		return cli.downloadMediaWithPath(msg.GetDirectPath(), msg.GetFileEncSha256(), msg.GetFileSha256(), msg.GetMediaKey(), int(msg.GetFileLength()), mediaType, mediaTypeToMMSType[mediaType])
	} else {
		return nil, ErrNoURLPresent
	}
}

func (cli *Client) downloadMediaWithPath(directPath string, encFileHash, fileHash, mediaKey []byte, fileLength int, mediaType MediaType, mmsType string) (data []byte, err error) {
	err = cli.refreshMediaConn(false)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh media connections: %w", err)
	}
	for i, host := range cli.mediaConn.Hosts {
		mediaURL := fmt.Sprintf("https://%s%s&hash=%s&mms-type=%s&__wa-mms=", host.Hostname, directPath, base64.URLEncoding.EncodeToString(encFileHash), mmsType)
		data, err = downloadAndDecrypt(mediaURL, mediaKey, mediaType, fileLength, encFileHash, fileHash)
		// TODO there are probably some errors that shouldn't retry
		if err != nil {
			if i >= len(cli.mediaConn.Hosts)-1 {
				return nil, fmt.Errorf("failed to download media from last host: %w", err)
			}
			cli.Log.Warnf("Failed to download media: %s, trying with next host...", err)
		}
	}
	return
}

func downloadAndDecrypt(url string, mediaKey []byte, appInfo MediaType, fileLength int, fileEncSha256, fileSha256 []byte) (data []byte, err error) {
	var ciphertext, mac, iv, cipherKey, macKey []byte
	if ciphertext, mac, err = downloadEncryptedMedia(url, fileEncSha256); err != nil {

	} else if iv, cipherKey, macKey, _, err = getMediaKeys(mediaKey, appInfo); err != nil {

	} else if err = validateMedia(iv, ciphertext, macKey, mac); err != nil {

	} else if data, err = cbcutil.Decrypt(cipherKey, iv, ciphertext); err != nil {
		err = fmt.Errorf("failed to decrypt file: %w", err)
	} else if len(data) != fileLength {
		err = fmt.Errorf("%w: expected %d, got %d", ErrFileLengthMismatch, fileLength, len(data))
	} else if len(fileSha256) == 32 && sha256.Sum256(data) != *(*[32]byte)(fileSha256) {
		err = ErrInvalidMediaSHA256
	}
	return
}

func getMediaKeys(mediaKey []byte, appInfo MediaType) (iv, cipherKey, macKey, refKey []byte, err error) {
	h := hkdf.New(sha256.New, mediaKey, nil, []byte(appInfo))
	mediaKeyExpanded := make([]byte, 112)
	_, err = io.ReadFull(h, mediaKeyExpanded)
	if err != nil {
		err = fmt.Errorf("failed to expand media key: %w", err)
		return
	}
	return mediaKeyExpanded[:16], mediaKeyExpanded[16:48], mediaKeyExpanded[48:80], mediaKeyExpanded[80:], nil
}

func downloadEncryptedMedia(url string, checksum []byte) (file, mac []byte, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, nil, ErrMediaDownloadFailedWith404
		}
		if resp.StatusCode == http.StatusGone {
			return nil, nil, ErrMediaDownloadFailedWith410
		}
		return nil, nil, fmt.Errorf("download failed with status code %d", resp.StatusCode)
	}
	if resp.ContentLength <= 10 {
		return nil, nil, ErrTooShortFile
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	} else if len(checksum) == 32 && sha256.Sum256(data) != *(*[32]byte)(checksum) {
		return nil, nil, ErrInvalidMediaEncSHA256
	}

	return data[:len(data)-10], data[len(data)-10:], nil
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
