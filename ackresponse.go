// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import waBinary "go.mau.fi/whatsmeow/binary"

type ackResponseKey struct {
	id, class, receiptType string
}

// WaitResponseForACK registers before sending a stanza whose ID can also appear
// in unrelated ACKs. Only an ACK with the requested ID, class and type consumes
// this waiter; ordinary ID-only waiters continue to receive other responses.
// Calls for the same key must be serialized. The caller must invoke cancel when
// finished; it removes only this registration and never closes a channel that a
// concurrent response dispatcher may already have detached.
func (cli *Client) WaitResponseForACK(id, class, receiptType string) (<-chan *waBinary.Node, func()) {
	key := ackResponseKey{id, class, receiptType}
	ch := make(chan *waBinary.Node, 1)
	cli.responseWaitersLock.Lock()
	if cli.ackResponseWaiters == nil {
		cli.ackResponseWaiters = make(map[ackResponseKey]chan *waBinary.Node)
	}
	cli.ackResponseWaiters[key] = ch
	cli.responseWaitersLock.Unlock()
	return ch, func() {
		cli.responseWaitersLock.Lock()
		if cli.ackResponseWaiters[key] == ch {
			delete(cli.ackResponseWaiters, key)
		}
		cli.responseWaitersLock.Unlock()
	}
}
