// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package main provides an example of active-passive failover deployment.
//
// This example demonstrates how to deploy multiple instances of whatsmeow
// where only one instance is active (connected) at a time, providing high
// availability through automatic failover.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/mattn/go-sqlite3"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/ha"
	"go.mau.fi/whatsmeow/health"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

var (
	log waLog.Logger
)

func main() {
	// Setup logging
	log = waLog.Stdout("Main", "INFO", true)

	// Configuration from environment
	databaseURL := getEnv("DATABASE_URL", "postgres://localhost/whatsmeow?sslmode=disable")
	businessID := getEnv("BUSINESS_ID", "default")
	httpPort := getEnv("HTTP_PORT", "8080")
	hostname, _ := os.Hostname()

	log.Infof("Starting HA Failover Example")
	log.Infof("Hostname: %s", hostname)
	log.Infof("Business ID: %s", businessID)
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

	// Create store container
	container := sqlstore.NewContainer(dbPool, businessID, log.Sub("Store"))

	// Get or create device
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		log.Errorf("Failed to get device: %v", err)
		os.Exit(1)
	}

	if device == nil {
		log.Infof("No device found. Please pair a new device first.")
		log.Infof("Run the mdtest example to create and link a device.")
		os.Exit(1)
	}

	// Setup leader election
	lockID := ha.GenerateLockID(businessID + ":" + device.ID.String())
	election := ha.NewLeaderElection(dbPool, lockID, log.Sub("Election"))
	defer election.Close()

	// Setup health monitoring
	healthMonitor := health.NewHealthMonitor(log.Sub("Health"))
	healthMonitor.AddChecker(health.NewDatabaseChecker(dbPool, "database"))
	healthMonitor.AddChecker(health.NewLeadershipChecker(election, "leadership"))
	healthMonitor.AddChecker(health.NewLivenessChecker("liveness"))

	// Start HTTP server for health checks
	http.HandleFunc("/health", healthMonitor.HTTPHandler())
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if election.IsLeader() {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Ready - Leader")
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Not Ready - Standby")
		}
	})

	go func() {
		addr := ":" + httpPort
		log.Infof("Starting HTTP server on %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Errorf("HTTP server error: %v", err)
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Main loop: try to become leader and maintain connection
	var client *whatsmeow.Client
	var monitor *ha.LeadershipMonitor

	for {
		select {
		case <-sigChan:
			log.Infof("Received shutdown signal")
			if client != nil {
				client.Disconnect()
			}
			if monitor != nil {
				monitor.Stop()
			}
			election.Release(ctx)
			return

		default:
			// Try to acquire leadership
			log.Infof("Attempting to acquire leadership...")
			err := election.Acquire(ctx)
			if err != nil {
				if err == context.Canceled {
					return
				}
				log.Errorf("Failed to acquire leadership: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			log.Infof("Acquired leadership, connecting to WhatsApp...")

			// Create client
			client = whatsmeow.NewClient(device, log.Sub("Client"))

			// Add client health checker
			healthMonitor.AddChecker(health.NewClientChecker(client, "whatsapp"))

			// Setup event handlers
			client.AddEventHandler(func(evt interface{}) {
				handleEvent(evt)
			})

			// Connect
			err = client.Connect()
			if err != nil {
				log.Errorf("Failed to connect: %v", err)
				election.Release(ctx)
				time.Sleep(5 * time.Second)
				continue
			}

			log.Infof("Connected to WhatsApp as leader")

			// Start leadership monitoring
			monitor = ha.NewLeadershipMonitor(election, ha.MonitorConfig{
				CheckInterval: 5 * time.Second,
				OnLoseLeader: func() {
					log.Warnf("Lost leadership, disconnecting...")
					if client != nil {
						client.Disconnect()
					}
				},
			}, log.Sub("Monitor"))
			monitor.Start()

			// Wait for disconnection or loss of leadership
			for {
				if !election.IsLeader() || !client.IsConnected() {
					log.Infof("Connection lost or leadership lost")
					break
				}
				time.Sleep(1 * time.Second)

				// Check for shutdown signal
				select {
				case <-sigChan:
					log.Infof("Received shutdown signal")
					monitor.Stop()
					client.Disconnect()
					election.Release(ctx)
					return
				default:
				}
			}

			// Cleanup
			monitor.Stop()
			if client.IsConnected() {
				client.Disconnect()
			}
			election.Release(ctx)

			log.Infof("Entering standby mode, will retry for leadership...")
			time.Sleep(2 * time.Second)
		}
	}
}

func handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		log.Infof("Received message from %s: %s", v.Info.Sender, v.Message.GetConversation())

	case *events.Receipt:
		log.Debugf("Received receipt for %s: %s", v.MessageIDs, v.Type)

	case *events.Connected:
		log.Infof("Connected to WhatsApp")

	case *events.Disconnected:
		log.Warnf("Disconnected from WhatsApp")

	case *events.StreamReplaced:
		log.Errorf("Stream replaced - another instance connected with same credentials")

	case *events.LoggedOut:
		log.Errorf("Logged out from WhatsApp")

	case *events.AppStateSyncComplete:
		log.Infof("App state sync complete: %s", v.Name)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
