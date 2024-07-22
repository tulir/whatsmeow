// Copyright (c) 2024 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"go.mau.fi/whatsmeow/socket"
	"go.mau.fi/whatsmeow/store"
)

var clientVersionRegex = regexp.MustCompile(`"client_revision":(\d+),`)

// GetLatestVersion returns the latest version number from web.whatsapp.com.
//
// After fetching, you can update the version to use using store.SetWAVersion, e.g.
//
//	latestVer, err := GetLatestVersion(nil)
//	if err != nil {
//		return err
//	}
//	store.SetWAVersion(*latestVer)
func GetLatestVersion(httpClient *http.Client) (*store.WAVersionContainer, error) {
	req, err := http.NewRequest(http.MethodGet, socket.Origin, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	data, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	} else if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected response with status %d: %s", resp.StatusCode, data)
	} else if match := clientVersionRegex.FindSubmatch(data); len(match) == 0 {
		return nil, fmt.Errorf("version number not found")
	} else if parsedVer, err := strconv.ParseInt(string(match[1]), 10, 64); err != nil {
		return nil, fmt.Errorf("failed to parse version number: %w", err)
	} else {
		return &store.WAVersionContainer{2, 3000, uint32(parsedVer)}, nil
	}
}
