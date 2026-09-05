// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package store

import (
	"sync"
	"testing"
	"time"
)

func mustNotBlock(t *testing.T, name string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s deadlocked", name)
	}
}

func TestLockSessionsDeduplicatesAddresses(t *testing.T) {
	var device Device
	mustNotBlock(t, "LockSessions with duplicate addresses", func() {
		device.LockSessions([]string{"a.0", "b.0", "a.0"})()
	})
}

func TestLockSessionsEmpty(t *testing.T) {
	var device Device
	device.LockSessions(nil)()
}

func TestLockSessionsOverlappingSetsDontDeadlock(t *testing.T) {
	var device Device
	// Different input orders must still be acquired in the same global order.
	sets := [][]string{
		{"a.0", "b.0", "c.0"},
		{"c.0", "b.0", "a.0"},
		{"b.0", "d.0"},
		{"d.0", "a.0"},
	}
	mustNotBlock(t, "concurrent overlapping LockSessions", func() {
		var wg sync.WaitGroup
		for _, addresses := range sets {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 200 {
					unlock := device.LockSessions(addresses)
					unlock()
				}
			}()
		}
		wg.Wait()
	})
}

func TestLockSessionIsExclusive(t *testing.T) {
	var device Device
	counter := 0
	mustNotBlock(t, "concurrent LockSession", func() {
		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 100 {
					unlock := device.LockSession("a.0")
					counter++
					unlock()
				}
			}()
		}
		wg.Wait()
	})
	if counter != 800 {
		t.Errorf("expected 800 increments, got %d", counter)
	}
}

func TestLockSessionsIsExclusive(t *testing.T) {
	var device Device
	counters := map[string]int{"a.0": 0, "b.0": 0}
	mustNotBlock(t, "concurrent LockSessions", func() {
		var wg sync.WaitGroup
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 100 {
					unlock := device.LockSessions([]string{"b.0", "a.0"})
					counters["a.0"]++
					counters["b.0"]++
					unlock()
				}
			}()
		}
		wg.Wait()
	})
	for addr, count := range counters {
		if count != 800 {
			t.Errorf("expected 800 increments for %s, got %d", addr, count)
		}
	}
}
