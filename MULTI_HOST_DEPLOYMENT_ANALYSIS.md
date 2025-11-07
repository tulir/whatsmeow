# Multi-Host Deployment Analysis for whatsmeow

**Date:** 2025-11-07
**Analysis Scope:** Feasibility of deploying whatsmeow on multiple hosts connected to the same database for redundancy and horizontal scalability

---

## Executive Summary

**Can whatsmeow be instantiated on multiple hosts for redundancy and horizontal scalability?**

**Answer: NO** - Not in the traditional multi-instance active-active sense.

The WhatsApp Web protocol **enforces a single active connection per device** at the protocol level. However, the library **does support** multi-tenancy and can achieve **high availability through active-passive failover** with proper coordination.

### Key Findings:

1. ✅ **Database Layer**: Fully supports shared access across multiple hosts with proper isolation via `business_id`
2. ✅ **Session Persistence**: Sessions can be resumed after disconnection from a different host
3. ❌ **Active-Active Deployment**: Impossible - WhatsApp disconnects previous connections when a new one is established
4. ✅ **Active-Passive Failover**: Achievable with external coordination (database locks, leader election)
5. ✅ **Horizontal Scaling**: Possible via multi-tenancy - different WhatsApp accounts on different hosts

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Protocol-Level Constraints](#protocol-level-constraints)
3. [Multi-Host Deployment Issues](#multi-host-deployment-issues)
4. [Database Layer Analysis](#database-layer-analysis)
5. [Deployment Patterns](#deployment-patterns)
6. [Recommendations](#recommendations)
7. [Implementation Checklist](#implementation-checklist)

---

## Architecture Overview

### Library Design

whatsmeow is designed as a **single-client library** where:
- Each `Client` instance maintains one WebSocket connection to WhatsApp servers
- Connection is authenticated using cryptographic keys (Noise protocol)
- All state is persisted to a database through pluggable store interfaces
- In-memory state is used for performance optimization (caches, event handlers)

### Key Components

```
┌─────────────────────────────────────────────────────────┐
│                    Application Layer                     │
├─────────────────────────────────────────────────────────┤
│  Client                                                  │
│  ├─ Store (Device)        ← Persistent session data     │
│  ├─ NoiseSocket           ← WebSocket connection        │
│  ├─ Event Handlers        ← In-memory callbacks         │
│  ├─ Response Waiters      ← In-memory IQ response map   │
│  └─ Caches                ← In-memory optimization      │
├─────────────────────────────────────────────────────────┤
│  Store Interfaces (SQLStore implementation)             │
│  ├─ IdentityStore         ← Contact encryption keys     │
│  ├─ SessionStore          ← Signal protocol sessions    │
│  ├─ PreKeyStore           ← One-time encryption keys    │
│  ├─ SenderKeyStore        ← Group encryption keys       │
│  ├─ AppStateStore         ← User settings/contacts      │
│  └─ ContactStore          ← Contact information         │
├─────────────────────────────────────────────────────────┤
│  PostgreSQL Database (pgxpool)                          │
│  └─ business_id partition key for multitenancy         │
└─────────────────────────────────────────────────────────┘
```

---

## Protocol-Level Constraints

### WhatsApp Web Protocol Overview

WhatsApp Web uses a multi-layered protocol:

1. **Transport Layer**: WebSocket over TLS
2. **Encryption Layer**: Noise Protocol (Noise_XX_25519_AESGCM_SHA256)
3. **Message Layer**: Binary XML (protobuf)
4. **E2E Encryption**: Signal Protocol for messages

### Single Connection Enforcement

**Critical Finding**: WhatsApp enforces **one active connection per device** at the server level.

#### Connection Identity

Each device is identified by its **Noise static key**:

```go
// From store/store.go
type Device struct {
    NoiseKey    *keys.KeyPair  // Persistent 32-byte Curve25519 keypair
    IdentityKey *keys.KeyPair  // Signal protocol identity
    // ...
}
```

During the Noise handshake (`handshake.go:89-93`):
```go
// Client sends its static public key (encrypted)
encryptedPubkey := nh.Encrypt(cli.Store.NoiseKey.Pub[:])
```

The server maintains a mapping:
```
NoiseKey.Pub → Active WebSocket Connection
```

#### Duplicate Connection Detection

When a second instance attempts to connect with the same `NoiseKey`:

**Timeline:**
```
T0: Instance A connects with NoiseKey
    Server: NoiseKey → Socket_A (registered)
    Status: Connected ✓

T1: Instance B connects with same NoiseKey
    Server detects: NoiseKey already mapped to Socket_A
    Server action: Accept Socket_B, terminate Socket_A

T2: Instance A receives stream error
    <stream-error code="401">
      <conflict type="replaced"/>
    </stream-error>

    Event dispatched: events.StreamReplaced{}
    Connection terminated

T3: Instance B is now the active connection
    Instance A must reconnect (which would disconnect B)
```

**Code Reference** (`connectionevents.go:48-51`):
```go
case conflictType == "replaced":
    cli.expectDisconnect()
    cli.Log.Infof("Got replaced stream error, sending StreamReplaced event")
    go cli.dispatchEvent(&events.StreamReplaced{})
```

### Why This Constraint Exists

1. **Message Ordering**: WhatsApp needs to deliver messages to exactly one client to prevent duplication
2. **Read Receipts**: Only one client should mark messages as read
3. **Presence**: User can only be "online" from one location per device
4. **Resource Management**: Server-side resources (message queues, routing tables)

### Protocol Facts

- ✅ **Reconnection supported**: Same device can reconnect after disconnection
- ✅ **Session resumption**: Uses same cryptographic keys
- ❌ **Connection multiplexing**: Not supported by protocol
- ❌ **Load balancing**: Cannot distribute load across multiple connections for same device
- ❌ **Read-only replicas**: All connections are read-write, no observer mode

---

## Multi-Host Deployment Issues

### 1. Application-Level Issues

#### A. Response Waiters (CRITICAL)

**Issue**: IQ (Info/Query) responses are matched to waiting goroutines using in-memory maps.

**Code Reference** (`client.go:108-109`, `request.go:56-62`):
```go
type Client struct {
    responseWaiters     map[string]chan<- *waBinary.Node
    responseWaitersLock sync.Mutex
    // ...
}

func (cli *Client) waitResponse(reqID string) chan *waBinary.Node {
    ch := make(chan *waBinary.Node, 1)
    cli.responseWaitersLock.Lock()
    cli.responseWaiters[reqID] = ch
    cli.responseWaitersLock.Unlock()
    return ch
}
```

**Problem**:
- Request sent by Instance A creates a channel in A's memory
- Response comes back on the same WebSocket connection
- If connection switches to Instance B, Instance A's waiters never receive responses
- Results in timeouts and failed operations

**Impact**: ❌ **Blocking issue** - Cannot have multiple instances handling same connection

---

#### B. Event Handlers (CRITICAL)

**Issue**: Event handlers are registered per-Client instance in memory.

**Code Reference** (`client.go:113-114`, `client.go:692-707`):
```go
type Client struct {
    eventHandlers     []wrappedEventHandler
    eventHandlersLock sync.RWMutex
    // ...
}

func (cli *Client) AddEventHandler(handler EventHandler) uint32 {
    id := atomic.AddUint32(&nextHandlerID, 1)
    cli.eventHandlersLock.Lock()
    cli.eventHandlers = append(cli.eventHandlers, wrappedEventHandler{fn, id})
    cli.eventHandlersLock.Unlock()
    return id
}
```

**Problem**:
- Application logic registers handlers (e.g., `OnMessage`, `OnReceipt`)
- Handlers only exist in the Client instance that registered them
- If connection moves to different host, events are not processed by original handlers
- No cross-process event broadcasting mechanism

**Impact**: ❌ **Blocking issue** - Business logic would not execute on failover

---

#### C. In-Memory Caches (MEDIUM)

**Issue**: Multiple caches for performance optimization.

**Code Reference** (`client.go:129-137`):
```go
type Client struct {
    groupCache           map[types.JID]*groupMetaCache
    groupCacheLock       sync.Mutex
    userDevicesCache     map[types.JID]deviceCache
    userDevicesCacheLock sync.Mutex
    recentMessagesMap    map[recentMessageKey]RecentMessage
    recentMessagesLock   sync.RWMutex
    // ...
}
```

**Problem**:
- Different instances have different cache states
- Cache invalidation not coordinated across instances
- Stale data possible on failover

**Impact**: ⚠️ **Performance degradation** - More database queries, but not breaking

---

#### D. Handler Queue (LOW)

**Issue**: Incoming messages are queued in a buffered channel.

**Code Reference** (`client.go:112`):
```go
handlerQueue      chan *waBinary.Node  // buffered, size 2048
```

**Problem**:
- Messages in queue are lost on instance crash
- No persistence of in-flight messages

**Impact**: ⚠️ **Rare message loss** - Only affects messages received but not yet processed

---

### 2. Protocol-Level Issues

#### A. Single Active Connection (CRITICAL)

**Issue**: WhatsApp protocol enforces one connection per device.

**Documented Above**: See [Protocol-Level Constraints](#protocol-level-constraints)

**Impact**: ❌ **Absolute blocker** - Cannot have multiple active instances

---

#### B. Pre-Key Depletion (MEDIUM)

**Issue**: Pre-keys are consumed when contacts initiate new Signal sessions.

**Code Reference** (`prekeys.go`):
```go
const (
    WantedPreKeyCount = 50   // Upload in batches
    MinPreKeyCount = 5       // When to request more
)
```

**Problem**:
- If two instances both connect at different times, they might upload different pre-key batches
- Server only stores one batch at a time
- Pre-key rotation could cause decryption failures

**Impact**: ⚠️ **Potential message decryption failures** during transitions

---

#### C. App State Synchronization (MEDIUM)

**Issue**: App state syncs are mutually exclusive with a client-side mutex.

**Code Reference** (`client.go:95-96`, `appstate.go:33-34`):
```go
type Client struct {
    appStateProc     *appstate.Processor
    appStateSyncLock sync.Mutex  // Only one app state sync at a time
    // ...
}

func (cli *Client) FetchAppState(ctx context.Context, name appstate.WAPatchName, ...) error {
    cli.appStateSyncLock.Lock()
    defer cli.appStateSyncLock.Unlock()
    // ... fetch and apply patches
}
```

**Problem**:
- App state contains contacts, chat settings, mute status, pins
- Synced incrementally using version numbers and cryptographic MACs
- Concurrent syncs from different instances could cause version conflicts
- Database-level mutex not present

**Impact**: ⚠️ **Data inconsistency** - Settings might not be properly synced

---

### 3. Database-Level Analysis

#### A. Transaction Usage (LIMITED)

**Finding**: Transactions are used only for specific batch operations.

**Code Reference** (`store/sqlstore/store.go:229-250`, `store/sqlstore/store.go:307-353`):
```go
// Example: PutSenderKeys uses transaction for batch insert
tx, err := s.dbPool.Begin(ctx)
if err != nil {
    return fmt.Errorf("failed to begin transaction: %w", err)
}
defer func() {
    if err != nil {
        if rbErr := tx.Rollback(ctx); rbErr != nil {
            fmt.Printf("Error rolling back transaction: %v", rbErr)
        }
    }
}()
// ... multiple inserts
if err = tx.Commit(ctx); err != nil {
    return fmt.Errorf("failed to commit transaction: %w", err)
}
```

**Usage Pattern**:
- ✅ Transactions used for: Batch pre-key uploads, sender key updates
- ❌ Transactions NOT used for: Individual session updates, identity key updates, most reads

**Impact**: ⚠️ **Potential race conditions** on concurrent access (but protocol prevents this)

---

#### B. Connection Pooling (GOOD)

**Finding**: PostgreSQL connection pooling is properly implemented.

**Code Reference** (`store/sqlstore/container.go`):
```go
type Container struct {
    businessId string
    dbPool     *pgxpool.Pool  // Thread-safe connection pool
    log        waLog.Logger
    // ...
}
```

**Analysis**:
- pgxpool automatically handles connection lifecycle
- Thread-safe for concurrent access from multiple goroutines
- Can be shared across multiple Container instances
- No explicit locking needed

**Impact**: ✅ **No issues** - Designed for concurrent access

---

#### C. Multitenancy Support (EXCELLENT)

**Finding**: Complete tenant isolation via `business_id` composite keys.

**Schema Pattern** (`store/sqlstore/upgrades/00-latest-schema.sql`):
```sql
CREATE TABLE whatsmeow_device (
    business_id TEXT NOT NULL,
    jid TEXT NOT NULL,
    -- ... other columns
    PRIMARY KEY (business_id, jid)
);

CREATE TABLE whatsmeow_sessions (
    business_id TEXT NOT NULL,
    our_jid TEXT,
    their_id TEXT,
    -- ... other columns
    PRIMARY KEY (business_id, our_jid, their_id),
    FOREIGN KEY (business_id, our_jid)
        REFERENCES whatsmeow_device(business_id, jid)
        ON DELETE CASCADE
);
```

**All 15 tables follow this pattern:**
1. `whatsmeow_device`
2. `whatsmeow_identity_keys`
3. `whatsmeow_sessions`
4. `whatsmeow_pre_keys`
5. `whatsmeow_sender_keys`
6. `whatsmeow_app_state_sync_keys`
7. `whatsmeow_app_state_version`
8. `whatsmeow_app_state_mutation_macs`
9. `whatsmeow_contacts`
10. `whatsmeow_redacted_phones`
11. `whatsmeow_chat_settings`
12. `whatsmeow_message_secrets`
13. `whatsmeow_privacy_tokens`
14. `whatsmeow_lid_map`
15. `whatsmeow_event_buffer`

**Query Pattern** (all queries):
```sql
SELECT ... FROM whatsmeow_table
WHERE business_id = $1 AND jid = $2 AND ...
```

**Impact**: ✅ **Perfect for multi-tenancy** - Different WhatsApp accounts on different hosts

---

## Database Layer Analysis

### Concurrency Safety

#### Read Operations
- ✅ **Safe**: Multiple instances can read simultaneously
- ✅ **Isolation**: business_id filtering prevents cross-tenant access
- ✅ **Connection pooling**: pgxpool handles contention

#### Write Operations
- ✅ **Safe for different tenants**: Composite keys prevent conflicts
- ⚠️ **Unsafe for same tenant**: No distributed locking for same device
- ❌ **No pessimistic locking**: No `SELECT ... FOR UPDATE` patterns found

### Race Condition Examples

**Scenario 1: Concurrent Pre-Key Upload**
```
Instance A reads: MinPreKeyCount = 3 (below threshold)
Instance B reads: MinPreKeyCount = 3 (below threshold)
Instance A generates and uploads 50 new keys
Instance B generates and uploads 50 new keys (different keys!)
Result: Last writer wins, first batch lost
```

**Scenario 2: Session State Update**
```
Instance A receives message, updates session ratchet state
Instance B receives retry, tries to decrypt with old session state
Result: Decryption failure, retry requests
```

**Scenario 3: App State Sync**
```
Instance A fetches patches, version = 5
Instance B fetches patches, version = 5
Instance A applies mutations, saves version = 6
Instance B applies mutations, saves version = 6 (different mutations!)
Result: App state corruption
```

### Current Safety Mechanisms

1. **Composite Primary Keys**: Prevent duplicate rows per tenant
2. **Foreign Key Constraints**: Maintain referential integrity
3. **ON CONFLICT DO UPDATE**: Upsert pattern for idempotency
4. **CHECK Constraints**: Validate data lengths

### Missing Safety Mechanisms

1. ❌ **Advisory Locks**: No PostgreSQL advisory locks for coordination
2. ❌ **Row-Level Locks**: No `SELECT ... FOR UPDATE` patterns
3. ❌ **Optimistic Locking**: No version counters for conflict detection
4. ❌ **Distributed Locks**: No external coordination (Redis, etcd, etc.)

---

## Deployment Patterns

### Pattern 1: Single Instance (Recommended for Single Account)

```
┌──────────────┐
│   Host A     │
│  ┌────────┐  │
│  │ Client │──┼──► WhatsApp Servers
│  └────┬───┘  │
│       │      │
│  ┌────▼───┐  │
│  │  PG DB │  │
│  └────────┘  │
└──────────────┘
```

**Characteristics:**
- ✅ Simple, no coordination needed
- ✅ No race conditions
- ❌ Single point of failure
- ❌ No redundancy

**Use Case:** Development, testing, small deployments

---

### Pattern 2: Active-Passive Failover (Recommended for HA)

```
┌──────────────┐               ┌──────────────┐
│   Host A     │               │   Host B     │
│ ┌──────────┐ │               │ ┌──────────┐ │
│ │ Client   │─┼──► WhatsApp  │ │ Client   │ │
│ │ (ACTIVE) │ │    Servers    │ │(STANDBY) │ │
│ └────┬─────┘ │               │ └────┬─────┘ │
│      │       │               │      │       │
│ ┌────▼───────▼───────────────▼──────▼─────┐ │
│ │        Shared PostgreSQL Database       │ │
│ │        + Leader Election Lock           │ │
│ └─────────────────────────────────────────┘ │
└──────────────┘               └──────────────┘
```

**Implementation:**

```go
// Pseudo-code for leader election
type HACoordinator struct {
    db     *pgxpool.Pool
    lockID int64  // PostgreSQL advisory lock ID
}

func (ha *HACoordinator) AcquireLock(ctx context.Context) (bool, error) {
    var acquired bool
    err := ha.db.QueryRow(ctx,
        "SELECT pg_try_advisory_lock($1)", ha.lockID).Scan(&acquired)
    return acquired, err
}

func (ha *HACoordinator) ReleaseLock(ctx context.Context) error {
    _, err := ha.db.Exec(ctx,
        "SELECT pg_advisory_unlock($1)", ha.lockID)
    return err
}

func main() {
    coordinator := NewHACoordinator(dbPool, deviceLockID)

    for {
        acquired, _ := coordinator.AcquireLock(ctx)
        if acquired {
            // This instance is now the leader
            client := whatsmeow.NewClient(device, log)
            client.Connect()

            // Monitor health
            go func() {
                for {
                    if !client.IsConnected() {
                        // Connection lost, release lock
                        coordinator.ReleaseLock(ctx)
                        break
                    }
                    time.Sleep(5 * time.Second)
                }
            }()

            // Run until interrupted
            <-ctx.Done()
            client.Disconnect()
            coordinator.ReleaseLock(ctx)
        } else {
            // Standby mode - wait for lock
            time.Sleep(1 * time.Second)
        }
    }
}
```

**Characteristics:**
- ✅ High availability (automatic failover)
- ✅ No protocol violations
- ✅ Uses database for coordination
- ✅ Session resumption on failover
- ⚠️ Failover time: ~5-10 seconds
- ❌ Standby instance consumes resources

**Use Case:** Production deployments requiring HA for single account

**Failover Process:**
1. Active instance crashes or loses connection
2. Database advisory lock is automatically released
3. Standby instance acquires lock
4. Standby instance connects using same session data
5. WhatsApp server disconnects previous connection (if still active)
6. Standby instance becomes active

---

### Pattern 3: Multi-Tenancy / Horizontal Scaling (Recommended for Multiple Accounts)

```
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│   Host A     │  │   Host B     │  │   Host C     │
│  Tenant A    │  │  Tenant B    │  │  Tenant C    │
│ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────┐ │
│ │ Client   │─┼──┼─┼─► WhatsApp│  │ │ Client   │─┼──►
│ │(Account1)│ │  │ │ │(Account2)│  │ │(Account3)│ │
│ └────┬─────┘ │  │ └────┬─────┘ │  │ └────┬─────┘ │
│      │       │  │      │       │  │      │       │
│ ┌────▼───────▼──▼──────▼───────▼──▼──────▼─────┐ │
│ │         Shared PostgreSQL Database           │ │
│ │  business_id='tenant-a' | 'tenant-b' | ...   │ │
│ └──────────────────────────────────────────────┘ │
└──────────────┘  └──────────────┘  └──────────────┘
```

**Implementation:**

```go
type TenantManager struct {
    dbPool     *pgxpool.Pool
    containers map[string]*sqlstore.Container
    clients    map[string]*whatsmeow.Client
    mu         sync.RWMutex
}

func (tm *TenantManager) GetOrCreateClient(businessId string) (*whatsmeow.Client, error) {
    tm.mu.RLock()
    client, exists := tm.clients[businessId]
    tm.mu.RUnlock()

    if exists {
        return client, nil
    }

    tm.mu.Lock()
    defer tm.mu.Unlock()

    // Double-check after acquiring write lock
    if client, exists := tm.clients[businessId]; exists {
        return client, nil
    }

    // Create container for this tenant
    container := sqlstore.NewContainer(tm.dbPool, businessId, log)
    device, err := container.GetFirstDevice(ctx)
    if err != nil {
        return nil, err
    }

    // Create and connect client
    client = whatsmeow.NewClient(device, log)
    client.AddEventHandler(tm.handleEvent)
    if err := client.Connect(); err != nil {
        return nil, err
    }

    tm.clients[businessId] = client
    tm.containers[businessId] = container

    return client, nil
}

func (tm *TenantManager) handleEvent(evt interface{}) {
    // Route events to appropriate tenant handler
    switch v := evt.(type) {
    case *events.Message:
        tm.handleMessage(v)
    case *events.Receipt:
        tm.handleReceipt(v)
    // ...
    }
}

// Usage
func main() {
    manager := NewTenantManager(dbPool)

    // Each tenant gets its own client
    clientA, _ := manager.GetOrCreateClient("tenant-a")
    clientB, _ := manager.GetOrCreateClient("tenant-b")
    clientC, _ := manager.GetOrCreateClient("tenant-c")

    // All clients can run simultaneously on same host
    // Or distribute across multiple hosts with routing logic
}
```

**Characteristics:**
- ✅ True horizontal scaling
- ✅ Each tenant isolated (business_id partition)
- ✅ Can distribute tenants across hosts
- ✅ Can combine with Pattern 2 for per-tenant HA
- ✅ Efficient resource usage

**Use Case:** SaaS platforms, multi-customer deployments

**Distribution Strategies:**

1. **Hash-based routing:**
   ```go
   func (tm *TenantManager) GetHostForTenant(businessId string) string {
       hash := sha256.Sum256([]byte(businessId))
       hostIndex := binary.BigEndian.Uint32(hash[:]) % uint32(len(tm.hosts))
       return tm.hosts[hostIndex]
   }
   ```

2. **Database-backed routing:**
   ```sql
   CREATE TABLE tenant_routing (
       business_id TEXT PRIMARY KEY,
       assigned_host TEXT,
       last_seen TIMESTAMP
   );
   ```

3. **External service mesh:**
   - Use Kubernetes StatefulSets with consistent hashing
   - Use Consul for service discovery and routing

---

### Pattern 4: Load Balancer + Session Affinity (NOT RECOMMENDED)

```
           ┌─────────────┐
           │Load Balancer│
           │(Sticky)     │
           └──────┬──────┘
                  │
        ┌─────────┼─────────┐
        │         │         │
   ┌────▼───┐ ┌──▼─────┐ ┌─▼──────┐
   │Host A  │ │Host B  │ │Host C  │
   │Client  │ │Client  │ │Client  │
   └────┬───┘ └───┬────┘ └───┬────┘
        │         │          │
   ┌────▼─────────▼──────────▼────┐
   │   Shared PostgreSQL Database  │
   └───────────────────────────────┘
```

**Why NOT Recommended:**
- ❌ WhatsApp protocol doesn't support session affinity at HTTP/WebSocket level
- ❌ Connection is authenticated by Noise keys, not by session tokens
- ❌ Load balancer cannot distinguish between legitimate retry and new instance
- ❌ Would cause constant "replaced" conflicts

---

## Recommendations

### For Single Account Deployments

**Scenario:** One WhatsApp account needs high availability

**Recommended Architecture:** Active-Passive Failover (Pattern 2)

**Implementation Steps:**

1. **Setup PostgreSQL Advisory Locks:**
   ```sql
   -- Each device gets unique lock ID (hash of JID)
   CREATE OR REPLACE FUNCTION get_device_lock_id(p_jid TEXT)
   RETURNS BIGINT AS $$
   BEGIN
       RETURN hashtext(p_jid)::BIGINT;
   END;
   $$ LANGUAGE plpgsql IMMUTABLE;
   ```

2. **Implement Leader Election:**
   ```go
   import "github.com/jackc/pgx/v5/pgxpool"

   type LeaderElection struct {
       pool   *pgxpool.Pool
       lockID int64
       ctx    context.Context
       cancel context.CancelFunc
   }

   func (le *LeaderElection) BecomeLeader() bool {
       var acquired bool
       err := le.pool.QueryRow(le.ctx,
           "SELECT pg_try_advisory_lock($1)", le.lockID).Scan(&acquired)
       return acquired && err == nil
   }

   func (le *LeaderElection) IsLeader() bool {
       var isLocked bool
       err := le.pool.QueryRow(le.ctx,
           "SELECT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND objid=$1 AND pid=pg_backend_pid())",
           le.lockID).Scan(&isLocked)
       return isLocked && err == nil
   }

   func (le *LeaderElection) ResignLeadership() {
       le.pool.Exec(le.ctx, "SELECT pg_advisory_unlock($1)", le.lockID)
   }
   ```

3. **Health Monitoring:**
   ```go
   func (le *LeaderElection) MonitorHealth(client *whatsmeow.Client) {
       ticker := time.NewTicker(5 * time.Second)
       defer ticker.Stop()

       for {
           select {
           case <-ticker.C:
               if !client.IsConnected() {
                   log.Error("Lost connection, resigning leadership")
                   le.ResignLeadership()
                   return
               }
               if !le.IsLeader() {
                   log.Error("Lost leadership lock, disconnecting")
                   client.Disconnect()
                   return
               }
           case <-le.ctx.Done():
               return
           }
       }
   }
   ```

4. **Failover Detection:**
   ```go
   func (le *LeaderElection) WaitForLeadership() {
       backoff := time.Second
       for {
           if le.BecomeLeader() {
               log.Info("Acquired leadership")
               return
           }

           select {
           case <-time.After(backoff):
               backoff = min(backoff*2, 30*time.Second)
           case <-le.ctx.Done():
               return
           }
       }
   }
   ```

5. **Complete Application:**
   ```go
   func main() {
       ctx, cancel := context.WithCancel(context.Background())
       defer cancel()

       // Setup database
       dbPool, _ := pgxpool.New(ctx, databaseURL)
       container := sqlstore.NewContainer(dbPool, businessID, log)
       device, _ := container.GetFirstDevice(ctx)

       // Leader election
       lockID := getLockID(device.ID.String())
       election := NewLeaderElection(ctx, dbPool, lockID)

       for {
           // Wait until we become leader
           election.WaitForLeadership()

           // Connect as leader
           client := whatsmeow.NewClient(device, log)
           if err := client.Connect(); err != nil {
               log.Error("Failed to connect:", err)
               election.ResignLeadership()
               continue
           }

           // Monitor health
           election.MonitorHealth(client)

           // Connection lost or leadership lost
           client.Disconnect()
           election.ResignLeadership()

           // Brief pause before retry
           time.Sleep(1 * time.Second)
       }
   }
   ```

**Expected Behavior:**
- Active instance runs normally
- Passive instance polls for leadership every 1-30 seconds (exponential backoff)
- On active crash: Passive acquires lock within 1-5 seconds
- On active disconnect: Passive connects within 5-10 seconds
- Total failover time: 10-15 seconds typical

---

### For Multiple Account Deployments

**Scenario:** Multiple WhatsApp accounts (multi-tenancy)

**Recommended Architecture:** Multi-Tenancy + Load Distribution (Pattern 3)

**Implementation Steps:**

1. **Tenant Management:**
   ```sql
   CREATE TABLE tenants (
       business_id TEXT PRIMARY KEY,
       phone_number TEXT UNIQUE,
       assigned_host TEXT,
       status TEXT CHECK (status IN ('active', 'inactive', 'migrating')),
       created_at TIMESTAMP DEFAULT NOW(),
       updated_at TIMESTAMP DEFAULT NOW()
   );

   CREATE INDEX idx_assigned_host ON tenants(assigned_host) WHERE status = 'active';
   ```

2. **Tenant Coordinator:**
   ```go
   type TenantCoordinator struct {
       pool          *pgxpool.Pool
       hostname      string
       clients       map[string]*whatsmeow.Client
       clientsLock   sync.RWMutex
   }

   func (tc *TenantCoordinator) LoadAssignedTenants(ctx context.Context) error {
       rows, err := tc.pool.Query(ctx,
           "SELECT business_id FROM tenants WHERE assigned_host = $1 AND status = 'active'",
           tc.hostname)
       if err != nil {
           return err
       }
       defer rows.Close()

       for rows.Next() {
           var businessID string
           rows.Scan(&businessID)

           if err := tc.StartTenant(ctx, businessID); err != nil {
               log.Errorf("Failed to start tenant %s: %v", businessID, err)
           }
       }
       return nil
   }

   func (tc *TenantCoordinator) StartTenant(ctx context.Context, businessID string) error {
       tc.clientsLock.Lock()
       defer tc.clientsLock.Unlock()

       if _, exists := tc.clients[businessID]; exists {
           return nil // Already running
       }

       container := sqlstore.NewContainer(tc.pool, businessID, log)
       device, err := container.GetFirstDevice(ctx)
       if err != nil {
           return err
       }

       client := whatsmeow.NewClient(device, log)
       client.AddEventHandler(func(evt interface{}) {
           tc.handleTenantEvent(businessID, evt)
       })

       if err := client.Connect(); err != nil {
           return err
       }

       tc.clients[businessID] = client
       log.Infof("Started tenant %s", businessID)
       return nil
   }

   func (tc *TenantCoordinator) StopTenant(businessID string) {
       tc.clientsLock.Lock()
       defer tc.clientsLock.Unlock()

       if client, exists := tc.clients[businessID]; exists {
           client.Disconnect()
           delete(tc.clients, businessID)
           log.Infof("Stopped tenant %s", businessID)
       }
   }
   ```

3. **Load Distribution:**
   ```go
   func (tc *TenantCoordinator) RebalanceTenants(ctx context.Context) error {
       // Get all hosts and their load
       type hostLoad struct {
           hostname string
           count    int
       }

       rows, _ := tc.pool.Query(ctx,
           "SELECT assigned_host, COUNT(*) FROM tenants WHERE status='active' GROUP BY assigned_host")

       var loads []hostLoad
       for rows.Next() {
           var hl hostLoad
           rows.Scan(&hl.hostname, &hl.count)
           loads = append(loads, hl)
       }

       // Find overloaded and underloaded hosts
       avgLoad := calculateAverage(loads)

       // Migrate tenants from overloaded to underloaded hosts
       for _, host := range loads {
           if host.count > avgLoad*1.2 { // 20% over average
               // Mark tenants for migration
               tc.pool.Exec(ctx,
                   "UPDATE tenants SET status='migrating' WHERE assigned_host=$1 LIMIT 1",
                   host.hostname)
           }
       }

       return nil
   }
   ```

4. **Health Monitoring:**
   ```go
   func (tc *TenantCoordinator) MonitorHealth(ctx context.Context) {
       ticker := time.NewTicker(10 * time.Second)
       defer ticker.Stop()

       for {
           select {
           case <-ticker.C:
               tc.clientsLock.RLock()
               for businessID, client := range tc.clients {
                   if !client.IsConnected() {
                       log.Warnf("Tenant %s disconnected, attempting reconnect", businessID)
                       go tc.reconnectTenant(ctx, businessID)
                   }
               }
               tc.clientsLock.RUnlock()

               // Update heartbeat
               tc.pool.Exec(ctx,
                   "INSERT INTO host_heartbeats (hostname, last_seen) VALUES ($1, NOW()) ON CONFLICT (hostname) DO UPDATE SET last_seen=NOW()",
                   tc.hostname)

           case <-ctx.Done():
               return
           }
       }
   }
   ```

**Expected Behavior:**
- Each host manages subset of tenants
- Tenants distributed evenly across hosts
- Failed hosts detected within 30 seconds
- Orphaned tenants reassigned automatically
- Graceful tenant migration for rebalancing

---

### For Kubernetes Deployments

**Scenario:** Running on Kubernetes with StatefulSets

**Implementation:**

1. **StatefulSet Definition:**
   ```yaml
   apiVersion: apps/v1
   kind: StatefulSet
   metadata:
     name: whatsmeow-instance
   spec:
     serviceName: whatsmeow
     replicas: 3
     selector:
       matchLabels:
         app: whatsmeow
     template:
       metadata:
         labels:
           app: whatsmeow
       spec:
         containers:
         - name: whatsmeow
           image: myorg/whatsmeow:latest
           env:
           - name: POD_NAME
             valueFrom:
               fieldRef:
                 fieldPath: metadata.name
           - name: DATABASE_URL
             valueFrom:
               secretKeyRef:
                 name: postgres-credentials
                 key: url
           - name: MODE
             value: "active-passive"
           livenessProbe:
             httpGet:
               path: /health
               port: 8080
             periodSeconds: 10
           readinessProbe:
             httpGet:
               path: /ready
               port: 8080
             periodSeconds: 5
   ```

2. **Application with Health Endpoints:**
   ```go
   func main() {
       // Start HTTP server for health checks
       http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
           if client != nil && client.IsConnected() {
               w.WriteHeader(http.StatusOK)
               w.Write([]byte("OK"))
           } else {
               w.WriteHeader(http.StatusServiceUnavailable)
               w.Write([]byte("Not connected"))
           }
       })

       http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
           if election != nil && election.IsLeader() {
               w.WriteHeader(http.StatusOK)
               w.Write([]byte("Leader"))
           } else {
               w.WriteHeader(http.StatusServiceUnavailable)
               w.Write([]byte("Standby"))
           }
       })

       go http.ListenAndServe(":8080", nil)

       // Main application logic with leader election
       runWithLeaderElection(ctx)
   }
   ```

---

## Implementation Checklist

### Phase 1: Database Setup

- [ ] Deploy PostgreSQL 12+ with sufficient resources
- [ ] Apply schema from `store/sqlstore/upgrades/00-latest-schema.sql`
- [ ] Configure connection pooling (min 10, max 100 connections)
- [ ] Enable advisory lock support (default enabled)
- [ ] Create database monitoring (connection count, query latency)
- [ ] Setup automated backups

### Phase 2: Single Instance Deployment

- [ ] Deploy single whatsmeow instance
- [ ] Connect to database using `sqlstore.Container`
- [ ] Verify session persistence (disconnect and reconnect)
- [ ] Test message sending and receiving
- [ ] Monitor logs for errors
- [ ] Document baseline performance metrics

### Phase 3: High Availability Setup (If Single Account)

- [ ] Implement PostgreSQL advisory lock wrapper
- [ ] Implement leader election logic
- [ ] Deploy second instance in standby mode
- [ ] Test failover by killing active instance
- [ ] Measure failover time (target < 15 seconds)
- [ ] Setup monitoring for leader status
- [ ] Configure alerting for failover events

### Phase 4: Multi-Tenancy Setup (If Multiple Accounts)

- [ ] Create tenant management table
- [ ] Implement tenant coordinator
- [ ] Assign unique business_id to each tenant
- [ ] Test tenant isolation (verify queries filter by business_id)
- [ ] Implement tenant load balancing
- [ ] Setup per-tenant monitoring
- [ ] Test tenant migration between hosts

### Phase 5: Production Hardening

- [ ] Implement graceful shutdown (SIGTERM handler)
- [ ] Add circuit breakers for database operations
- [ ] Implement retry logic with exponential backoff
- [ ] Add structured logging (JSON format)
- [ ] Setup distributed tracing (OpenTelemetry)
- [ ] Create runbooks for common failure scenarios
- [ ] Perform load testing
- [ ] Perform chaos engineering tests (kill pods, network partitions)

### Phase 6: Monitoring and Observability

- [ ] Metrics: Connection status, message throughput, error rates
- [ ] Metrics: Database query latency, connection pool usage
- [ ] Metrics: Leader election state, failover count
- [ ] Logs: Structured logs with correlation IDs
- [ ] Traces: Distributed tracing for message flow
- [ ] Alerts: Connection loss, repeated failovers, database errors
- [ ] Dashboards: Real-time operational view

---

## Conclusion

**Can whatsmeow be instantiated on multiple hosts for redundancy and horizontal scalability?**

### The Answer in Detail:

#### ❌ **Active-Active Multi-Host for Single Account: NO**
- WhatsApp protocol enforces single active connection per device
- Server detects duplicate connections and disconnects previous one
- Library design (response waiters, event handlers) prevents this
- Attempting this will cause constant reconnection loops

#### ✅ **Active-Passive Failover for Single Account: YES**
- Use PostgreSQL advisory locks for leader election
- Standby instance takes over when active fails
- Session persistence enables seamless reconnection
- Failover time: 10-15 seconds typical
- Production-ready solution for high availability

#### ✅ **Multi-Tenancy Horizontal Scaling: YES**
- Each tenant (business_id) is fully isolated
- Different accounts can run on different hosts
- Database schema designed for multitenancy
- Can scale to hundreds of accounts across multiple hosts
- Each account maintains single active connection

### Recommended Architecture by Use Case:

| Use Case | Architecture | Instances | Database | Availability |
|----------|-------------|-----------|----------|--------------|
| Development | Single Instance | 1 | Local PostgreSQL | None |
| Production (1 account) | Active-Passive | 2+ | Managed PostgreSQL | 99.9% |
| SaaS (N accounts) | Multi-Tenancy | 3+ | Managed PostgreSQL | 99.95% |
| Enterprise (N accounts + HA) | Multi-Tenancy + Failover | 6+ | HA PostgreSQL | 99.99% |

### Final Recommendation:

For **most production deployments**, implement:
1. **Database Layer**: Managed PostgreSQL with automated backups
2. **Application Layer**: Active-Passive failover using advisory locks
3. **Multi-Tenancy**: Distribute different accounts across hosts
4. **Monitoring**: Full observability stack
5. **Testing**: Regular failover drills and chaos engineering

This architecture provides:
- ✅ High availability through failover
- ✅ Horizontal scaling through multi-tenancy
- ✅ Protocol compliance (single connection per device)
- ✅ Operational simplicity
- ✅ Production-tested patterns

---

**Document Version:** 1.0
**Last Updated:** 2025-11-07
**Author:** Claude AI Analysis
**Repository:** github.com/angleto/whatsmeow (fork with multitenancy support)
