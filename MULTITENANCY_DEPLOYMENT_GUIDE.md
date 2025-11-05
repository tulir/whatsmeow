# Multitenancy Deployment Guide

This guide provides comprehensive instructions for deploying and managing the businessId-based multitenancy feature in your WhatsApp library fork.

## Table of Contents

1. [Overview](#overview)
2. [Prerequisites](#prerequisites)
3. [Initial Setup](#initial-setup)
4. [Database Migration](#database-migration)
5. [Application Integration](#application-integration)
6. [Security Hardening (Optional)](#security-hardening-optional)
7. [Testing](#testing)
8. [Monitoring & Maintenance](#monitoring--maintenance)
9. [Troubleshooting](#troubleshooting)

## Overview

The multitenancy feature allows multiple independent tenants (identified by `businessId`) to use the same database while ensuring complete data isolation. Each tenant's data is segregated at the database level through composite primary keys and filtered queries.

### Architecture

- **Tenant Identifier**: `businessId` (TEXT field)
- **Isolation Method**: Query-level filtering + Composite Primary Keys
- **Database**: PostgreSQL with pgx/v5 driver
- **Security Layers**:
  1. Application-level filtering (all queries include businessId)
  2. Database-level constraints (composite PKs and FKs)
  3. Optional: Row-Level Security (RLS) policies

## Prerequisites

### Required

- PostgreSQL 12 or higher
- Go 1.24 or higher
- pgx/v5 PostgreSQL driver

### Recommended

- PostgreSQL 14+ for better RLS performance
- Connection pooling configured (pgxpool)
- Database monitoring tools

## Initial Setup

### 1. Database Connection

```go
import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
    "go.mau.fi/whatsmeow/store/sqlstore"
    waLog "go.mau.fi/whatsmeow/util/log"
)

// Create connection pool
ctx := context.Background()
dbPool, err := pgxpool.New(ctx, "postgres://user:password@localhost:5432/whatsmeow?sslmode=require")
if err != nil {
    log.Fatalf("Failed to connect to database: %v", err)
}
defer dbPool.Close()
```

### 2. Create Container for Each Tenant

```go
// Create container with businessId
log := waLog.Stdout("Database", "INFO", true)
container := sqlstore.NewContainer(dbPool, "your-business-id", log)
defer container.Close()

// Upgrade database to latest version
err = container.Upgrade()
if err != nil {
    log.Fatalf("Failed to upgrade database: %v", err)
}
```

### 3. Manage Devices

```go
// Create new device
device := container.NewDevice()

// Or get existing device
jid, _ := types.ParseJID("1234567890@s.whatsapp.net")
device, err := container.GetDevice(ctx, jid)
if err != nil {
    log.Fatalf("Failed to get device: %v", err)
}

// Save device
err = container.PutDevice(ctx, device)
if err != nil {
    log.Fatalf("Failed to save device: %v", err)
}
```

## Database Migration

### For New Deployments

Simply call the `Upgrade()` method:

```go
err := container.Upgrade()
if err != nil {
    log.Fatalf("Database upgrade failed: %v", err)
}
```

This will:
1. Create all tables with businessId columns
2. Set up composite primary keys
3. Create foreign key constraints
4. Add performance indexes

### For Existing Deployments (Migration from Single-Tenant)

⚠️ **WARNING**: Migrating from single-tenant to multi-tenant requires careful planning.

#### Step 1: Backup Your Database

```bash
pg_dump -h localhost -U postgres whatsmeow > backup_$(date +%Y%m%d).sql
```

#### Step 2: Add businessId Column to Existing Tables

```sql
-- Add businessId column with default value for existing data
ALTER TABLE whatsmeow_device ADD COLUMN business_id TEXT;
UPDATE whatsmeow_device SET business_id = 'default-tenant' WHERE business_id IS NULL;
ALTER TABLE whatsmeow_device ALTER COLUMN business_id SET NOT NULL;

-- Repeat for all tables...
-- (See migration script below)
```

#### Step 3: Update Primary Keys

```sql
-- Drop old primary key
ALTER TABLE whatsmeow_device DROP CONSTRAINT whatsmeow_device_pkey;

-- Add new composite primary key
ALTER TABLE whatsmeow_device ADD PRIMARY KEY (business_id, jid);
```

#### Step 4: Update Foreign Keys

```sql
-- Example for whatsmeow_sessions
ALTER TABLE whatsmeow_sessions DROP CONSTRAINT whatsmeow_sessions_our_jid_fkey;
ALTER TABLE whatsmeow_sessions ADD FOREIGN KEY (business_id, our_jid)
    REFERENCES whatsmeow_device(business_id, jid) ON DELETE CASCADE ON UPDATE CASCADE;
```

#### Step 5: Apply Indexes

```sql
-- Run the upgrade or manually apply indexes
\i store/sqlstore/upgrades/12-security-improvements.sql
```

### Complete Migration Script

See `store/sqlstore/migration_single_to_multi.sql` for a complete migration script (create this file if migrating).

## Application Integration

### Multi-Tenant Application Example

```go
package main

import (
    "context"
    "sync"

    "github.com/jackc/pgx/v5/pgxpool"
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/store/sqlstore"
    waLog "go.mau.fi/whatsmeow/util/log"
)

type TenantManager struct {
    dbPool     *pgxpool.Pool
    containers map[string]*sqlstore.Container
    mu         sync.RWMutex
}

func NewTenantManager(dbURL string) (*TenantManager, error) {
    ctx := context.Background()
    dbPool, err := pgxpool.New(ctx, dbURL)
    if err != nil {
        return nil, err
    }

    return &TenantManager{
        dbPool:     dbPool,
        containers: make(map[string]*sqlstore.Container),
    }, nil
}

func (tm *TenantManager) GetContainer(businessId string) *sqlstore.Container {
    tm.mu.RLock()
    container, exists := tm.containers[businessId]
    tm.mu.RUnlock()

    if exists {
        return container
    }

    tm.mu.Lock()
    defer tm.mu.Unlock()

    // Double-check after acquiring write lock
    if container, exists := tm.containers[businessId]; exists {
        return container
    }

    // Create new container
    log := waLog.Stdout("WhatsApp", "INFO", true)
    container = sqlstore.NewContainer(tm.dbPool, businessId, log)
    tm.containers[businessId] = container

    return container
}

func (tm *TenantManager) CreateClient(businessId string, jid types.JID) (*whatsmeow.Client, error) {
    container := tm.GetContainer(businessId)
    device, err := container.GetDevice(context.Background(), jid)
    if err != nil {
        return nil, err
    }

    if device == nil {
        device = container.NewDevice()
        device.ID = &jid
    }

    return whatsmeow.NewClient(device, nil), nil
}

func (tm *TenantManager) Close() {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    for _, container := range tm.containers {
        container.Close()
    }
    tm.dbPool.Close()
}
```

### Usage Example

```go
func main() {
    // Initialize tenant manager
    manager, err := NewTenantManager("postgres://user:pass@localhost/whatsmeow")
    if err != nil {
        log.Fatal(err)
    }
    defer manager.Close()

    // Create client for tenant 1
    jid1, _ := types.ParseJID("1111111111@s.whatsapp.net")
    client1, err := manager.CreateClient("tenant-1", jid1)
    if err != nil {
        log.Fatal(err)
    }

    // Create client for tenant 2 (can use same JID!)
    client2, err := manager.CreateClient("tenant-2", jid1)
    if err != nil {
        log.Fatal(err)
    }

    // Each client is completely isolated
    client1.Connect()
    client2.Connect()
}
```

## Security Hardening (Optional)

### Enable Row-Level Security (RLS)

For defense-in-depth, enable PostgreSQL Row-Level Security:

```bash
psql -U postgres whatsmeow < store/sqlstore/rls_policies.sql
```

### Update Application to Set Session Variable

```go
func (c *Container) setSessionBusinessId(ctx context.Context) error {
    _, err := c.dbPool.Exec(ctx,
        "SET app.current_business_id = $1",
        c.businessId)
    return err
}

// Call before any queries
err := container.setSessionBusinessId(ctx)
```

### Validate businessId Input

```go
import "regexp"

var businessIdRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func validateBusinessId(businessId string) error {
    if businessId == "" {
        return errors.New("businessId cannot be empty")
    }
    if len(businessId) > 64 {
        return errors.New("businessId too long (max 64 characters)")
    }
    if !businessIdRegex.MatchString(businessId) {
        return errors.New("businessId contains invalid characters")
    }
    return nil
}
```

## Testing

### Unit Tests

Run the multitenancy tests:

```bash
# Set test database URL
export TEST_DB_URL="postgres://user:pass@localhost:5432/whatsmeow_test"

# Run tests
go test -v ./store/sqlstore -run TestCrossTenantIsolation
go test -v ./store/sqlstore -run TestDeleteCascade
```

### Manual Testing Checklist

- [ ] Create two containers with different businessIds
- [ ] Create devices with same JID in both containers
- [ ] Verify each container only sees its own devices
- [ ] Verify sessions are isolated
- [ ] Verify contacts are isolated
- [ ] Verify identity keys are isolated
- [ ] Test DELETE CASCADE behavior
- [ ] Verify composite PK allows same JID across tenants
- [ ] Check query performance with indexes

### Load Testing

```bash
# Use pgbench or custom load test
# Monitor query performance with EXPLAIN ANALYZE
```

## Monitoring & Maintenance

### Key Metrics to Monitor

1. **Query Performance**
   ```sql
   -- Check slow queries
   SELECT query, mean_exec_time, calls
   FROM pg_stat_statements
   WHERE query LIKE '%business_id%'
   ORDER BY mean_exec_time DESC
   LIMIT 10;
   ```

2. **Index Usage**
   ```sql
   SELECT schemaname, tablename, indexname, idx_scan
   FROM pg_stat_user_indexes
   WHERE schemaname = 'public'
   AND tablename LIKE 'whatsmeow_%'
   ORDER BY idx_scan ASC;
   ```

3. **Table Sizes Per Tenant**
   ```sql
   SELECT business_id, COUNT(*) as device_count
   FROM whatsmeow_device
   GROUP BY business_id;
   ```

### Regular Maintenance Tasks

1. **Vacuum & Analyze**
   ```sql
   VACUUM ANALYZE whatsmeow_device;
   -- Repeat for all tables
   ```

2. **Index Rebuild** (if fragmented)
   ```sql
   REINDEX TABLE whatsmeow_device;
   ```

3. **Audit Tenant Isolation**
   ```sql
   -- Verify no data leaks
   SELECT d1.business_id as tenant1, d2.business_id as tenant2
   FROM whatsmeow_device d1, whatsmeow_device d2
   WHERE d1.business_id != d2.business_id
   AND d1.jid = d2.jid;
   -- Should return rows (this is expected and correct)
   ```

## Troubleshooting

### Problem: Duplicate Key Error on (jid) Rather Than (business_id, jid)

**Cause**: Schema not properly updated with composite primary key

**Solution**:
```sql
-- Check current primary key
SELECT conname, contype, conrelid::regclass
FROM pg_constraint
WHERE conrelid = 'whatsmeow_device'::regclass;

-- If wrong, recreate:
ALTER TABLE whatsmeow_device DROP CONSTRAINT whatsmeow_device_pkey;
ALTER TABLE whatsmeow_device ADD PRIMARY KEY (business_id, jid);
```

### Problem: Foreign Key Constraint Violations

**Cause**: FK constraints not updated to include businessId

**Solution**: Recreate foreign keys with composite references (see migration script)

### Problem: Slow Queries

**Cause**: Missing indexes on business_id

**Solution**:
```bash
# Run index creation
go run -tags=postgres ./cmd/upgrade_indexes.go

# Or manually:
psql -U postgres whatsmeow < store/sqlstore/upgrades/12-security-improvements.sql
```

### Problem: Cross-Tenant Data Leakage

**Cause**: Query missing businessId filter

**Solution**:
1. Review all custom queries
2. Ensure all WHERE clauses include `business_id = $1`
3. Enable RLS for additional protection
4. Run isolation tests

### Problem: Connection Pool Exhaustion

**Cause**: Too many containers sharing one pool

**Solution**:
```go
// Configure pool settings
config, _ := pgxpool.ParseConfig(dbURL)
config.MaxConns = 100  // Adjust based on your needs
config.MinConns = 10
dbPool, _ := pgxpool.NewWithConfig(ctx, config)
```

## Best Practices

1. **Always validate businessId** before creating containers
2. **Use connection pooling** - share dbPool across containers
3. **Monitor index usage** and query performance
4. **Set up alerting** for constraint violations
5. **Regular backups** with point-in-time recovery
6. **Test isolation** in staging before production
7. **Document businessId** naming conventions for your org
8. **Audit logs** should include businessId for all operations
9. **Consider RLS** for high-security requirements
10. **Plan capacity** based on number of tenants

## Support & Security

For security issues or questions:
- Review `SECURITY_ANALYSIS_MULTITENANCY.md`
- Check unit tests in `multitenancy_test.go`
- Consult PostgreSQL documentation on RLS

## Version History

- **v2**: Added performance indexes and RLS support
- **v1**: Initial multitenancy implementation with businessId

---

**Last Updated**: 2025
**Compatible with**: PostgreSQL 12+, Go 1.24+
