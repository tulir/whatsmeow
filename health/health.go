// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package health provides health checking and monitoring utilities.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/ha"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// Status represents the health status of a component.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// ComponentHealth represents the health of a single component.
type ComponentHealth struct {
	Status    Status                 `json:"status"`
	Message   string                 `json:"message,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// HealthReport represents the overall health status.
type HealthReport struct {
	Status     Status                      `json:"status"`
	Timestamp  time.Time                   `json:"timestamp"`
	Components map[string]ComponentHealth  `json:"components"`
}

// Checker defines the interface for health checkers.
type Checker interface {
	Check(ctx context.Context) ComponentHealth
	Name() string
}

// HealthMonitor monitors the health of various components.
type HealthMonitor struct {
	mu       sync.RWMutex
	checkers map[string]Checker
	log      waLog.Logger
}

// NewHealthMonitor creates a new health monitor.
func NewHealthMonitor(log waLog.Logger) *HealthMonitor {
	return &HealthMonitor{
		checkers: make(map[string]Checker),
		log:      log,
	}
}

// AddChecker adds a health checker.
func (hm *HealthMonitor) AddChecker(checker Checker) {
	hm.mu.Lock()
	hm.checkers[checker.Name()] = checker
	hm.mu.Unlock()
}

// Check performs health checks on all registered checkers.
func (hm *HealthMonitor) Check(ctx context.Context) HealthReport {
	hm.mu.RLock()
	checkers := make(map[string]Checker, len(hm.checkers))
	for name, checker := range hm.checkers {
		checkers[name] = checker
	}
	hm.mu.RUnlock()

	components := make(map[string]ComponentHealth)
	overallStatus := StatusHealthy

	for name, checker := range checkers {
		health := checker.Check(ctx)
		components[name] = health

		// Determine overall status
		if health.Status == StatusUnhealthy {
			overallStatus = StatusUnhealthy
		} else if health.Status == StatusDegraded && overallStatus == StatusHealthy {
			overallStatus = StatusDegraded
		}
	}

	return HealthReport{
		Status:     overallStatus,
		Timestamp:  time.Now(),
		Components: components,
	}
}

// HTTPHandler returns an HTTP handler for health checks.
func (hm *HealthMonitor) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		report := hm.Check(ctx)

		w.Header().Set("Content-Type", "application/json")

		// Set HTTP status based on health
		switch report.Status {
		case StatusHealthy:
			w.WriteHeader(http.StatusOK)
		case StatusDegraded:
			w.WriteHeader(http.StatusOK) // Still accepting traffic
		case StatusUnhealthy:
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(report)
	}
}

// DatabaseChecker checks database connectivity.
type DatabaseChecker struct {
	pool *pgxpool.Pool
	name string
}

// NewDatabaseChecker creates a database health checker.
func NewDatabaseChecker(pool *pgxpool.Pool, name string) *DatabaseChecker {
	if name == "" {
		name = "database"
	}
	return &DatabaseChecker{
		pool: pool,
		name: name,
	}
}

func (dc *DatabaseChecker) Name() string {
	return dc.name
}

func (dc *DatabaseChecker) Check(ctx context.Context) ComponentHealth {
	start := time.Now()

	var result int
	err := dc.pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	latency := time.Since(start)

	if err != nil {
		return ComponentHealth{
			Status:    StatusUnhealthy,
			Message:   fmt.Sprintf("database query failed: %v", err),
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"error":   err.Error(),
				"latency": latency.String(),
			},
		}
	}

	status := StatusHealthy
	if latency > 100*time.Millisecond {
		status = StatusDegraded
	}

	return ComponentHealth{
		Status:    status,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"latency":  latency.String(),
			"acquired": dc.pool.Stat().AcquiredConns(),
			"idle":     dc.pool.Stat().IdleConns(),
			"max":      dc.pool.Stat().MaxConns(),
		},
	}
}

// ClientChecker checks WhatsApp client connectivity.
type ClientChecker struct {
	client *whatsmeow.Client
	name   string
}

// NewClientChecker creates a WhatsApp client health checker.
func NewClientChecker(client *whatsmeow.Client, name string) *ClientChecker {
	if name == "" {
		name = "whatsapp"
	}
	return &ClientChecker{
		client: client,
		name:   name,
	}
}

func (cc *ClientChecker) Name() string {
	return cc.name
}

func (cc *ClientChecker) Check(ctx context.Context) ComponentHealth {
	if cc.client == nil {
		return ComponentHealth{
			Status:    StatusUnhealthy,
			Message:   "client is nil",
			Timestamp: time.Now(),
		}
	}

	isConnected := cc.client.IsConnected()
	isLoggedIn := cc.client.IsLoggedIn()

	if !isConnected {
		return ComponentHealth{
			Status:    StatusUnhealthy,
			Message:   "not connected to WhatsApp",
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"connected": false,
				"logged_in": isLoggedIn,
			},
		}
	}

	if !isLoggedIn {
		return ComponentHealth{
			Status:    StatusDegraded,
			Message:   "connected but not logged in",
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"connected": true,
				"logged_in": false,
			},
		}
	}

	return ComponentHealth{
		Status:    StatusHealthy,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"connected": true,
			"logged_in": true,
		},
	}
}

// LeadershipChecker checks leadership status.
type LeadershipChecker struct {
	election *ha.LeaderElection
	name     string
}

// NewLeadershipChecker creates a leadership health checker.
func NewLeadershipChecker(election *ha.LeaderElection, name string) *LeadershipChecker {
	if name == "" {
		name = "leadership"
	}
	return &LeadershipChecker{
		election: election,
		name:     name,
	}
}

func (lc *LeadershipChecker) Name() string {
	return lc.name
}

func (lc *LeadershipChecker) Check(ctx context.Context) ComponentHealth {
	if lc.election == nil {
		return ComponentHealth{
			Status:    StatusHealthy,
			Message:   "leadership not enabled",
			Timestamp: time.Now(),
		}
	}

	isLeader, err := lc.election.VerifyLeadership(ctx)
	if err != nil {
		return ComponentHealth{
			Status:    StatusDegraded,
			Message:   fmt.Sprintf("failed to verify leadership: %v", err),
			Timestamp: time.Now(),
			Details: map[string]interface{}{
				"error": err.Error(),
			},
		}
	}

	return ComponentHealth{
		Status:    StatusHealthy,
		Timestamp: time.Now(),
		Details: map[string]interface{}{
			"is_leader": isLeader,
		},
	}
}

// ReadinessChecker checks if the application is ready to serve traffic.
type ReadinessChecker struct {
	checks []func() bool
	name   string
}

// NewReadinessChecker creates a readiness checker.
func NewReadinessChecker(name string) *ReadinessChecker {
	if name == "" {
		name = "readiness"
	}
	return &ReadinessChecker{
		checks: make([]func() bool, 0),
		name:   name,
	}
}

// AddCheck adds a readiness check function.
func (rc *ReadinessChecker) AddCheck(check func() bool) {
	rc.checks = append(rc.checks, check)
}

func (rc *ReadinessChecker) Name() string {
	return rc.name
}

func (rc *ReadinessChecker) Check(ctx context.Context) ComponentHealth {
	for i, check := range rc.checks {
		if !check() {
			return ComponentHealth{
				Status:    StatusUnhealthy,
				Message:   fmt.Sprintf("readiness check %d failed", i),
				Timestamp: time.Now(),
			}
		}
	}

	return ComponentHealth{
		Status:    StatusHealthy,
		Timestamp: time.Now(),
	}
}

// LivenessChecker is a simple health check that always returns healthy.
// Useful for Kubernetes liveness probes.
type LivenessChecker struct {
	name string
}

// NewLivenessChecker creates a liveness checker.
func NewLivenessChecker(name string) *LivenessChecker {
	if name == "" {
		name = "liveness"
	}
	return &LivenessChecker{name: name}
}

func (lc *LivenessChecker) Name() string {
	return lc.name
}

func (lc *LivenessChecker) Check(ctx context.Context) ComponentHealth {
	return ComponentHealth{
		Status:    StatusHealthy,
		Timestamp: time.Now(),
	}
}

// Helper to add missing import
import "fmt"
