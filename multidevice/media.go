// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
)

func (cli *Client) downloadMedia(directPath string, encFileHash, mediaKey []byte, fileLength int, mediaType whatsapp.MediaType, mmsType string) (data []byte, err error) {
	err = cli.refreshMediaConn(false)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh media connections: %w", err)
	}
	for i, host := range cli.mediaConn.Hosts {
		url := fmt.Sprintf("https://%s%s&hash=%s&mms-type=%s&__wa-mms=", host.Hostname, directPath, base64.URLEncoding.EncodeToString(encFileHash), mmsType)
		data, err = whatsapp.Download(url, mediaKey, mediaType, fileLength)
		if errors.Is(err, whatsapp.ErrInvalidMediaHMAC) {
			err = nil
		}
		if err != nil {
			if i >= len(cli.mediaConn.Hosts)-1 {
				return nil, fmt.Errorf("failed to download media from last host: %w", err)
			} else {
				cli.Log.Warnfln("Failed to download media: %s, trying with next host...", err)
			}
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

func (cli *Client) queryMediaConn() (*whatsapp.MediaConn, error) {
	resp, err := cli.sendIQ(InfoQuery{
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
	var mc whatsapp.MediaConn
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
			cli.Log.Warnln("Unexpected child in media_conn element:", child.XMLString())
			continue
		}
		cag := child.AttrGetter()
		mc.Hosts = append(mc.Hosts, whatsapp.MediaConnHost{
			Hostname: cag.String("hostname"),
		})
		if !cag.OK() {
			return nil, fmt.Errorf("failed to parse media connection host: %+v", ag.Errors)
		}
	}
	return &mc, nil
}
