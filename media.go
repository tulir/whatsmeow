// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"golang.org/x/crypto/hkdf"

	waBinary "go.mau.fi/whatsmeow/binary"
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

var mediaTypeMap = map[MediaType]string{
	MediaImage:    "/mms/image",
	MediaVideo:    "/mms/video",
	MediaDocument: "/mms/document",
	MediaAudio:    "/mms/audio",
}

// Download downloads and decrypts a file from WhatsApp.
func Download(url string, mediaKey []byte, appInfo MediaType, fileLength int) (data []byte, err error) {
	if url == "" {
		err = ErrNoURLPresent
		return
	}
	var file, mac []byte
	file, mac, err = downloadMedia(url)
	if err != nil {
		return
	}
	var iv, cipherKey, macKey []byte
	iv, cipherKey, macKey, _, err = getMediaKeys(mediaKey, appInfo)
	if err != nil {
		return
	}
	data, err = cbcutil.Decrypt(cipherKey, iv, file)
	if err == nil && len(data) != fileLength {
		err = fmt.Errorf("%w: expected %d, got %d", ErrFileLengthMismatch, fileLength, len(data))
	} else if err == nil {
		err = validateMedia(iv, file, macKey, mac)
	}
	return
}

func validateMedia(iv, file, macKey, mac []byte) error {
	h := hmac.New(sha256.New, macKey)
	n, err := h.Write(append(iv, file...))
	if err != nil {
		return err
	}
	if n < 10 {
		return ErrInvalidHashLength
	}
	if !hmac.Equal(h.Sum(nil)[:10], mac) {
		return ErrInvalidMediaHMAC
	}
	return nil
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

func downloadMedia(url string) (file, mac []byte, err error) {
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
	}

	n := len(data)
	return data[:n-10], data[n-10 : n], nil
}

//type MediaConnIP struct {
//	IP4 net.IP
//	IP6 net.IP
//}

// MediaConnHost represents a single host to download media from.
type MediaConnHost struct {
	Hostname string
	//IPs      []MediaConnIP
}

// MediaConn contains a list of WhatsApp servers from which attachments can be downloaded from.
type MediaConn struct {
	Auth       string
	AuthTTL    int
	TTL        int
	MaxBuckets int
	FetchedAt  time.Time
	Hosts      []MediaConnHost
}

// Expiry returns the time when the MediaConn expires.
func (mc *MediaConn) Expiry() time.Time {
	return mc.FetchedAt.Add(time.Duration(mc.TTL) * time.Second)
}

//func (wac *Conn) Upload(reader io.Reader, appInfo MediaType) (downloadURL string, mediaKey, fileEncSha256, fileSha256 []byte, fileLength uint64, err error) {
//	data, err := ioutil.ReadAll(reader)
//	if err != nil {
//		return "", nil, nil, nil, 0, err
//	}
//
//	mediaKey = make([]byte, 32)
//	rand.Read(mediaKey)
//
//	iv, cipherKey, macKey, _, err := getMediaKeys(mediaKey, appInfo)
//	if err != nil {
//		return "", nil, nil, nil, 0, err
//	}
//
//	enc, err := cbcutil.Encrypt(cipherKey, iv, data)
//	if err != nil {
//		return "", nil, nil, nil, 0, err
//	}
//
//	fileLength = uint64(len(data))
//
//	h := hmac.New(sha256.New, macKey)
//	h.Write(append(iv, enc...))
//	mac := h.Sum(nil)[:10]
//
//	sha := sha256.New()
//	sha.Write(data)
//	fileSha256 = sha.Sum(nil)
//
//	sha.Reset()
//	sha.Write(append(enc, mac...))
//	fileEncSha256 = sha.Sum(nil)
//
//	hostname, auth, _, err := wac.queryMediaConn()
//	if err != nil {
//		return "", nil, nil, nil, 0, err
//	}
//
//	token := base64.URLEncoding.EncodeToString(fileEncSha256)
//	q := url.Values{
//		"auth":  []string{auth},
//		"token": []string{token},
//	}
//	path := mediaTypeMap[appInfo]
//	uploadURL := url.URL{
//		Scheme:   "https",
//		Host:     hostname,
//		Path:     fmt.Sprintf("%s/%s", path, token),
//		RawQuery: q.Encode(),
//	}
//
//	body := bytes.NewReader(append(enc, mac...))
//
//	req, err := http.NewRequest(http.MethodPost, uploadURL.String(), body)
//	if err != nil {
//		return "", nil, nil, nil, 0, err
//	}
//
//	req.Header.Set("Origin", "https://web.whatsapp.com")
//	req.Header.Set("Referer", "https://web.whatsapp.com/")
//
//	client := &http.Client{}
//	// Submit the request
//	res, err := client.Do(req)
//	if err != nil {
//		return "", nil, nil, nil, 0, err
//	}
//
//	if res.StatusCode != http.StatusOK {
//		return "", nil, nil, nil, 0, fmt.Errorf("upload failed with status code %d", res.StatusCode)
//	}
//
//	var jsonRes map[string]string
//	if err := json.NewDecoder(res.Body).Decode(&jsonRes); err != nil {
//		return "", nil, nil, nil, 0, err
//	}
//
//	return jsonRes["url"], mediaKey, fileEncSha256, fileSha256, fileLength, nil
//}

func (cli *Client) downloadMedia(directPath string, encFileHash, mediaKey []byte, fileLength int, mediaType MediaType, mmsType string) (data []byte, err error) {
	err = cli.refreshMediaConn(false)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh media connections: %w", err)
	}
	for i, host := range cli.mediaConn.Hosts {
		url := fmt.Sprintf("https://%s%s&hash=%s&mms-type=%s&__wa-mms=", host.Hostname, directPath, base64.URLEncoding.EncodeToString(encFileHash), mmsType)
		data, err = Download(url, mediaKey, mediaType, fileLength)
		if errors.Is(err, ErrInvalidMediaHMAC) {
			err = nil
		}
		if err != nil {
			if i >= len(cli.mediaConn.Hosts)-1 {
				return nil, fmt.Errorf("failed to download media from last host: %w", err)
			}
			cli.Log.Warnf("Failed to download media: %s, trying with next host...", err)
		}
	}
	return
}

func (cli *Client) refreshMediaConn(force bool) error {
	cli.mediaConnLock.Lock()
	defer cli.mediaConnLock.Unlock()
	if cli.mediaConn == nil || force || time.Now().After(cli.mediaConn.Expiry()) {
		var err error
		cli.mediaConn, err = cli.queryMediaConn()
		if err != nil {
			return err
		}
	}
	return nil
}

func (cli *Client) queryMediaConn() (*MediaConn, error) {
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:m",
		Type:      "set",
		To:        waBinary.ServerJID,
		Content:   []waBinary.Node{{Tag: "media_conn"}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query media connections: %w", err)
	} else if len(resp.GetChildren()) == 0 || resp.GetChildren()[0].Tag != "media_conn" {
		return nil, fmt.Errorf("failed to query media connections: unexpected child tag")
	}
	respMC := resp.GetChildren()[0]
	var mc MediaConn
	ag := respMC.AttrGetter()
	mc.FetchedAt = time.Now()
	mc.Auth = ag.String("auth")
	mc.TTL = ag.Int("ttl")
	mc.AuthTTL = ag.Int("auth_ttl")
	mc.MaxBuckets = ag.Int("max_buckets")
	if !ag.OK() {
		return nil, fmt.Errorf("failed to parse media connections: %+v", ag.Errors)
	}
	for _, child := range respMC.GetChildren() {
		if child.Tag != "host" {
			cli.Log.Warnf("Unexpected child in media_conn element: %s", child.XMLString())
			continue
		}
		cag := child.AttrGetter()
		mc.Hosts = append(mc.Hosts, MediaConnHost{
			Hostname: cag.String("hostname"),
		})
		if !cag.OK() {
			return nil, fmt.Errorf("failed to parse media connection host: %+v", ag.Errors)
		}
	}
	return &mc, nil
}
