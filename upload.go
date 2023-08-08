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

	fileEncSHA256 := sha256.Sum256(dataToUpload)
	resp.FileEncSHA256 = fileEncSHA256[:]

	var mediaConn *MediaConn
	mediaConn, err = cli.refreshMediaConn(false)
	if err != nil {
		err = fmt.Errorf("failed to refresh media connections: %w", err)
		return
	}

	token := base64.URLEncoding.EncodeToString(resp.FileEncSHA256)
	q := url.Values{
		"auth":  []string{mediaConn.Auth},
		"token": []string{token},
	}
	mmsType := mediaTypeToMMSType[appInfo]
	uploadURL := url.URL{
		Scheme:   "https",
		Host:     mediaConn.Hosts[0].Hostname,
		Path:     fmt.Sprintf("/mms/%s/%s", mmsType, token),
		RawQuery: q.Encode(),
	}

	var req *http.Request
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, uploadURL.String(), bytes.NewReader(dataToUpload))
	if err != nil {
		err = fmt.Errorf("failed to prepare request: %w", err)
		return
	}

	req.Header.Set("Origin", socket.Origin)
	req.Header.Set("Referer", socket.Origin+"/")

	var httpResp *http.Response
	httpResp, err = cli.http.Do(req)
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
	return
}
