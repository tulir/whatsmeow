// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"fmt"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

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

func (cli *Client) refreshMediaConn(force bool) (*MediaConn, error) {
	cli.mediaConnLock.Lock()
	defer cli.mediaConnLock.Unlock()
	if cli.mediaConnCache == nil || force || time.Now().After(cli.mediaConnCache.Expiry()) {
		var err error
		cli.mediaConnCache, err = cli.queryMediaConn()
		if err != nil {
			return nil, err
		}
	}
	return cli.mediaConnCache, nil
}

func (cli *Client) queryMediaConn() (*MediaConn, error) {
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:m",
		Type:      "set",
		To:        types.ServerJID,
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
