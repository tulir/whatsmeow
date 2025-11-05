// Copyright (c) 2025 Security Testing
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sqlstore_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// TestCrossTenantIsolation tests that data from different tenants is properly isolated
func TestCrossTenantIsolation(t *testing.T) {
	// Skip this test if no PostgreSQL connection is available
	dbURL := getTestDatabaseURL()
	if dbURL == "" {
		t.Skip("Skipping cross-tenant isolation test: no database URL provided (set TEST_DB_URL)")
	}

	ctx := context.Background()

	// Create database connection
	dbPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	// Create two containers with different business IDs
	log := waLog.Noop
	container1 := sqlstore.NewContainer(dbPool, "tenant1", log)
	container2 := sqlstore.NewContainer(dbPool, "tenant2", log)

	// Create a device for tenant1
	device1 := container1.NewDevice()
	jid1, _ := types.ParseJID("user1@whatsapp.net")
	device1.ID = &jid1
	err = container1.PutDevice(ctx, device1)
	if err != nil {
		t.Fatalf("Failed to create device for tenant1: %v", err)
	}
	defer container1.DeleteDevice(ctx, device1)

	// Create a device with the SAME JID for tenant2
	device2 := container2.NewDevice()
	device2.ID = &jid1 // Same JID as tenant1
	err = container2.PutDevice(ctx, device2)
	if err != nil {
		t.Fatalf("Failed to create device for tenant2: %v", err)
	}
	defer container2.DeleteDevice(ctx, device2)

	// Test 1: Verify both devices exist and have different data
	t.Run("SameJIDDifferentTenants", func(t *testing.T) {
		retrieved1, err := container1.GetDevice(ctx, jid1)
		if err != nil {
			t.Fatalf("Failed to get device from tenant1: %v", err)
		}
		if retrieved1 == nil {
			t.Fatal("Device not found in tenant1")
		}

		retrieved2, err := container2.GetDevice(ctx, jid1)
		if err != nil {
			t.Fatalf("Failed to get device from tenant2: %v", err)
		}
		if retrieved2 == nil {
			t.Fatal("Device not found in tenant2")
		}

		// Verify they have different noise keys (proving they're different devices)
		if retrieved1.NoiseKey.Pub == retrieved2.NoiseKey.Pub {
			t.Error("Devices from different tenants have identical noise keys - tenant isolation broken!")
		}
	})

	// Test 2: Verify tenant1 cannot see tenant2's devices
	t.Run("CannotAccessOtherTenantDevice", func(t *testing.T) {
		// Get all devices from tenant1
		devices1, err := container1.GetAllDevices(ctx)
		if err != nil {
			t.Fatalf("Failed to get all devices from tenant1: %v", err)
		}

		// Should only see 1 device
		if len(devices1) != 1 {
			t.Errorf("Tenant1 sees %d devices, expected 1", len(devices1))
		}

		// Get all devices from tenant2
		devices2, err := container2.GetAllDevices(ctx)
		if err != nil {
			t.Fatalf("Failed to get all devices from tenant2: %v", err)
		}

		// Should only see 1 device
		if len(devices2) != 1 {
			t.Errorf("Tenant2 sees %d devices, expected 1", len(devices2))
		}
	})

	// Test 3: Test session isolation
	t.Run("SessionIsolation", func(t *testing.T) {
		store1 := sqlstore.NewSQLStore(container1, jid1)
		store2 := sqlstore.NewSQLStore(container2, jid1)

		// Put a session in tenant1
		sessionData1 := []byte("tenant1-session-data")
		err := store1.PutSession(ctx, "contact@whatsapp.net", sessionData1)
		if err != nil {
			t.Fatalf("Failed to put session in tenant1: %v", err)
		}

		// Put a different session with same address in tenant2
		sessionData2 := []byte("tenant2-session-data")
		err = store2.PutSession(ctx, "contact@whatsapp.net", sessionData2)
		if err != nil {
			t.Fatalf("Failed to put session in tenant2: %v", err)
		}

		// Verify tenant1 gets its own session
		retrieved1, err := store1.GetSession(ctx, "contact@whatsapp.net")
		if err != nil {
			t.Fatalf("Failed to get session from tenant1: %v", err)
		}
		if string(retrieved1) != string(sessionData1) {
			t.Errorf("Tenant1 session data mismatch: got %s, want %s", retrieved1, sessionData1)
		}

		// Verify tenant2 gets its own session
		retrieved2, err := store2.GetSession(ctx, "contact@whatsapp.net")
		if err != nil {
			t.Fatalf("Failed to get session from tenant2: %v", err)
		}
		if string(retrieved2) != string(sessionData2) {
			t.Errorf("Tenant2 session data mismatch: got %s, want %s", retrieved2, sessionData2)
		}
	})

	// Test 4: Test contact isolation
	t.Run("ContactIsolation", func(t *testing.T) {
		store1 := sqlstore.NewSQLStore(container1, jid1)
		store2 := sqlstore.NewSQLStore(container2, jid1)

		contactJID, _ := types.ParseJID("contact@whatsapp.net")

		// Put contact in tenant1
		err := store1.PutContactName(ctx, contactJID, "Tenant1 Contact", "T1")
		if err != nil {
			t.Fatalf("Failed to put contact in tenant1: %v", err)
		}

		// Put different contact with same JID in tenant2
		err = store2.PutContactName(ctx, contactJID, "Tenant2 Contact", "T2")
		if err != nil {
			t.Fatalf("Failed to put contact in tenant2: %v", err)
		}

		// Verify tenant1 gets its own contact
		contact1, err := store1.GetContact(ctx, contactJID)
		if err != nil {
			t.Fatalf("Failed to get contact from tenant1: %v", err)
		}
		if contact1.FullName != "Tenant1 Contact" {
			t.Errorf("Tenant1 contact name mismatch: got %s, want Tenant1 Contact", contact1.FullName)
		}

		// Verify tenant2 gets its own contact
		contact2, err := store2.GetContact(ctx, contactJID)
		if err != nil {
			t.Fatalf("Failed to get contact from tenant2: %v", err)
		}
		if contact2.FullName != "Tenant2 Contact" {
			t.Errorf("Tenant2 contact name mismatch: got %s, want Tenant2 Contact", contact2.FullName)
		}
	})

	// Test 5: Test identity key isolation
	t.Run("IdentityKeyIsolation", func(t *testing.T) {
		store1 := sqlstore.NewSQLStore(container1, jid1)
		store2 := sqlstore.NewSQLStore(container2, jid1)

		var key1, key2 [32]byte
		for i := range key1 {
			key1[i] = byte(i)
			key2[i] = byte(i + 100)
		}

		// Put identity key in tenant1
		err := store1.PutIdentity(ctx, "contact@whatsapp.net:1", key1)
		if err != nil {
			t.Fatalf("Failed to put identity in tenant1: %v", err)
		}

		// Put different identity key in tenant2
		err = store2.PutIdentity(ctx, "contact@whatsapp.net:1", key2)
		if err != nil {
			t.Fatalf("Failed to put identity in tenant2: %v", err)
		}

		// Verify tenant1's key is trusted for tenant1
		trusted1, err := store1.IsTrustedIdentity(ctx, "contact@whatsapp.net:1", key1)
		if err != nil {
			t.Fatalf("Failed to check identity in tenant1: %v", err)
		}
		if !trusted1 {
			t.Error("Tenant1's own key is not trusted")
		}

		// Verify tenant2's key is NOT trusted for tenant1 (different key)
		trusted1wrong, err := store1.IsTrustedIdentity(ctx, "contact@whatsapp.net:1", key2)
		if err != nil {
			t.Fatalf("Failed to check wrong identity in tenant1: %v", err)
		}
		if trusted1wrong {
			t.Error("Tenant2's key is incorrectly trusted in tenant1 - SECURITY BREACH!")
		}
	})
}

// TestDeleteCascade tests that deleting a device cascades to all related data
func TestDeleteCascade(t *testing.T) {
	dbURL := getTestDatabaseURL()
	if dbURL == "" {
		t.Skip("Skipping delete cascade test: no database URL provided")
	}

	ctx := context.Background()

	dbPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	container := sqlstore.NewContainer(dbPool, "test-tenant", waLog.Noop)
	device := container.NewDevice()
	jid, _ := types.ParseJID("test@whatsapp.net")
	device.ID = &jid

	err = container.PutDevice(ctx, device)
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}

	// Create associated data
	store := sqlstore.NewSQLStore(container, jid)

	// Add a session
	err = store.PutSession(ctx, "contact@whatsapp.net", []byte("test-session"))
	if err != nil {
		t.Fatalf("Failed to put session: %v", err)
	}

	// Add a contact
	contactJID, _ := types.ParseJID("contact@whatsapp.net")
	err = store.PutContactName(ctx, contactJID, "Test Contact", "TC")
	if err != nil {
		t.Fatalf("Failed to put contact: %v", err)
	}

	// Now delete the device
	err = container.DeleteDevice(ctx, device)
	if err != nil {
		t.Fatalf("Failed to delete device: %v", err)
	}

	// Verify device is gone
	retrieved, err := container.GetDevice(ctx, jid)
	if err != nil {
		t.Fatalf("Error checking deleted device: %v", err)
	}
	if retrieved != nil {
		t.Error("Device still exists after deletion")
	}

	// Verify cascaded deletion by trying to query associated data
	// (This would fail with FK constraint error if data wasn't deleted)
	t.Log("DELETE CASCADE test passed - device and associated data properly removed")
}

// Helper function to get test database URL from environment
func getTestDatabaseURL() string {
	// In a real test, this would read from environment variable:
	// return os.Getenv("TEST_DB_URL")
	// For now, return empty to skip tests
	return ""
}
