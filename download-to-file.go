// Copyright (c) 2024 Tulir Asokan
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
	"os"
	"strings"
	"time"

	"go.mau.fi/util/fallocate"
	"go.mau.fi/util/retryafter"

	"go.mau.fi/whatsmeow/proto/waMediaTransport"
	"go.mau.fi/whatsmeow/util/cbcutil"
)

type File interface {
	io.Reader
	io.Writer
	io.Seeker
	io.ReaderAt
	io.WriterAt
	Truncate(size int64) error
	Stat() (os.FileInfo, error)
}

// DownloadToFile downloads the attachment from the given protobuf message.
//
// This is otherwise identical to [Download], but writes the attachment to a file instead of returning it as a byte slice.
func (cli *Client) DownloadToFile(ctx context.Context, msg DownloadableMessage, file File) error {
	if cli == nil {
		return ErrClientIsNil
	}
	mediaType := GetMediaType(msg)
	if mediaType == "" {
		return fmt.Errorf("%w %T", ErrUnknownMediaType, msg)
	}
	urlable, ok := msg.(downloadableMessageWithURL)
	var url string
	var isWebWhatsappNetURL bool
	if ok {
		url = urlable.GetURL()
		isWebWhatsappNetURL = strings.HasPrefix(url, "https://web.whatsapp.net")
	}
	if len(url) > 0 && !isWebWhatsappNetURL {
		return cli.downloadAndDecryptToFile(ctx, url, msg.GetMediaKey(), mediaType, getSize(msg), msg.GetFileEncSHA256(), msg.GetFileSHA256(), file)
	} else if len(msg.GetDirectPath()) > 0 {
		return cli.DownloadMediaWithPathToFile(ctx, msg.GetDirectPath(), msg.GetFileEncSHA256(), msg.GetFileSHA256(), msg.GetMediaKey(), getSize(msg), mediaType, mediaTypeToMMSType[mediaType], file)
	} else {
		if isWebWhatsappNetURL {
			cli.Log.Warnf("Got a media message with a web.whatsapp.net URL (%s) and no direct path", url)
		}
		return ErrNoURLPresent
	}
}

func (cli *Client) DownloadFBToFile(
	ctx context.Context,
	transport *waMediaTransport.WAMediaTransport_Integral,
	mediaType MediaType,
	file File,
) error {
	return cli.DownloadMediaWithPathToFile(ctx, transport.GetDirectPath(), transport.GetFileEncSHA256(), transport.GetFileSHA256(), transport.GetMediaKey(), -1, mediaType, mediaTypeToMMSType[mediaType], file)
}

func (cli *Client) DownloadMediaWithPathToFile(
	ctx context.Context,
	directPath string,
	encFileHash, fileHash, mediaKey []byte,
	fileLength int,
	mediaType MediaType,
	mmsType string,
	file File,
) error {
	mediaConn, err := cli.refreshMediaConn(ctx, false)
	if err != nil {
		return fmt.Errorf("failed to refresh media connections: %w", err)
	}
	if len(mmsType) == 0 {
		mmsType = mediaTypeToMMSType[mediaType]
	}
	for i, host := range mediaConn.Hosts {
		// TODO omit hash for unencrypted media?
		mediaURL := fmt.Sprintf("https://%s%s&hash=%s&mms-type=%s&__wa-mms=", host.Hostname, directPath, base64.URLEncoding.EncodeToString(encFileHash), mmsType)
		err = cli.downloadAndDecryptToFile(ctx, mediaURL, mediaKey, mediaType, fileLength, encFileHash, fileHash, file)
		if err == nil ||
			errors.Is(err, ErrFileLengthMismatch) ||
			errors.Is(err, ErrInvalidMediaSHA256) ||
			errors.Is(err, ErrMediaDownloadFailedWith403) ||
			errors.Is(err, ErrMediaDownloadFailedWith404) ||
			errors.Is(err, ErrMediaDownloadFailedWith410) ||
			errors.Is(err, context.Canceled) {
			return err
		} else if i >= len(mediaConn.Hosts)-1 {
			return fmt.Errorf("failed to download media from last host: %w", err)
		}
		cli.Log.Warnf("Failed to download media: %s, trying with next host...", err)
	}
	return err
}

func (cli *Client) downloadAndDecryptToFile(
	ctx context.Context,
	url string,
	mediaKey []byte,
	appInfo MediaType,
	fileLength int,
	fileEncSHA256, fileSHA256 []byte,
	file File,
) error {
	iv, cipherKey, macKey, _ := getMediaKeys(mediaKey, appInfo)
	hasher := sha256.New()
	if mac, err := cli.downloadPossiblyEncryptedMediaWithRetriesToFile(ctx, url, fileEncSHA256, file); err != nil {
		return err
	} else if mediaKey == nil && fileEncSHA256 == nil && mac == nil {
		// Unencrypted media, just return the downloaded data
		return nil
	} else if err = validateMediaFile(file, iv, macKey, mac); err != nil {
		return err
	} else if _, err = file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start of file after validating mac: %w", err)
	} else if err = cbcutil.DecryptFile(cipherKey, iv, file); err != nil {
		return fmt.Errorf("failed to decrypt file: %w", err)
	} else if ReturnDownloadWarnings {
		if info, err := file.Stat(); err != nil {
			return fmt.Errorf("failed to stat file: %w", err)
		} else if fileLength >= 0 && info.Size() != int64(fileLength) {
			return fmt.Errorf("%w: expected %d, got %d", ErrFileLengthMismatch, fileLength, info.Size())
		} else if _, err = file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek to start of file after decrypting: %w", err)
		} else if _, err = io.Copy(hasher, file); err != nil {
			return fmt.Errorf("failed to hash file: %w", err)
		} else if !hmac.Equal(fileSHA256, hasher.Sum(nil)) {
			return ErrInvalidMediaSHA256
		}
	}
	return nil
}

func (cli *Client) downloadPossiblyEncryptedMediaWithRetriesToFile(ctx context.Context, url string, checksum []byte, file File) (mac []byte, err error) {
	for retryNum := 0; retryNum < 5; retryNum++ {
		if checksum == nil {
			_, _, err = cli.downloadMediaToFile(ctx, url, file)
		} else {
			mac, err = cli.downloadEncryptedMediaToFile(ctx, url, checksum, file)
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
		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("failed to seek to start of file to retry download: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryDuration):
		}
	}
	return
}

func (cli *Client) downloadMediaToFile(ctx context.Context, url string, file io.Writer) (int64, []byte, error) {
	resp, err := cli.doMediaDownloadRequest(ctx, url)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	osFile, ok := file.(*os.File)
	if ok && resp.ContentLength > 0 {
		err = fallocate.Fallocate(osFile, int(resp.ContentLength))
		if err != nil {
			return 0, nil, fmt.Errorf("failed to preallocate file: %w", err)
		}
	}
	hasher := sha256.New()
	n, err := io.Copy(file, io.TeeReader(resp.Body, hasher))
	return n, hasher.Sum(nil), err
}

func (cli *Client) downloadEncryptedMediaToFile(ctx context.Context, url string, checksum []byte, file File) ([]byte, error) {
	size, hash, err := cli.downloadMediaToFile(ctx, url, file)
	if err != nil {
		return nil, err
	} else if size <= mediaHMACLength {
		return nil, ErrTooShortFile
	} else if len(checksum) == 32 && !hmac.Equal(checksum, hash) {
		return nil, ErrInvalidMediaEncSHA256
	}
	mac := make([]byte, mediaHMACLength)
	_, err = file.ReadAt(mac, size-mediaHMACLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read MAC from file: %w", err)
	}
	err = file.Truncate(size - mediaHMACLength)
	if err != nil {
		return nil, fmt.Errorf("failed to truncate file to remove MAC: %w", err)
	}
	return mac, nil
}

func validateMediaFile(file io.ReadSeeker, iv, macKey, mac []byte) error {
	h := hmac.New(sha256.New, macKey)
	h.Write(iv)
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to start of file: %w", err)
	}
	_, err = io.Copy(h, file)
	if err != nil {
		return fmt.Errorf("failed to hash file: %w", err)
	}
	if !hmac.Equal(h.Sum(nil)[:mediaHMACLength], mac) {
		return ErrInvalidMediaHMAC
	}
	return nil
}
