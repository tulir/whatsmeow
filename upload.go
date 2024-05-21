// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"go.mau.fi/util/random"

	"go.mau.fi/whatsmeow/socket"
	"go.mau.fi/whatsmeow/util/cbcutil"
)

// UploadResponse contains the data from the attachment upload, which can be put into a message to send the attachment.
type UploadResponse struct {
	URL        string `json:"url"`
	DirectPath string `json:"direct_path"`
	Handle     string `json:"handle"`
	ObjectID   string `json:"object_id"`

	MediaKey      []byte `json:"-"`
	FileEncSHA256 []byte `json:"-"`
	FileSHA256    []byte `json:"-"`
	FileLength    uint64 `json:"-"`
}

// Upload uploads the given attachment to WhatsApp servers.
//
// You should copy the fields in the response to the corresponding fields in a protobuf message.
//
// For example, to send an image:
//
//	resp, err := cli.Upload(context.Background(), yourImageBytes, whatsmeow.MediaImage)
//	// handle error
//
//	imageMsg := &waProto.ImageMessage{
//		Caption:  proto.String("Hello, world!"),
//		Mimetype: proto.String("image/png"), // replace this with the actual mime type
//		// you can also optionally add other fields like ContextInfo and JpegThumbnail here
//
//		Url:           &resp.URL,
//		DirectPath:    &resp.DirectPath,
//		MediaKey:      resp.MediaKey,
//		FileEncSha256: resp.FileEncSHA256,
//		FileSha256:    resp.FileSha256,
//		FileLength:    &resp.FileLength,
//	}
//	_, err = cli.SendMessage(context.Background(), targetJID, &waProto.Message{
//		ImageMessage: imageMsg,
//	})
//	// handle error again
//
// The same applies to the other message types like DocumentMessage, just replace the struct type and Message field name.
func (cli *Client) Upload(ctx context.Context, plaintext []byte, appInfo MediaType) (resp UploadResponse, err error) {
	resp.FileLength = uint64(len(plaintext))
	resp.MediaKey = random.Bytes(32)

	plaintextSHA256 := sha256.Sum256(plaintext)
	resp.FileSHA256 = plaintextSHA256[:]

	iv, cipherKey, macKey, _ := getMediaKeys(resp.MediaKey, appInfo)

	var ciphertext []byte
	ciphertext, err = cbcutil.Encrypt(cipherKey, iv, plaintext)
	if err != nil {
		err = fmt.Errorf("failed to encrypt file: %w", err)
		return
	}

	h := hmac.New(sha256.New, macKey)
	h.Write(iv)
	h.Write(ciphertext)
	dataToUpload := append(ciphertext, h.Sum(nil)[:10]...)

	dataHash := sha256.Sum256(dataToUpload)
	resp.FileEncSHA256 = dataHash[:]

	err = cli.rawUpload(ctx, dataToUpload, resp.FileEncSHA256, appInfo, false, &resp)
	return
}

// UploadNewsletter uploads the given attachment to WhatsApp servers without encrypting it first.
//
// Newsletter media works mostly the same way as normal media, with a few differences:
// * Since it's unencrypted, there's no MediaKey or FileEncSha256 fields.
// * There's a "media handle" that needs to be passed in SendRequestExtra.
//
// Example:
//
//	resp, err := cli.UploadNewsletter(context.Background(), yourImageBytes, whatsmeow.MediaImage)
//	// handle error
//
//	imageMsg := &waProto.ImageMessage{
//		// Caption, mime type and other such fields work like normal
//		Caption:  proto.String("Hello, world!"),
//		Mimetype: proto.String("image/png"),
//
//		// URL and direct path are also there like normal media
//		Url:        &resp.URL,
//		DirectPath: &resp.DirectPath,
//		FileSha256: resp.FileSha256,
//		FileLength: &resp.FileLength,
//		// Newsletter media isn't encrypted, so the media key and file enc sha fields are not applicable
//	}
//	_, err = cli.SendMessage(context.Background(), newsletterJID, &waProto.Message{
//		ImageMessage: imageMsg,
//	}, whatsmeow.SendRequestExtra{
//		// Unlike normal media, newsletters also include a "media handle" in the send request.
//		MediaHandle: resp.Handle,
//	})
//	// handle error again
func (cli *Client) UploadNewsletter(ctx context.Context, data []byte, appInfo MediaType) (resp UploadResponse, err error) {
	resp.FileLength = uint64(len(data))
	hash := sha256.Sum256(data)
	resp.FileSHA256 = hash[:]
	err = cli.rawUpload(ctx, data, resp.FileSHA256, appInfo, true, &resp)
	return
}

func (cli *Client) rawUpload(ctx context.Context, dataToUpload, fileHash []byte, appInfo MediaType, newsletter bool, resp *UploadResponse) error {
	mediaConn, err := cli.refreshMediaConn(false)
	if err != nil {
		return fmt.Errorf("failed to refresh media connections: %w", err)
	}

	token := base64.URLEncoding.EncodeToString(fileHash)
	q := url.Values{
		"auth":  []string{mediaConn.Auth},
		"token": []string{token},
	}
	mmsType := mediaTypeToMMSType[appInfo]
	uploadPrefix := "mms"
	if cli.MessengerConfig != nil {
		uploadPrefix = "wa-msgr/mms"
		// Messenger upload only allows voice messages, not audio files
		if mmsType == "audio" {
			mmsType = "ptt"
		}
	}
	if newsletter {
		mmsType = fmt.Sprintf("newsletter-%s", mmsType)
		uploadPrefix = "newsletter"
	}
	var host string
	// Hacky hack to prefer last option (rupload.facebook.com) for messenger uploads.
	// For some reason, the primary host doesn't work, even though it has the <upload/> tag.
	if cli.MessengerConfig != nil {
		host = mediaConn.Hosts[len(mediaConn.Hosts)-1].Hostname
	} else {
		host = mediaConn.Hosts[0].Hostname
	}
	uploadURL := url.URL{
		Scheme:   "https",
		Host:     host,
		Path:     fmt.Sprintf("/%s/%s/%s", uploadPrefix, mmsType, token),
		RawQuery: q.Encode(),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL.String(), bytes.NewReader(dataToUpload))
	if err != nil {
		return fmt.Errorf("failed to prepare request: %w", err)
	}

	req.Header.Set("Origin", socket.Origin)
	req.Header.Set("Referer", socket.Origin+"/")

	httpResp, err := cli.http.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to execute request: %w", err)
	} else if httpResp.StatusCode != http.StatusOK {
		err = fmt.Errorf("upload failed with status code %d", httpResp.StatusCode)
	} else if err = json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		err = fmt.Errorf("failed to parse upload response: %w", err)
	}
	if httpResp != nil {
		_ = httpResp.Body.Close()
	}
	return err
}
