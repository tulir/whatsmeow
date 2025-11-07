# whatsmeow Multi-Host Deployment Guide

This guide provides step-by-step instructions for deploying whatsmeow in production with high availability and multi-tenancy support.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Architecture Overview](#architecture-overview)
3. [Prerequisites](#prerequisites)
4. [Database Setup](#database-setup)
5. [Deployment Patterns](#deployment-patterns)
6. [Configuration](#configuration)
7. [Monitoring](#monitoring)
8. [Production Checklist](#production-checklist)
9. [Troubleshooting](#troubleshooting)

## Quick Start

### For Single Account with HA

```bash
# 1. Setup database
psql -U postgres -c "CREATE DATABASE whatsmeow;"
psql -U postgres whatsmeow < store/sqlstore/upgrades/00-latest-schema.sql

# 2. Pair device
cd mdtest
go run main.go --business-id myaccount

# 3. Run two instances
# Terminal 1 (Active)
DATABASE_URL="postgres://localhost/whatsmeow" \
BUSINESS_ID="myaccount" \
HTTP_PORT="8080" \
go run examples/ha-failover/main.go

# Terminal 2 (Standby)
DATABASE_URL="postgres://localhost/whatsmeow" \
BUSINESS_ID="myaccount" \
HTTP_PORT="8081" \
go run examples/ha-failover/main.go
```

### For Multiple Accounts

```bash
# 1. Setup database (same as above)

# 2. Pair devices
cd mdtest
go run main.go --business-id tenant1
go run main.go --business-id tenant2

# 3. Run coordinator
DATABASE_URL="postgres://localhost/whatsmeow" \
go run examples/multi-tenant/main.go

# 4. Start tenants via API
curl -X POST http://localhost:8080/api/tenants \
  -H "Content-Type: application/json" \
  -d '{"business_id": "tenant1"}'

curl -X POST http://localhost:8080/api/tenants \
  -H "Content-Type: application/json" \
  -d '{"business_id": "tenant2"}'
```

## Architecture Overview

### Available Components

1. **ha** - High availability package with leader election
2. **coordinator** - Multi-tenant management
3. **health** - Health checking and monitoring
4. **examples/ha-failover** - Active-passive failover example
5. **examples/multi-tenant** - Multi-tenancy example

### Deployment Patterns

| Pattern | Use Case | Instances | HA | Scalability |
|---------|----------|-----------|-----|-------------|
| Single Instance | Development, testing | 1 | ❌ | ❌ |
| Active-Passive | Production (1 account) | 2+ | ✅ | ❌ |
| Multi-Tenant | Multiple accounts | 1+ | ❌ | ✅ |
| Multi-Tenant + HA | Production (N accounts) | 2+ per tenant | ✅ | ✅ |

## Prerequisites

### Required

- **Go**: 1.21 or higher
- **PostgreSQL**: 12 or higher
- **WhatsApp**: Paired device(s)

### Recommended for Production

- **Load Balancer**: Nginx, HAProxy, or cloud LB
- **Monitoring**: Prometheus + Grafana
- **Logging**: ELK stack or cloud logging
- **Orchestration**: Kubernetes or Docker Swarm
- **Secrets Management**: HashiCorp Vault or cloud secrets

## Database Setup

### PostgreSQL Installation

**Ubuntu/Debian:**
```bash
sudo apt-get update
sudo apt-get install postgresql postgresql-contrib
```

**macOS:**
```bash
brew install postgresql@15
brew services start postgresql@15
```

**Docker:**
```bash
docker run -d \
  --name whatsmeow-db \
  -e POSTGRES_PASSWORD=secret \
  -e POSTGRES_DB=whatsmeow \
  -p 5432:5432 \
  -v postgres_data:/var/lib/postgresql/data \
  postgres:15
```

### Schema Setup

```bash
# Create database
createdb whatsmeow

# Apply schema
psql whatsmeow < store/sqlstore/upgrades/00-latest-schema.sql

# Verify tables
psql whatsmeow -c "\dt whatsmeow_*"
```

Should show 15 tables:
- whatsmeow_device
- whatsmeow_identity_keys
- whatsmeow_sessions
- whatsmeow_pre_keys
- whatsmeow_sender_keys
- whatsmeow_app_state_sync_keys
- whatsmeow_app_state_version
- whatsmeow_app_state_mutation_macs
- whatsmeow_contacts
- whatsmeow_redacted_phones
- whatsmeow_chat_settings
- whatsmeow_message_secrets
- whatsmeow_privacy_tokens
- whatsmeow_lid_map
- whatsmeow_event_buffer

### Connection Pooling

Configure based on deployment:

```
postgres://user:pass@host:5432/whatsmeow?
  pool_max_conns=50&
  pool_min_conns=10&
  pool_max_conn_lifetime=1h&
  pool_max_conn_idle_time=30m&
  pool_health_check_period=1m
```

**Sizing guidelines:**
- Single instance: 10-20 connections
- Active-Passive (2 instances): 20-30 connections
- Multi-tenant: `num_tenants * 2 + 10` connections

### High Availability Setup

**Managed Services** (Recommended):
- AWS RDS PostgreSQL with Multi-AZ
- Google Cloud SQL PostgreSQL with HA
- Azure Database for PostgreSQL with read replicas
- DigitalOcean Managed PostgreSQL

**Self-Hosted:**
- PostgreSQL with streaming replication
- Patroni for automatic failover
- PgBouncer for connection pooling

## Deployment Patterns

### Pattern 1: Active-Passive Failover

**Use Case:** Single account requiring high availability

**Architecture:**
```
┌─────────┐         ┌─────────┐
│ Active  │────────►│WhatsApp │
└────┬────┘         └─────────┘
     │
     │ PostgreSQL
     │ Advisory Lock
     │
┌────▼────┐
│ Standby │ (waiting for lock)
└─────────┘
```

**Implementation:**

1. **Build application:**
```bash
cd examples/ha-failover
go build -o whatsmeow-ha main.go
```

2. **Create systemd service:**

`/etc/systemd/system/whatsmeow-ha.service`:
```ini
[Unit]
Description=WhatsApp HA Instance
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=whatsmeow
Group=whatsmeow
WorkingDirectory=/opt/whatsmeow
Environment="DATABASE_URL=postgres://user:pass@localhost/whatsmeow"
Environment="BUSINESS_ID=production"
Environment="HTTP_PORT=8080"
ExecStart=/opt/whatsmeow/whatsmeow-ha
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

3. **Deploy on multiple hosts:**

```bash
# Host A
sudo systemctl enable whatsmeow-ha
sudo systemctl start whatsmeow-ha

# Host B
sudo systemctl enable whatsmeow-ha
sudo systemctl start whatsmeow-ha
```

4. **Verify:**

```bash
# Check leader status
curl http://host-a:8080/ready  # Should return "Ready - Leader"
curl http://host-b:8080/ready  # Should return "Not Ready - Standby"

# Check health
curl http://host-a:8080/health | jq .
```

**Load Balancer Configuration (Nginx):**

```nginx
upstream whatsmeow_ha {
    # Health check via readiness endpoint
    server host-a:8080 max_fails=3 fail_timeout=30s;
    server host-b:8080 max_fails=3 fail_timeout=30s backup;
}

server {
    listen 80;
    server_name whatsmeow.example.com;

    location /health {
        proxy_pass http://whatsmeow_ha;
        proxy_set_header Host $host;
    }

    location /ready {
        proxy_pass http://whatsmeow_ha;
        proxy_set_header Host $host;
    }
}
```

### Pattern 2: Multi-Tenancy

**Use Case:** Multiple accounts, horizontal scaling

**Architecture:**
```
       ┌─────────────┐
       │   API/LB    │
       └──────┬──────┘
              │
    ┌─────────┼─────────┐
    │         │         │
┌───▼───┐ ┌──▼────┐ ┌──▼────┐
│Host A │ │Host B │ │Host C │
│T1,T2  │ │T3,T4  │ │T5,T6  │
└───────┘ └───────┘ └───────┘
```

**Implementation:**

1. **Build application:**
```bash
cd examples/multi-tenant
go build -o whatsmeow-mt main.go
```

2. **Create systemd service:**

`/etc/systemd/system/whatsmeow-mt.service`:
```ini
[Unit]
Description=WhatsApp Multi-Tenant Instance
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=whatsmeow
Group=whatsmeow
WorkingDirectory=/opt/whatsmeow
Environment="DATABASE_URL=postgres://user:pass@db-host/whatsmeow"
Environment="HTTP_PORT=8080"
ExecStart=/opt/whatsmeow/whatsmeow-mt
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

3. **Deploy:**

```bash
sudo systemctl enable whatsmeow-mt
sudo systemctl start whatsmeow-mt
```

4. **Manage tenants via API:**

```bash
# Start tenants
curl -X POST http://localhost:8080/api/tenants \
  -d '{"business_id": "customer1"}'

# List tenants
curl http://localhost:8080/api/tenants | jq .

# Stop tenant
curl -X DELETE http://localhost:8080/api/tenants/customer1
```

### Pattern 3: Multi-Tenancy + HA

**Use Case:** Production with multiple accounts and HA per account

**Architecture:**
```
Tenant 1: Active (Host A) + Standby (Host B)
Tenant 2: Active (Host C) + Standby (Host D)
Tenant 3: Active (Host A) + Standby (Host C)
...
```

**Implementation:**

Use HA failover example with unique BUSINESS_ID per tenant. Deploy multiple HA pairs, each managing one tenant.

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | - | PostgreSQL connection string |
| `BUSINESS_ID` | HA only | `default` | Unique tenant identifier |
| `HTTP_PORT` | No | `8080` | Health check server port |

### Database URL Format

```
postgres://[user[:password]@][host][:port][/dbname][?param1=value1&...]
```

**Common parameters:**
- `sslmode` - SSL mode (disable, require, verify-full)
- `pool_max_conns` - Maximum connections
- `pool_min_conns` - Minimum connections
- `connect_timeout` - Connection timeout in seconds

**Examples:**

```bash
# Local development
DATABASE_URL="postgres://localhost/whatsmeow?sslmode=disable"

# Production with SSL
DATABASE_URL="postgres://user:pass@db.example.com:5432/whatsmeow?sslmode=verify-full&pool_max_conns=50"

# AWS RDS
DATABASE_URL="postgres://user:pass@mydb.abc123.us-east-1.rds.amazonaws.com:5432/whatsmeow?sslmode=require"

# Google Cloud SQL
DATABASE_URL="postgres://user:pass@/whatsmeow?host=/cloudsql/project:region:instance"
```

### Secrets Management

**Environment Files:**
```bash
# /etc/whatsmeow/env
DATABASE_URL="postgres://..."
BUSINESS_ID="production"
```

**Docker Secrets:**
```yaml
secrets:
  db_url:
    external: true

services:
  whatsmeow:
    secrets:
      - db_url
    environment:
      DATABASE_URL_FILE: /run/secrets/db_url
```

**Kubernetes Secrets:**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: whatsmeow-db
type: Opaque
stringData:
  url: postgres://user:pass@host/db
---
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: whatsmeow
    env:
    - name: DATABASE_URL
      valueFrom:
        secretKeyRef:
          name: whatsmeow-db
          key: url
```

## Monitoring

### Health Endpoints

**`/health`** - Component health status
- Returns 200 if all components healthy
- Returns 503 if any component unhealthy
- JSON response with detailed status

**`/ready`** - Readiness for traffic
- Returns 200 if ready to serve
- Returns 503 if not ready
- Used for load balancer health checks

### Prometheus Metrics

Example custom metrics to export:

```go
import "github.com/prometheus/client_golang/prometheus"

var (
    tenantsTotal = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "whatsmeow_tenants_total",
            Help: "Total number of tenants",
        },
    )

    tenantsConnected = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "whatsmeow_tenants_connected",
            Help: "Number of connected tenants",
        },
    )

    messagesReceived = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "whatsmeow_messages_received_total",
            Help: "Total messages received",
        },
        []string{"business_id"},
    )

    connectionDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "whatsmeow_connection_duration_seconds",
            Help: "Connection duration",
        },
        []string{"business_id"},
    )
)

func init() {
    prometheus.MustRegister(tenantsTotal)
    prometheus.MustRegister(tenantsConnected)
    prometheus.MustRegister(messagesReceived)
    prometheus.MustRegister(connectionDuration)
}
```

### Logging

Use structured logging:

```go
log.Infof("Event occurred",
    "business_id", businessID,
    "event_type", eventType,
    "timestamp", time.Now(),
    "details", details,
)
```

Ship logs to:
- ELK Stack (Elasticsearch, Logstash, Kibana)
- Loki + Grafana
- Cloud logging (CloudWatch, Stackdriver, etc.)

### Alerting Rules

Example Prometheus alerts:

```yaml
groups:
- name: whatsmeow
  rules:
  - alert: WhatsAppDisconnected
    expr: whatsmeow_tenants_connected < whatsmeow_tenants_total
    for: 5m
    annotations:
      summary: "Tenant disconnected"

  - alert: HighFailoverRate
    expr: rate(whatsmeow_failover_total[5m]) > 0.1
    for: 5m
    annotations:
      summary: "High failover rate detected"

  - alert: DatabaseLatencyHigh
    expr: whatsmeow_db_query_duration_seconds > 0.1
    for: 2m
    annotations:
      summary: "Database queries are slow"
```

## Production Checklist

### Security

- [ ] Use SSL for database connections
- [ ] Rotate database credentials regularly
- [ ] Use secrets management (Vault, cloud secrets)
- [ ] Enable PostgreSQL authentication
- [ ] Use network policies/firewalls
- [ ] Implement API authentication for multi-tenant API
- [ ] Regular security audits

### Reliability

- [ ] Deploy at least 2 instances for HA
- [ ] Use managed database with automated backups
- [ ] Configure automatic restarts (systemd, k8s)
- [ ] Test failover regularly
- [ ] Monitor connection status
- [ ] Set up alerting
- [ ] Document runbooks

### Performance

- [ ] Size database connection pool appropriately
- [ ] Monitor memory usage per tenant
- [ ] Set resource limits (CPU, memory)
- [ ] Use connection pooling (PgBouncer)
- [ ] Monitor query performance
- [ ] Optimize for tenant count

### Observability

- [ ] Export Prometheus metrics
- [ ] Configure structured logging
- [ ] Set up dashboards (Grafana)
- [ ] Enable distributed tracing
- [ ] Monitor health endpoints
- [ ] Track SLIs/SLOs

### Operations

- [ ] Automate deployments
- [ ] Version control configuration
- [ ] Document deployment process
- [ ] Create runbooks for common issues
- [ ] Plan capacity based on growth
- [ ] Regular backups and restore testing
- [ ] Disaster recovery plan

## Troubleshooting

### Common Issues

#### "Failed to acquire lock"

**Symptom:** Standby instance logs "Failed to acquire lock"

**Cause:** Active instance holds the advisory lock

**Solution:** This is normal. Standby waits until active releases lock.

#### "Stream Replaced"

**Symptom:** Active instance receives StreamReplaced event

**Cause:** Another instance connected with same device credentials

**Solution:**
- Check that BUSINESS_ID is same on all instances
- Verify only expected instances are running
- Check for manual connections (e.g., using mdtest)

#### High Memory Usage

**Symptom:** Memory usage grows over time

**Cause:** Each tenant consumes ~50-100MB

**Solution:**
- Distribute tenants across more hosts
- Increase container memory limits
- Monitor per-tenant memory usage

#### Database Connection Pool Exhausted

**Symptom:** "sorry, too many clients already"

**Cause:** Too many tenants for connection pool size

**Solution:**
```bash
# Increase pool size
DATABASE_URL="postgres://...?pool_max_conns=100"

# Or use connection pooler
# Install PgBouncer
sudo apt-get install pgbouncer

# Configure PgBouncer
# /etc/pgbouncer/pgbouncer.ini
[databases]
whatsmeow = host=localhost dbname=whatsmeow

[pgbouncer]
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 50
```

#### Slow Failover

**Symptom:** Failover takes > 30 seconds

**Cause:** Network latency, database slow, or check interval too long

**Solution:**
- Reduce health check interval
- Optimize database connection
- Check network latency between hosts and database

### Debug Mode

Enable debug logging:

```go
log = waLog.Stdout("Main", "DEBUG", true)
```

### Database Debugging

```sql
-- Check advisory locks
SELECT * FROM pg_locks WHERE locktype = 'advisory';

-- Check active connections
SELECT * FROM pg_stat_activity;

-- Check tenant devices
SELECT business_id, jid FROM whatsmeow_device;

-- Check session count per tenant
SELECT business_id, our_jid, COUNT(*)
FROM whatsmeow_sessions
GROUP BY business_id, our_jid;
```

## Next Steps

1. Review [Multi-Host Deployment Analysis](MULTI_HOST_DEPLOYMENT_ANALYSIS.md)
2. Try [HA Failover Example](examples/ha-failover/)
3. Try [Multi-Tenant Example](examples/multi-tenant/)
4. Read [Multitenancy Guide](MULTITENANCY_DEPLOYMENT_GUIDE.md)
5. Review [Security Analysis](SECURITY_ANALYSIS_MULTITENANCY.md)

## Support

- **Documentation**: https://pkg.go.dev/go.mau.fi/whatsmeow
- **Issues**: https://github.com/tulir/whatsmeow/issues
- **Matrix**: [#whatsmeow:maunium.net](https://matrix.to/#/#whatsmeow:maunium.net)

---

**Note:** This is a fork with multitenancy and HA support. For the original library, visit https://github.com/tulir/whatsmeow
