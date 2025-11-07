# whatsmeow
[![Go Reference](https://pkg.go.dev/badge/go.mau.fi/whatsmeow.svg)](https://pkg.go.dev/go.mau.fi/whatsmeow)

whatsmeow is a Go library for the WhatsApp web multidevice API.

**This fork adds enterprise-grade multi-host deployment support:**
- ✅ **High Availability** - Active-passive failover using PostgreSQL advisory locks
- ✅ **Multi-Tenancy** - Manage multiple WhatsApp accounts with complete isolation
- ✅ **Production Ready** - Health checks, monitoring, graceful shutdown
- ✅ **Kubernetes Native** - StatefulSets, health probes, and service discovery

See [DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md) for production deployment instructions.

## Discussion
Matrix room: [#whatsmeow:maunium.net](https://matrix.to/#/#whatsmeow:maunium.net)

For questions about the WhatsApp protocol (like how to send a specific type of
message), you can also use the [WhatsApp protocol Q&A] section on GitHub
discussions.

[WhatsApp protocol Q&A]: https://github.com/tulir/whatsmeow/discussions/categories/whatsapp-protocol-q-a

## Usage
The [godoc](https://pkg.go.dev/go.mau.fi/whatsmeow) includes docs for all methods and event types.
There's also a [simple example](https://pkg.go.dev/go.mau.fi/whatsmeow#example-package) at the top.

## Features
Most core features are already present:

* Sending messages to private chats and groups (both text and media)
* Receiving all messages
* Managing groups and receiving group change events
* Joining via invite messages, using and creating invite links
* Sending and receiving typing notifications
* Sending and receiving delivery and read receipts
* Reading and writing app state (contact list, chat pin/mute status, etc)
* Sending and handling retry receipts if message decryption fails
* Sending status messages (experimental, may not work for large contact lists)

Things that are not yet implemented:

* Sending broadcast list messages (this is not supported on WhatsApp web either)
* Calls

## Multi-Host Deployment (This Fork)

This fork adds comprehensive support for production multi-host deployments:

### Packages

- **ha/** - High availability with PostgreSQL advisory lock-based leader election
- **coordinator/** - Multi-tenant coordination for managing multiple accounts
- **health/** - Health checking and monitoring with HTTP endpoints

### Examples

- **examples/ha-failover/** - Active-passive failover example
  - Automatic leader election
  - Health and readiness checks
  - Graceful failover (10-15s typical)

- **examples/multi-tenant/** - Multi-tenancy example
  - REST API for tenant management
  - Dynamic tenant lifecycle
  - Per-tenant health monitoring

### Documentation

- [MULTI_HOST_DEPLOYMENT_ANALYSIS.md](MULTI_HOST_DEPLOYMENT_ANALYSIS.md) - Comprehensive analysis of multi-host feasibility
- [DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md) - Production deployment guide
- [MULTITENANCY_DEPLOYMENT_GUIDE.md](MULTITENANCY_DEPLOYMENT_GUIDE.md) - Multi-tenancy guide
- [SECURITY_ANALYSIS_MULTITENANCY.md](SECURITY_ANALYSIS_MULTITENANCY.md) - Security analysis

### Quick Start - High Availability

```bash
# Terminal 1 (Active)
DATABASE_URL="postgres://localhost/whatsmeow" \
BUSINESS_ID="myaccount" \
go run examples/ha-failover/main.go

# Terminal 2 (Standby)
DATABASE_URL="postgres://localhost/whatsmeow" \
BUSINESS_ID="myaccount" \
HTTP_PORT="8081" \
go run examples/ha-failover/main.go
```

### Quick Start - Multi-Tenancy

```bash
# Start coordinator
DATABASE_URL="postgres://localhost/whatsmeow" \
go run examples/multi-tenant/main.go

# Manage tenants via API
curl -X POST http://localhost:8080/api/tenants \
  -H "Content-Type: application/json" \
  -d '{"business_id": "tenant1"}'
```

See [DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md) for complete setup instructions.
