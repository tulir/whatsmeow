// Copyright (c) 2022 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"

	"go.mau.fi/libsignal/keys/prekey"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

type DangerousInternalClient struct {
	c *Client
}

// DangerousInternals allows access to some unexported methods in Client.
//
// Deprecated: dangerous
func (cli *Client) DangerousInternals() *DangerousInternalClient {
	return &DangerousInternalClient{cli}
}

type DangerousInfoQuery = infoQuery
type DangerousInfoQueryType = infoQueryType

func (int *DangerousInternalClient) SendIQ(query DangerousInfoQuery) (*waBinary.Node, error) {
	return int.c.sendIQ(query)
}

func (int *DangerousInternalClient) SendIQAsync(query DangerousInfoQuery) (<-chan *waBinary.Node, error) {
	return int.c.sendIQAsync(query)
}

func (int *DangerousInternalClient) SendNode(node waBinary.Node) error {
	return int.c.sendNode(node)
}

func (int *DangerousInternalClient) WaitResponse(reqID string) chan *waBinary.Node {
	return int.c.waitResponse(reqID)
}

func (int *DangerousInternalClient) CancelResponse(reqID string, ch chan *waBinary.Node) {
	int.c.cancelResponse(reqID, ch)
}

func (int *DangerousInternalClient) QueryMediaConn() (*MediaConn, error) {
	return int.c.queryMediaConn()
}

func (int *DangerousInternalClient) RefreshMediaConn(force bool) (*MediaConn, error) {
	return int.c.refreshMediaConn(force)
}

func (int *DangerousInternalClient) GetServerPreKeyCount() (int, error) {
	return int.c.getServerPreKeyCount()
}

func (int *DangerousInternalClient) RequestAppStateKeys(ctx context.Context, keyIDs [][]byte) {
	int.c.requestAppStateKeys(ctx, keyIDs)
}

func (int *DangerousInternalClient) SendRetryReceipt(node *waBinary.Node, info *types.MessageInfo, forceIncludeIdentity bool) {
	int.c.sendRetryReceipt(node, info, forceIncludeIdentity)
}

func (int *DangerousInternalClient) EncryptMessageForDevice(plaintext []byte, to types.JID, bundle *prekey.Bundle, extraAttrs waBinary.Attrs) (*waBinary.Node, bool, error) {
	return int.c.encryptMessageForDevice(plaintext, to, bundle, extraAttrs)
}

func (int *DangerousInternalClient) GetOwnID() types.JID {
	return int.c.getOwnID()
}

func (int *DangerousInternalClient) DecryptDM(child *waBinary.Node, from types.JID, isPreKey bool) ([]byte, error) {
	return int.c.decryptDM(child, from, isPreKey)
}

func (int *DangerousInternalClient) MakeDeviceIdentityNode() waBinary.Node {
	return int.c.makeDeviceIdentityNode()
}
