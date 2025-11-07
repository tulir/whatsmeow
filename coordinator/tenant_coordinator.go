// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package coordinator provides multi-tenancy coordination for managing multiple WhatsApp accounts.
package coordinator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var (
	ErrTenantNotFound    = errors.New("tenant not found")
	ErrTenantNotRunning  = errors.New("tenant not running")
	ErrTenantAlreadyExists = errors.New("tenant already exists")
)

// TenantStatus represents the current state of a tenant.
type TenantStatus string

const (
	TenantStatusStopped      TenantStatus = "stopped"
	TenantStatusStarting     TenantStatus = "starting"
	TenantStatusConnected    TenantStatus = "connected"
	TenantStatusDisconnected TenantStatus = "disconnected"
	TenantStatusError        TenantStatus = "error"
)

// Tenant represents a managed WhatsApp client instance.
type Tenant struct {
	BusinessID string
	Container  *sqlstore.Container
	Client     *whatsmeow.Client
	Status     TenantStatus
	LastError  error
	StartedAt  time.Time
	UpdatedAt  time.Time
}

// EventHandler is called when events occur for any tenant.
type EventHandler func(businessID string, evt interface{})

// TenantCoordinator manages multiple WhatsApp client instances (tenants).
type TenantCoordinator struct {
	pool     *pgxpool.Pool
	log      waLog.Logger
	hostname string

	mu      sync.RWMutex
	tenants map[string]*Tenant

	eventHandlers     []EventHandler
	eventHandlersLock sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// CoordinatorConfig configures the tenant coordinator.
type CoordinatorConfig struct {
	Hostname         string        // Hostname of this instance
	HealthCheckInterval time.Duration // How often to check tenant health
}

// NewTenantCoordinator creates a new tenant coordinator.
func NewTenantCoordinator(pool *pgxpool.Pool, config CoordinatorConfig, log waLog.Logger) *TenantCoordinator {
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 10 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &TenantCoordinator{
		pool:          pool,
		log:           log,
		hostname:      config.Hostname,
		tenants:       make(map[string]*Tenant),
		eventHandlers: make([]EventHandler, 0),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// AddEventHandler registers a handler for events from all tenants.
func (tc *TenantCoordinator) AddEventHandler(handler EventHandler) {
	tc.eventHandlersLock.Lock()
	tc.eventHandlers = append(tc.eventHandlers, handler)
	tc.eventHandlersLock.Unlock()
}

// dispatchEvent sends an event to all registered handlers.
func (tc *TenantCoordinator) dispatchEvent(businessID string, evt interface{}) {
	tc.eventHandlersLock.RLock()
	handlers := make([]EventHandler, len(tc.eventHandlers))
	copy(handlers, tc.eventHandlers)
	tc.eventHandlersLock.RUnlock()

	for _, handler := range handlers {
		go handler(businessID, evt)
	}
}

// StartTenant starts a WhatsApp client for the specified business ID.
func (tc *TenantCoordinator) StartTenant(ctx context.Context, businessID string) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Check if already exists
	if tenant, exists := tc.tenants[businessID]; exists {
		if tenant.Client != nil && tenant.Client.IsConnected() {
			return ErrTenantAlreadyExists
		}
		// Tenant exists but not connected, clean up and restart
		tc.log.Infof("Restarting tenant %s", businessID)
		tc.stopTenantLocked(businessID)
	}

	// Create tenant
	tenant := &Tenant{
		BusinessID: businessID,
		Status:     TenantStatusStarting,
		StartedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	tc.tenants[businessID] = tenant

	// Create container and get device
	container := sqlstore.NewContainer(tc.pool, businessID, tc.log.Sub(businessID))
	tenant.Container = container

	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		tenant.Status = TenantStatusError
		tenant.LastError = fmt.Errorf("failed to get device: %w", err)
		return tenant.LastError
	}

	if device == nil {
		tenant.Status = TenantStatusError
		tenant.LastError = errors.New("no device found for tenant")
		return tenant.LastError
	}

	// Create and configure client
	client := whatsmeow.NewClient(device, tc.log.Sub(businessID))
	tenant.Client = client

	// Add event handler
	client.AddEventHandler(func(evt interface{}) {
		tc.handleTenantEvent(businessID, evt)
	})

	// Connect
	if err := client.Connect(); err != nil {
		tenant.Status = TenantStatusError
		tenant.LastError = fmt.Errorf("failed to connect: %w", err)
		return tenant.LastError
	}

	tenant.Status = TenantStatusConnected
	tenant.UpdatedAt = time.Now()
	tc.log.Infof("Started tenant %s", businessID)

	return nil
}

// StopTenant stops the WhatsApp client for the specified business ID.
func (tc *TenantCoordinator) StopTenant(businessID string) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	return tc.stopTenantLocked(businessID)
}

func (tc *TenantCoordinator) stopTenantLocked(businessID string) error {
	tenant, exists := tc.tenants[businessID]
	if !exists {
		return ErrTenantNotFound
	}

	if tenant.Client != nil {
		tenant.Client.Disconnect()
	}

	tenant.Status = TenantStatusStopped
	tenant.UpdatedAt = time.Now()
	delete(tc.tenants, businessID)

	tc.log.Infof("Stopped tenant %s", businessID)
	return nil
}

// GetTenant returns information about a tenant.
func (tc *TenantCoordinator) GetTenant(businessID string) (*Tenant, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	tenant, exists := tc.tenants[businessID]
	if !exists {
		return nil, ErrTenantNotFound
	}

	// Return a copy to prevent external modification
	tenantCopy := *tenant
	return &tenantCopy, nil
}

// ListTenants returns all currently managed tenants.
func (tc *TenantCoordinator) ListTenants() []*Tenant {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	tenants := make([]*Tenant, 0, len(tc.tenants))
	for _, tenant := range tc.tenants {
		tenantCopy := *tenant
		tenants = append(tenants, &tenantCopy)
	}

	return tenants
}

// SendMessage sends a message on behalf of a tenant.
func (tc *TenantCoordinator) SendMessage(businessID string, to string, message interface{}) error {
	tc.mu.RLock()
	tenant, exists := tc.tenants[businessID]
	tc.mu.RUnlock()

	if !exists {
		return ErrTenantNotFound
	}

	if tenant.Client == nil || !tenant.Client.IsConnected() {
		return ErrTenantNotRunning
	}

	// Note: Actual SendMessage implementation depends on message type
	// This is a placeholder - you'll need to implement proper message sending
	tc.log.Infof("SendMessage called for tenant %s to %s", businessID, to)
	return errors.New("not implemented - use tenant.Client.SendMessage directly")
}

// handleTenantEvent processes events from tenant clients.
func (tc *TenantCoordinator) handleTenantEvent(businessID string, evt interface{}) {
	// Update tenant status based on events
	tc.mu.Lock()
	tenant, exists := tc.tenants[businessID]
	if exists {
		switch evt.(type) {
		case *events.Connected:
			tenant.Status = TenantStatusConnected
			tenant.UpdatedAt = time.Now()
		case *events.Disconnected:
			tenant.Status = TenantStatusDisconnected
			tenant.UpdatedAt = time.Now()
		case *events.LoggedOut:
			tenant.Status = TenantStatusError
			tenant.LastError = errors.New("logged out")
			tenant.UpdatedAt = time.Now()
		case *events.StreamReplaced:
			tenant.Status = TenantStatusError
			tenant.LastError = errors.New("stream replaced")
			tenant.UpdatedAt = time.Now()
		}
	}
	tc.mu.Unlock()

	// Dispatch to external handlers
	tc.dispatchEvent(businessID, evt)
}

// StartHealthMonitoring begins periodic health checks of all tenants.
func (tc *TenantCoordinator) StartHealthMonitoring(interval time.Duration) {
	tc.wg.Add(1)
	go tc.healthMonitorLoop(interval)
}

func (tc *TenantCoordinator) healthMonitorLoop(interval time.Duration) {
	defer tc.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tc.performHealthCheck()
		case <-tc.ctx.Done():
			return
		}
	}
}

func (tc *TenantCoordinator) performHealthCheck() {
	tc.mu.RLock()
	tenantsToCheck := make([]*Tenant, 0, len(tc.tenants))
	for _, tenant := range tc.tenants {
		tenantCopy := *tenant
		tenantsToCheck = append(tenantsToCheck, &tenantCopy)
	}
	tc.mu.RUnlock()

	for _, tenant := range tenantsToCheck {
		if tenant.Client == nil {
			continue
		}

		isConnected := tenant.Client.IsConnected()

		tc.mu.Lock()
		currentTenant, exists := tc.tenants[tenant.BusinessID]
		if exists {
			if isConnected && currentTenant.Status != TenantStatusConnected {
				currentTenant.Status = TenantStatusConnected
				currentTenant.UpdatedAt = time.Now()
				tc.log.Infof("Tenant %s reconnected", tenant.BusinessID)
			} else if !isConnected && currentTenant.Status == TenantStatusConnected {
				currentTenant.Status = TenantStatusDisconnected
				currentTenant.UpdatedAt = time.Now()
				tc.log.Warnf("Tenant %s disconnected", tenant.BusinessID)

				// Attempt reconnection
				tc.mu.Unlock()
				go tc.reconnectTenant(tenant.BusinessID)
				tc.mu.Lock()
			}
		}
		tc.mu.Unlock()
	}
}

func (tc *TenantCoordinator) reconnectTenant(businessID string) {
	tc.log.Infof("Attempting to reconnect tenant %s", businessID)

	tc.mu.RLock()
	tenant, exists := tc.tenants[businessID]
	tc.mu.RUnlock()

	if !exists || tenant.Client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := tenant.Client.Connect()
	if err != nil {
		tc.log.Errorf("Failed to reconnect tenant %s: %v", businessID, err)

		tc.mu.Lock()
		if t, exists := tc.tenants[businessID]; exists {
			t.LastError = err
			t.UpdatedAt = time.Now()
		}
		tc.mu.Unlock()
	} else {
		tc.log.Infof("Successfully reconnected tenant %s", businessID)
	}
}

// StopAll stops all tenants and cleans up resources.
func (tc *TenantCoordinator) StopAll() {
	tc.cancel()
	tc.wg.Wait()

	tc.mu.Lock()
	defer tc.mu.Unlock()

	for businessID := range tc.tenants {
		tc.stopTenantLocked(businessID)
	}

	tc.log.Infof("Stopped all tenants")
}

// Close is an alias for StopAll.
func (tc *TenantCoordinator) Close() {
	tc.StopAll()
}
