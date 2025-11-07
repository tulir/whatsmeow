# Multi-Tenant Example

This example demonstrates how to manage multiple WhatsApp accounts (tenants) from a single application instance with proper isolation and resource management.

## Architecture

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

## Features

- **Dynamic Tenant Management**: Start/stop tenants via REST API
- **Complete Isolation**: Each tenant has separate database partition
- **Auto-Reconnection**: Automatic reconnection on connection loss
- **Health Monitoring**: Per-tenant health tracking
- **Event Routing**: Events properly routed to tenant handlers
- **Resource Management**: Efficient connection pooling

## Prerequisites

1. PostgreSQL database
2. Paired WhatsApp devices (one per tenant, use mdtest example)
3. Go 1.21 or higher

## Configuration

Environment variables:

- `DATABASE_URL` - PostgreSQL connection string (default: `postgres://localhost/whatsmeow?sslmode=disable`)
- `HTTP_PORT` - Port for API/health server (default: `8080`)

## Running

```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/whatsmeow"
go run main.go
```

## API Reference

### Start a Tenant

Start a WhatsApp client for a specific business ID.

```bash
curl -X POST http://localhost:8080/api/tenants \
  -H "Content-Type: application/json" \
  -d '{"business_id": "tenant1"}'
```

Response (201 Created):
```json
{
  "status": "started",
  "business_id": "tenant1"
}
```

### List All Tenants

Get status of all managed tenants.

```bash
curl http://localhost:8080/api/tenants
```

Response (200 OK):
```json
[
  {
    "business_id": "tenant1",
    "status": "connected",
    "started_at": "2025-01-07T10:00:00Z",
    "updated_at": "2025-01-07T10:00:05Z"
  },
  {
    "business_id": "tenant2",
    "status": "disconnected",
    "started_at": "2025-01-07T10:05:00Z",
    "updated_at": "2025-01-07T10:15:30Z",
    "error": "connection timeout"
  }
]
```

### Get Tenant Status

Get detailed status for a specific tenant.

```bash
curl http://localhost:8080/api/tenants/tenant1
```

Response (200 OK):
```json
{
  "business_id": "tenant1",
  "status": "connected",
  "started_at": "2025-01-07T10:00:00Z",
  "updated_at": "2025-01-07T10:00:05Z"
}
```

### Stop a Tenant

Stop a tenant's WhatsApp client.

```bash
curl -X DELETE http://localhost:8080/api/tenants/tenant1
```

Response (200 OK):
```json
{
  "status": "stopped",
  "business_id": "tenant1"
}
```

## Health Endpoints

### `/health` - Overall Health

Returns health status of the application:

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy",
  "timestamp": "2025-01-07T10:30:00Z",
  "components": {
    "database": {
      "status": "healthy",
      "timestamp": "2025-01-07T10:30:00Z",
      "details": {
        "latency": "2ms",
        "acquired": 5,
        "idle": 15,
        "max": 20
      }
    },
    "liveness": {
      "status": "healthy",
      "timestamp": "2025-01-07T10:30:00Z"
    }
  }
}
```

### `/ready` - Readiness Check

Returns 200 if at least one tenant is connected, 503 otherwise:

```bash
curl http://localhost:8080/ready
```

Response (200 OK):
```json
{
  "ready": true,
  "total": 5,
  "connected": 3
}
```

## Tenant Lifecycle

### Creating a New Tenant

1. Pair a device using the mdtest example:
   ```bash
   cd mdtest
   go run main.go --business-id tenant-new
   ```

2. Scan the QR code with WhatsApp

3. Start the tenant via API:
   ```bash
   curl -X POST http://localhost:8080/api/tenants \
     -H "Content-Type: application/json" \
     -d '{"business_id": "tenant-new"}'
   ```

### Tenant States

- `stopped` - Not running
- `starting` - Initializing connection
- `connected` - Connected to WhatsApp
- `disconnected` - Connection lost, will retry
- `error` - Fatal error occurred

### Auto-Reconnection

The coordinator automatically monitors tenant health and attempts reconnection:

- Health check every 10 seconds (configurable)
- Immediate reconnection attempt on disconnect
- Exponential backoff on failure
- No retry limit (keeps trying)

## Scaling

### Single Instance

Run all tenants on one host:

```bash
# Start tenants
curl -X POST .../api/tenants -d '{"business_id": "tenant1"}'
curl -X POST .../api/tenants -d '{"business_id": "tenant2"}'
curl -X POST .../api/tenants -d '{"business_id": "tenant3"}'
```

### Multiple Instances

Distribute tenants across hosts using consistent hashing or manual assignment:

**Host A:**
```bash
# Tenants 1-5
for i in {1..5}; do
  curl -X POST http://host-a:8080/api/tenants \
    -d "{\"business_id\": \"tenant$i\"}"
done
```

**Host B:**
```bash
# Tenants 6-10
for i in {6..10}; do
  curl -X POST http://host-b:8080/api/tenants \
    -d "{\"business_id\": \"tenant$i\"}"
done
```

### High Availability per Tenant

Combine with HA failover for critical tenants:

- Deploy HA failover setup per tenant
- Use different business IDs for each tenant
- Each tenant gets active-passive pair

## Kubernetes Deployment

Example StatefulSet for tenant management:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: whatsmeow-multitenant
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
        image: your-registry/whatsmeow-multitenant:latest
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: whatsmeow-db
              key: url
        - name: HTTP_PORT
          value: "8080"
        ports:
        - containerPort: 8080
          name: http
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
        resources:
          requests:
            cpu: 200m
            memory: 256Mi
          limits:
            cpu: 2000m
            memory: 2Gi
---
apiVersion: v1
kind: Service
metadata:
  name: whatsmeow
spec:
  clusterIP: None  # Headless service for StatefulSet
  selector:
    app: whatsmeow
  ports:
  - port: 8080
    name: http
```

## Monitoring

Key metrics to monitor:

1. **Tenant count** - Total managed tenants
2. **Connected count** - Successfully connected tenants
3. **Error rate** - Tenant errors per minute
4. **Reconnection rate** - How often tenants reconnect
5. **Database connections** - Pool usage
6. **Memory usage** - Per-tenant memory consumption
7. **Message throughput** - Messages per tenant

Example Prometheus metrics:

```
whatsmeow_tenants_total 10
whatsmeow_tenants_connected 8
whatsmeow_tenants_disconnected 2
whatsmeow_tenant_messages_total{business_id="tenant1"} 1523
whatsmeow_tenant_errors_total{business_id="tenant2"} 5
```

## Troubleshooting

### Tenant won't connect

1. Check database for device:
   ```sql
   SELECT * FROM whatsmeow_device WHERE business_id = 'tenant1';
   ```

2. Check logs for errors:
   ```
   [Coordinator] [tenant1] Failed to connect: ...
   ```

3. Verify device is still paired (check in WhatsApp)

### High memory usage

Each tenant consumes ~50-100MB. For many tenants:

- Increase container memory limits
- Distribute across more hosts
- Monitor and set alerts

### Tenant keeps disconnecting

Check for:
- Network issues
- Another instance connecting with same credentials
- Device unlinked in WhatsApp
- Server-side rate limiting

### Database connection pool exhausted

Increase pool size in database URL:
```
postgres://...?pool_max_conns=50
```

Rule of thumb: `max_conns >= num_tenants * 2 + 10`

## Production Recommendations

1. **Database**: Use connection pooling, adjust based on tenant count
2. **Resources**: Allocate ~100MB per tenant + base overhead
3. **Monitoring**: Track per-tenant metrics
4. **Alerting**: Alert on tenant disconnections
5. **Backups**: Regular database backups
6. **Rate Limiting**: Implement API rate limiting
7. **Authentication**: Add API authentication for production
8. **Load Balancing**: Use consistent hashing for tenant assignment
9. **Graceful Shutdown**: Handle SIGTERM properly (already implemented)
10. **Testing**: Test with realistic tenant counts

## Example Usage Script

```bash
#!/bin/bash
# Start 10 tenants

BASE_URL="http://localhost:8080"

for i in {1..10}; do
  echo "Starting tenant-$i..."
  curl -X POST $BASE_URL/api/tenants \
    -H "Content-Type: application/json" \
    -d "{\"business_id\": \"tenant-$i\"}" \
    -s | jq .
  sleep 2
done

echo "Listing all tenants..."
curl $BASE_URL/api/tenants -s | jq .
```

## See Also

- [Multi-Host Deployment Analysis](../../MULTI_HOST_DEPLOYMENT_ANALYSIS.md)
- [HA Failover Example](../ha-failover/)
- [whatsmeow Documentation](https://pkg.go.dev/go.mau.fi/whatsmeow)
- [Multitenancy Guide](../../MULTITENANCY_DEPLOYMENT_GUIDE.md)
