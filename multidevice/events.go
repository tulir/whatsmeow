// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	waBinary "go.mau.fi/whatsmeow/binary"
)

type nodeHandler func(cli *Client, node *waBinary.Node) bool

var nodeHandlers = [...]nodeHandler{
	handlePairDevice,
}
