// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package ha provides high availability primitives for multi-host deployments.
package ha

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// LeaderElection implements leader election using PostgreSQL advisory locks.
// Only one instance across all hosts can be the leader at any time.
type LeaderElection struct {
	pool   *pgxpool.Pool
	lockID int64
	log    waLog.Logger

	mu       sync.RWMutex
	isLeader bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewLeaderElection creates a new leader election instance.
// The lockID should be unique per device/tenant to prevent conflicts.
// Use GenerateLockID to create a consistent lock ID from a string identifier.
func NewLeaderElection(pool *pgxpool.Pool, lockID int64, log waLog.Logger) *LeaderElection {
	ctx, cancel := context.WithCancel(context.Background())
	return &LeaderElection{
		pool:     pool,
		lockID:   lockID,
		log:      log,
		ctx:      ctx,
		cancel:   cancel,
		isLeader: false,
	}
}

// GenerateLockID generates a consistent lock ID from a string identifier.
// The same identifier will always produce the same lock ID.
func GenerateLockID(identifier string) int64 {
	hash := sha256.Sum256([]byte(identifier))
	// Use first 8 bytes as int64, ensure it's positive
	lockID := int64(binary.BigEndian.Uint64(hash[:8]))
	if lockID < 0 {
		lockID = -lockID
	}
	return lockID
}

// TryAcquire attempts to acquire leadership without blocking.
// Returns true if leadership was acquired, false otherwise.
func (le *LeaderElection) TryAcquire(ctx context.Context) (bool, error) {
	var acquired bool
	err := le.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", le.lockID).Scan(&acquired)
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	le.mu.Lock()
	le.isLeader = acquired
	le.mu.Unlock()

	if acquired {
		le.log.Infof("Acquired leadership (lock ID: %d)", le.lockID)
	}

	return acquired, nil
}

// Acquire attempts to acquire leadership, blocking until successful or context is cancelled.
// Uses exponential backoff with jitter for retries.
func (le *LeaderElection) Acquire(ctx context.Context) error {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		acquired, err := le.TryAcquire(ctx)
		if err != nil {
			return err
		}

		if acquired {
			return nil
		}

		// Exponential backoff with jitter
		select {
		case <-time.After(backoff):
			backoff = min(backoff*2, maxBackoff)
		case <-ctx.Done():
			return ctx.Err()
		case <-le.ctx.Done():
			return le.ctx.Err()
		}
	}
}

// IsLeader returns true if this instance currently holds leadership.
func (le *LeaderElection) IsLeader() bool {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.isLeader
}

// VerifyLeadership verifies that this instance still holds the advisory lock.
// This is more expensive than IsLeader() as it queries the database.
func (le *LeaderElection) VerifyLeadership(ctx context.Context) (bool, error) {
	var isLocked bool
	err := le.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM pg_locks
			WHERE locktype='advisory'
			AND objid=$1
			AND pid=pg_backend_pid()
		)
	`, le.lockID).Scan(&isLocked)

	if err != nil {
		return false, fmt.Errorf("failed to verify leadership: %w", err)
	}

	le.mu.Lock()
	le.isLeader = isLocked
	le.mu.Unlock()

	return isLocked, nil
}

// Release releases leadership and the advisory lock.
func (le *LeaderElection) Release(ctx context.Context) error {
	var released bool
	err := le.pool.QueryRow(ctx, "SELECT pg_advisory_unlock($1)", le.lockID).Scan(&released)
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	le.mu.Lock()
	le.isLeader = false
	le.mu.Unlock()

	if !released {
		le.log.Warnf("Failed to release lock (lock ID: %d), may not have been held", le.lockID)
		return errors.New("lock was not held")
	}

	le.log.Infof("Released leadership (lock ID: %d)", le.lockID)
	return nil
}

// Close releases leadership and cleans up resources.
func (le *LeaderElection) Close() error {
	le.cancel()

	if le.IsLeader() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return le.Release(ctx)
	}

	return nil
}

// LeadershipMonitor monitors leadership status and calls callbacks on state changes.
type LeadershipMonitor struct {
	election       *LeaderElection
	checkInterval  time.Duration
	onBecomeLeader func()
	onLoseLeader   func()
	log            waLog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// MonitorConfig configures the leadership monitor.
type MonitorConfig struct {
	CheckInterval  time.Duration
	OnBecomeLeader func()
	OnLoseLeader   func()
}

// NewLeadershipMonitor creates a monitor that continuously checks leadership status.
func NewLeadershipMonitor(election *LeaderElection, config MonitorConfig, log waLog.Logger) *LeadershipMonitor {
	if config.CheckInterval == 0 {
		config.CheckInterval = 5 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &LeadershipMonitor{
		election:       election,
		checkInterval:  config.CheckInterval,
		onBecomeLeader: config.OnBecomeLeader,
		onLoseLeader:   config.OnLoseLeader,
		log:            log,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start begins monitoring leadership status.
func (lm *LeadershipMonitor) Start() {
	lm.wg.Add(1)
	go lm.monitorLoop()
}

func (lm *LeadershipMonitor) monitorLoop() {
	defer lm.wg.Done()

	ticker := time.NewTicker(lm.checkInterval)
	defer ticker.Stop()

	wasLeader := lm.election.IsLeader()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(lm.ctx, 5*time.Second)
			isLeader, err := lm.election.VerifyLeadership(ctx)
			cancel()

			if err != nil {
				lm.log.Errorf("Failed to verify leadership: %v", err)
				continue
			}

			// Detect leadership changes
			if isLeader && !wasLeader {
				lm.log.Infof("Became leader")
				if lm.onBecomeLeader != nil {
					go lm.onBecomeLeader()
				}
			} else if !isLeader && wasLeader {
				lm.log.Warnf("Lost leadership")
				if lm.onLoseLeader != nil {
					go lm.onLoseLeader()
				}
			}

			wasLeader = isLeader

		case <-lm.ctx.Done():
			return
		}
	}
}

// Stop stops monitoring and waits for cleanup.
func (lm *LeadershipMonitor) Stop() {
	lm.cancel()
	lm.wg.Wait()
}

// Helper function for min calculation
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
