# High Availability Failover Example

This example demonstrates how to deploy multiple instances of whatsmeow with active-passive failover for high availability.

## Architecture

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

## Features

- **Leader Election**: Uses PostgreSQL advisory locks for coordination
- **Automatic Failover**: Standby automatically takes over when active fails
- **Health Checks**: HTTP endpoints for Kubernetes/monitoring
- **Graceful Shutdown**: Proper cleanup on termination
- **Session Persistence**: Resumes from database after failover

## Prerequisites

1. PostgreSQL database
2. Existing WhatsApp device (paired using mdtest example)
3. Go 1.21 or higher

## Configuration

Environment variables:

- `DATABASE_URL` - PostgreSQL connection string (default: `postgres://localhost/whatsmeow?sslmode=disable`)
- `BUSINESS_ID` - Unique identifier for this account (default: `default`)
- `HTTP_PORT` - Port for health check server (default: `8080`)

## Running

### Single Instance (for testing)

```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/whatsmeow"
export BUSINESS_ID="myaccount"
go run main.go
```

### Two Instances (HA setup)

Terminal 1 (Active):
```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/whatsmeow"
export BUSINESS_ID="myaccount"
export HTTP_PORT="8080"
go run main.go
```

Terminal 2 (Standby):
```bash
export DATABASE_URL="postgres://user:pass@localhost:5432/whatsmeow"
export BUSINESS_ID="myaccount"
export HTTP_PORT="8081"
go run main.go
```

The first instance will acquire leadership and connect to WhatsApp. The second will wait in standby mode.

### Testing Failover

1. Kill the active instance (Ctrl+C or `kill` command)
2. Watch the standby instance acquire leadership and connect
3. Typical failover time: 5-15 seconds

## Health Endpoints

### `/health` - Overall Health

Returns health status of all components:

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
        "acquired": 2,
        "idle": 8,
        "max": 10
      }
    },
    "whatsapp": {
      "status": "healthy",
      "timestamp": "2025-01-07T10:30:00Z",
      "details": {
        "connected": true,
        "logged_in": true
      }
    },
    "leadership": {
      "status": "healthy",
      "timestamp": "2025-01-07T10:30:00Z",
      "details": {
        "is_leader": true
      }
    }
  }
}
```

### `/ready` - Readiness Check

Returns 200 if leader, 503 if standby:

```bash
curl http://localhost:8080/ready
```

## Kubernetes Deployment

Example Deployment manifest:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: whatsmeow-ha
spec:
  replicas: 2
  selector:
    matchLabels:
      app: whatsmeow-ha
  template:
    metadata:
      labels:
        app: whatsmeow-ha
    spec:
      containers:
      - name: whatsmeow
        image: your-registry/whatsmeow-ha:latest
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: whatsmeow-db
              key: url
        - name: BUSINESS_ID
          value: "default"
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
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
```

## Monitoring

Key metrics to monitor:

1. **Leadership status** - Which instance is active
2. **Connection status** - Is WhatsApp connected
3. **Failover count** - How often leadership changes
4. **Database latency** - Connection pool health
5. **Error rate** - Application errors

## Troubleshooting

### Both instances claim leadership

This should never happen with PostgreSQL advisory locks. If it does:
- Check database connectivity
- Verify both instances use the same `BUSINESS_ID`
- Check PostgreSQL logs for errors

### Failover takes too long

Typical failover: 10-15 seconds

If longer:
- Check network latency to database
- Verify standby instance is running
- Check standby logs for errors

### "Stream Replaced" errors

This means another instance connected with the same credentials. This is expected during failover but should not happen frequently.

If frequent:
- Check that only two instances are running
- Verify leader election is working properly
- Check for network partitions

## Production Recommendations

1. **Database**: Use managed PostgreSQL with replication
2. **Monitoring**: Set up alerts for failover events
3. **Testing**: Regularly test failover (chaos engineering)
4. **Logging**: Use structured logging with correlation IDs
5. **Tracing**: Add distributed tracing for debugging
6. **Backups**: Regular database backups
7. **Scaling**: Can run 3+ instances for higher availability

## See Also

- [Multi-Host Deployment Analysis](../../MULTI_HOST_DEPLOYMENT_ANALYSIS.md)
- [Multi-Tenant Example](../multi-tenant/)
- [whatsmeow Documentation](https://pkg.go.dev/go.mau.fi/whatsmeow)
