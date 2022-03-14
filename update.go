// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.mau.fi/whatsmeow/socket"
	"go.mau.fi/whatsmeow/store"
)

// CheckUpdateResponse is the data returned by CheckUpdate.
type CheckUpdateResponse struct {
	IsBroken       bool
	IsBelowSoft    bool
	IsBelowHard    bool
	CurrentVersion string

	ParsedVersion store.WAVersionContainer `json:"-"`
}

// CheckUpdateURL is the base URL to check for WhatsApp web updates.
const CheckUpdateURL = "https://web.whatsapp.com/check-update"

// CheckUpdate asks the WhatsApp servers if there is an update available
// (using the HTTP client and proxy settings of this whatsmeow Client instance).
func (cli *Client) CheckUpdate() (respData CheckUpdateResponse, err error) {
	return CheckUpdate(cli.http)
}

// CheckUpdate asks the WhatsApp servers if there is an update available.
func CheckUpdate(httpClient *http.Client) (respData CheckUpdateResponse, err error) {
	var reqURL *url.URL
	reqURL, err = url.Parse(CheckUpdateURL)
	if err != nil {
		err = fmt.Errorf("failed to parse check update URL: %w", err)
		return
	}
	q := reqURL.Query()
	q.Set("version", store.GetWAVersion().String())
	q.Set("platform", "web")
	reqURL.RawQuery = q.Encode()
	var req *http.Request
	req, err = http.NewRequest(http.MethodGet, reqURL.String(), nil)
	if err != nil {
		err = fmt.Errorf("failed to prepare request: %w", err)
		return
	}
	req.Header.Set("Origin", socket.Origin)
	req.Header.Set("Referer", socket.Origin+"/")
	var resp *http.Response
	resp, err = httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to send request: %w", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		err = fmt.Errorf("unexpected response with status %d: %s", resp.StatusCode, body)
		return
	}
	err = json.NewDecoder(resp.Body).Decode(&respData)
	if err != nil {
		err = fmt.Errorf("failed to decode response body (status %d): %w", resp.StatusCode, err)
		return
	}
	respData.ParsedVersion, err = store.ParseVersion(respData.CurrentVersion)
	if err != nil {
		err = fmt.Errorf("failed to parse version string: %w", err)
	}
	return
}
