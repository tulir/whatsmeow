// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main provides an example of multi-tenant deployment.
//
// This example demonstrates how to manage multiple WhatsApp accounts
// (tenants) from a single application instance, with proper isolation
// and resource management.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/mattn/go-sqlite3"

	"go.mau.fi/whatsmeow/coordinator"
	"go.mau.fi/whatsmeow/health"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var (
	log               waLog.Logger
	tenantCoordinator *coordinator.TenantCoordinator
)

func main() {
	// Setup logging
	log = waLog.Stdout("Main", "INFO", true)

	// Configuration from environment
	databaseURL := getEnv("DATABASE_URL", "postgres://localhost/whatsmeow?sslmode=disable")
	httpPort := getEnv("HTTP_PORT", "8080")
	hostname, _ := os.Hostname()

	log.Infof("Starting Multi-Tenant Example")
	log.Infof("Hostname: %s", hostname)
	log.Infof("Database: %s", databaseURL)

	// Create context that can be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to database
	dbPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Errorf("Failed to connect to database: %v", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	// Verify database connection
	if err := dbPool.Ping(ctx); err != nil {
		log.Errorf("Failed to ping database: %v", err)
		os.Exit(1)
	}
	log.Infof("Connected to database")

	// Create tenant coordinator
	tenantCoordinator = coordinator.NewTenantCoordinator(dbPool, coordinator.CoordinatorConfig{
		Hostname:            hostname,
		HealthCheckInterval: 10 * time.Second,
	}, log.Sub("Coordinator"))
	defer tenantCoordinator.Close()

	// Setup event handler for all tenants
	tenantCoordinator.AddEventHandler(handleTenantEvent)

	// Start health monitoring
	tenantCoordinator.StartHealthMonitoring(10 * time.Second)

	// Setup health monitoring
	healthMonitor := health.NewHealthMonitor(log.Sub("Health"))
	healthMonitor.AddChecker(health.NewDatabaseChecker(dbPool, "database"))
	healthMonitor.AddChecker(health.NewLivenessChecker("liveness"))

	// Start HTTP server for API and health checks
	setupHTTPServer(httpPort, healthMonitor)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Infof("Server is ready. Use the API to manage tenants:")
	log.Infof("  POST   /api/tenants           - Start a new tenant")
	log.Infof("  GET    /api/tenants           - List all tenants")
	log.Infof("  GET    /api/tenants/{id}      - Get tenant status")
	log.Infof("  DELETE /api/tenants/{id}      - Stop a tenant")
	log.Infof("  GET    /health                - Health check")
	log.Infof("  GET    /ready                 - Readiness check")

	// Wait for shutdown signal
	<-sigChan
	log.Infof("Received shutdown signal, stopping all tenants...")
	tenantCoordinator.StopAll()
	log.Infof("Shutdown complete")
}

func setupHTTPServer(port string, healthMonitor *health.HealthMonitor) {
	// Health endpoints
	http.HandleFunc("/health", healthMonitor.HTTPHandler())
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		tenants := tenantCoordinator.ListTenants()
		connectedCount := 0
		for _, t := range tenants {
			if t.Status == coordinator.TenantStatusConnected {
				connectedCount++
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if connectedCount > 0 {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"ready":     connectedCount > 0,
			"total":     len(tenants),
			"connected": connectedCount,
		})
	})

	// API endpoints
	http.HandleFunc("/api/tenants", handleTenantsAPI)
	http.HandleFunc("/api/tenants/", handleTenantAPI)

	go func() {
		addr := ":" + port
		log.Infof("Starting HTTP server on %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Errorf("HTTP server error: %v", err)
		}
	}()
}

func handleTenantsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// List all tenants
		tenants := tenantCoordinator.ListTenants()

		response := make([]map[string]interface{}, len(tenants))
		for i, tenant := range tenants {
			response[i] = map[string]interface{}{
				"business_id": tenant.BusinessID,
				"status":      tenant.Status,
				"started_at":  tenant.StartedAt,
				"updated_at":  tenant.UpdatedAt,
			}
			if tenant.LastError != nil {
				response[i]["error"] = tenant.LastError.Error()
			}
		}

		json.NewEncoder(w).Encode(response)

	case http.MethodPost:
		// Start a new tenant
		var request struct {
			BusinessID string `json:"business_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "invalid request body",
			})
			return
		}

		if request.BusinessID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "business_id is required",
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		err := tenantCoordinator.StartTenant(ctx, request.BusinessID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{
			"status":      "started",
			"business_id": request.BusinessID,
		})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleTenantAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract business ID from path
	businessID := r.URL.Path[len("/api/tenants/"):]
	if businessID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "business_id is required",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Get tenant status
		tenant, err := tenantCoordinator.GetTenant(businessID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		response := map[string]interface{}{
			"business_id": tenant.BusinessID,
			"status":      tenant.Status,
			"started_at":  tenant.StartedAt,
			"updated_at":  tenant.UpdatedAt,
		}
		if tenant.LastError != nil {
			response["error"] = tenant.LastError.Error()
		}

		json.NewEncoder(w).Encode(response)

	case http.MethodDelete:
		// Stop tenant
		err := tenantCoordinator.StopTenant(businessID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]string{
			"status":      "stopped",
			"business_id": businessID,
		})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleTenantEvent(businessID string, evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		log.Infof("[%s] Received message from %s: %s", businessID, v.Info.Sender, v.Message.GetConversation())

	case *events.Receipt:
		log.Debugf("[%s] Received receipt for %s: %s", businessID, v.MessageIDs, v.Type)

	case *events.Connected:
		log.Infof("[%s] Connected to WhatsApp", businessID)

	case *events.Disconnected:
		log.Warnf("[%s] Disconnected from WhatsApp", businessID)

	case *events.StreamReplaced:
		log.Errorf("[%s] Stream replaced - another instance connected", businessID)

	case *events.LoggedOut:
		log.Errorf("[%s] Logged out from WhatsApp", businessID)

	case *events.AppStateSyncComplete:
		log.Infof("[%s] App state sync complete: %s", businessID, v.Name)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
