// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"slices"
	"sync"
)

// Mutexes are never evicted: the map keeps one entry per distinct signal
// address the process has locked, for the lifetime of the process.
func (device *Device) sessionLock(address string) *sync.Mutex {
	val, _ := device.sessionLocks.LoadOrStore(address, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// LockSession acquires the lock for the session record of the given signal
// address and returns the function that releases it. Both encrypting and
// decrypting do a read-modify-write of the whole record, so they must hold
// this lock to avoid losing each other's ratchet advances.
func (device *Device) LockSession(address string) func() {
	lock := device.sessionLock(address)
	lock.Lock()
	return lock.Unlock
}

// LockSessions acquires the session locks for all given addresses,
// deduplicated and in sorted order to prevent deadlocks between concurrent
// multi-address lockers. The returned function releases all locks.
//
// Sorting only makes concurrent calls safe against each other, so no caller
// may hold a session lock while acquiring more: every path that locks several
// addresses (including the PN->LID session migration) must acquire them in one
// call and release before locking anything else.
func (device *Device) LockSessions(addresses []string) func() {
	if len(addresses) == 0 {
		return func() {}
	}
	sorted := slices.Clone(addresses)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)
	locks := make([]*sync.Mutex, len(sorted))
	for i, addr := range sorted {
		locks[i] = device.sessionLock(addr)
		locks[i].Lock()
	}
	return func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].Unlock()
		}
	}
}
