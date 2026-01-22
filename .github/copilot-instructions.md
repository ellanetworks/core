# Ella Core - AI Coding Agent Instructions

## Project Overview

Ella Core is a 5G private mobile network in a single Go binary. It consolidates traditional 5G network functions (AMF, SMF, UPF, AUSF) into one application with an embedded SQLite database and Next.js UI. The architecture prioritizes simplicity, reliability, and security for private deployments.

## Architecture

### Core Components (internal/)

- **amf/** - Access and Mobility Management Function (handles device registration/mobility via NGAP/NAS)
- **smf/** - Session Management Function (manages PDU sessions, assigns IPs)
- **upf/** - User Plane Function with eBPF/XDP data plane (packet forwarding between n3/n6 interfaces)
- **ausf/** - Authentication Server Function (5G AKA, Milenage algorithm)
- **api/** - HTTP REST API and embedded Next.js UI serving
- **db/** - SQLite database layer using [sqlair](https://github.com/canonical/sqlair) ORM
- **pfcp_dispatcher/** - PFCP protocol dispatcher routing between SMF and UPF handlers

**Key insight**: Network Functions communicate via in-process function calls (not HTTP), orchestrated in [pkg/runtime/runtime.go](pkg/runtime/runtime.go#L37-L156). The `pfcp_dispatcher.Dispatcher` routes PFCP messages internally.

### Database Patterns

All database operations use **sqlair** (not standard SQL). Statements are pre-compiled at init time and stored in the `Database` struct:

```go
// internal/db/db.go
type Database struct {
    listSubscribersStmt *sqlair.Statement
    createSubscriberStmt *sqlair.Statement
    // ... pre-compiled statements
}
```

Table definitions use sqlair syntax with Go struct tags (`db:"columnName"`). See [internal/db/subscribers.go](internal/db/subscribers.go) for examples.

### eBPF/XDP Data Plane

High-performance packet processing happens in kernel space:

- eBPF programs in `internal/upf/ebpf/xdp/` (C code)
- Generate Go bindings: `go generate ./...` (uses `bpf2go` from cilium/ebpf)
- Handles GTP-U encap/decap, QoS rate limiting, NAT between n3 (radio) and n6 (data network)
- Inspect BPF maps: `sudo bpftool map dump name pdrs_uplink`

**XDP attach modes** (config: `xdp.attach-mode`): `native` (best performance, driver-specific) or `generic` (fallback).

## Development Workflows

### Build Process

**Prerequisites**: Docker, Go 1.25, Node 24, clang/llvm, libbpf-dev

```bash
# Generate eBPF Go bindings (required before building)
go generate ./...

# Build backend binary
REVISION=`git rev-parse HEAD`
go build -ldflags "-X github.com/ellanetworks/core/version.GitCommit=${REVISION}" ./cmd/core/main.go

# Build frontend (embedded in binary via ui/embed.go)
npm install --prefix ui
npm run build --prefix ui
```

### Testing

```bash
# Unit tests
go test ./...

# Integration tests (requires Docker, uses UERANSIM simulator)
INTEGRATION=1 go test ./integration/... -v
```

Integration tests use Docker Compose in `integration/compose/` with real 5G simulators (UERANSIM, gNBSIM).

### Container Builds

Uses **Rockcraft** (not Dockerfile):

```bash
# Local registry setup (first time)
docker run -d -p 5000:5000 --name registry registry:2

# Build OCI image
rockcraft pack
sudo rockcraft.skopeo --insecure-policy copy oci-archive:ella-core_v1.1.0_amd64.rock docker-daemon:ella-core:latest
docker tag ella-core:latest localhost:5000/ella-core:latest
docker push localhost:5000/ella-core:latest
```

Config: [rockcraft.yaml](rockcraft.yaml) - single binary + ip/iptables tools, builds UI and backend together.

## Project Conventions

### Code Organization

- **client/** - Go SDK for REST API (used by tests/external tools, not by core itself)
- **internal/** - Core implementation (not importable externally)
- **pkg/runtime/** - Application startup/orchestration
- **ui/** - Next.js frontend (Material UI, TanStack Query, TypeScript)

### Testing Patterns

- Unit tests: `*_test.go` files alongside source, use table-driven tests
- Integration tests: `integration/*_test.go`, skip unless `INTEGRATION=1` env var set
- Client SDK tests use `fakeRequester` pattern - see [client/initialize_test.go](client/initialize_test.go)

### API Structure

REST API handlers in `internal/api/server/`. Pattern:

```go
type CreateSubscriberParams struct { /* request body */ }
type Subscriber struct { /* response */ }
func (s *Server) handleCreateSubscriber(w http.ResponseWriter, r *http.Request) {
    // Decode params, call db.*, encode response
}
```

**Middleware**: JWT auth (`internal/api/middleware/`), audit logging (`logger.AuditLog`), CORS.

### Configuration

Single YAML config file (`config/core.yaml`), parsed in `internal/config/`. Key sections:

- `interfaces`: n2 (control), n3 (user plane to radio), n6 (to data network), api (REST/UI)
- `db.path`: SQLite file location
- `xdp.attach-mode`: XDP performance mode
- `logging`: system (zap structured) + audit logs
- `telemetry`: OpenTelemetry OTLP traces (optional)

## Critical Details

### Deployment Context

- **Single binary** runs all NFs + API + UI server (see [cmd/core/main.go](cmd/core/main.go))
- Requires `--config` flag to start: `./core --config /path/to/core.yaml`
- Embedded Next.js assets served via `ui.FrontendFS` (embed.FS)

### 5G Standards Compliance

- Uses free5gc libraries for 3GPP message encoding (NGAP, NAS, PFCP)
- PLMN ID format: MCC + MNC (see [docs/explanation/obtaining_plmn_id.md](docs/explanation/obtaining_plmn_id.md))
- Subscriber auth: 5G AKA with Milenage algorithm (SQN, K, OPc parameters)

### Performance Considerations

- eBPF XDP path achieves >10 Gbps, <1.2ms latency (see [docs/reference/performance.md](docs/reference/performance.md))
- SQLite runs in WAL mode, prepared statements reduce parsing overhead
- Metrics exposed on `/metrics` (Prometheus format)

### Common Gotchas

- **eBPF changes require `go generate ./...`** - regenerates Go bindings from C code
- **Frontend changes need `npm run build --prefix ui`** - must rebuild before Go binary includes them
- **Integration tests modify Docker state** - always run `dockerClient.ComposeDown()` in cleanup
- **sqlair syntax differs from SQL** - uses `$Variable.field` placeholders, not `?` or `$1`

## Documentation

- Architecture: [docs/explanation/architecture.md](docs/explanation/architecture.md)
- API reference: `docs/reference/api/` (auto-generated)
- Config reference: [docs/reference/config_file.md](docs/reference/config_file.md)
- eBPF details: [docs/explanation/user_plane_packet_processing_with_ebpf.md](docs/explanation/user_plane_packet_processing_with_ebpf.md)
