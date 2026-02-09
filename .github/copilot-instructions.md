# Ella Core - AI Coding Agent Instructions

## Project Overview

5G private mobile network in a single Go binary. Consolidates AMF, SMF, UPF, AUSF network functions with an embedded SQLite database and React/Vite UI. All NFs communicate via **in-process function calls** (not HTTP), orchestrated in `pkg/runtime/runtime.go`.

## Build & Test

```bash
go generate ./...                    # Required: regenerate eBPF Go bindings from C code
npm install --prefix ui && npm run build --prefix ui  # Build frontend (embedded via ui/embed.go)
REVISION=$(git rev-parse HEAD)
go build -ldflags "-X github.com/ellanetworks/core/version.GitCommit=${REVISION}" ./cmd/core/main.go

go test ./...                                   # Unit tests
INTEGRATION=1 go test ./integration/... -v      # Integration tests (requires Docker)
```

**Gotchas**: eBPF C changes require `go generate ./...` before building. Frontend changes require `npm run build --prefix ui` before the Go binary includes them.

## Architecture

| Directory | Purpose |
|-----------|---------|
| `internal/amf/` | Access & Mobility Management (NGAP/NAS) |
| `internal/smf/` | Session Management (PDU sessions, IP assignment) |
| `internal/upf/` | User Plane with eBPF/XDP data plane |
| `internal/ausf/` | Authentication (5G AKA, Milenage) |
| `internal/api/` | REST API + embedded UI serving |
| `internal/db/` | SQLite via [sqlair](https://github.com/canonical/sqlair) ORM |
| `internal/pfcp_dispatcher/` | In-process PFCP message routing between SMF↔UPF |
| `internal/kernel/` | Linux networking (routes, nftables, ARP) via netlink |
| `internal/models/` | 3GPP protocol structs shared across NFs (not DB models) |
| `client/` | Go SDK for REST API (used by tests/external tools) |
| `ui/` | React 19 + Vite + MUI 7 + TanStack Query 5 frontend |
| `etsi/` | 3GPP identity types (GUTI, TMSI allocation) |

## Database Layer (`internal/db/`)

Uses **sqlair** (not standard SQL). Statements are pre-compiled at init time in `PrepareStatements()` and stored as `*sqlair.Statement` fields on the `Database` struct.

**Sqlair syntax** — uses `$Struct.field` for params, `&Struct.*` for output:
```go
// SELECT with output binding and WHERE parameter
"SELECT &Subscriber.* FROM subscribers WHERE imsi==$Subscriber.imsi"
// INSERT with struct field binding
"INSERT INTO subscribers (imsi, policyID) VALUES ($Subscriber.imsi, $Subscriber.policyID)"
// Pagination via ListArgs helper
"SELECT &Subscriber.* FROM subscribers LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
// Count via NumItems helper
"SELECT COUNT(*) AS &NumItems.count FROM subscribers"
```

**Struct tags** use `db:"columnName"` (camelCase matching SQLite columns). Nullable fields use pointers (`*string`).

**Adding a new entity**: (1) Create `internal/db/<entity>.go` with DDL const, statement consts, Go struct with `db:` tags, CRUD methods. (2) Add `*sqlair.Statement` fields to `Database` struct in `db.go`. (3) Add `sqlair.Prepare()` calls in `PrepareStatements()`. (4) Add `createTable()` call in `Initialize()`. Each CRUD method starts an OpenTelemetry span and records Prometheus metrics.

**Sentinel errors**: `ErrNotFound`, `ErrAlreadyExists`. Use `isUniqueNameError()` to detect SQLite constraint violations.

**Helper structs** in `internal/db/helpers.go`: `ListArgs` (pagination), `NumItems` (counts), `cutoffArgs`/`cutoffDaysArgs` (retention).

## API Layer (`internal/api/server/`)

**Handlers are closure factories** returning `http.Handler`, not struct methods:
```go
func ListSubscribers(dbInstance *db.Database) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // parse params → call db → writeResponse(w, result, http.StatusOK, logger.APILog)
    })
}
```

**Route registration** in `routes.go` uses Go 1.22+ method-pattern syntax with 3-layer middleware:
```go
mux.HandleFunc("GET /api/v1/subscribers",
    Authenticate(jwtSecret, dbInstance,
        Authorize(PermListSubscribers,
            ListSubscribers(dbInstance))).ServeHTTP)
```

**Response envelope**: Success → `{"result": <payload>}`, Error → `{"error": "message"}`, Mutation → `{"result": {"message": "..."}}`. Use `writeResponse()` / `writeError()` helpers.

**Auth**: JWT tokens (session) + API tokens (prefixed `ella_`). RBAC with 3 roles: admin (1), operator (2), viewer (3). Permissions are strings like `subscriber:list`.

**Server**: HTTP/2 always enabled (TLS native or h2c for cleartext). JWT secret generated randomly at startup.

## Frontend (`ui/src/`)

React 19 + TypeScript + Vite + MUI 7 + TanStack Query 5. Embedded in Go binary via `//go:embed all:dist` in `ui/embed.go`.

**API calls**: `apiGet<T>()` / `apiCall()` wrappers in `queries/common.ts`. Each resource has a query file exporting types + async functions. Pages use `useQuery` with 5-second polling:
```tsx
const { data } = useQuery({
    queryKey: ["subscribers", page], queryFn: () => listSubscribers(token, page, perPage),
    enabled: authReady && !!accessToken, refetchInterval: 5000,
});
```

**Auth**: `AuthContext` manages JWT with automatic silent refresh (schedules 120s before expiry).

## Client SDK (`client/`)

Uses `Requester` interface for testability. Tests inject `fakeRequester` to avoid HTTP:
```go
type fakeRequester struct {
    response *client.RequestResponse
    err      error
    lastOpts *client.RequestOptions  // captures request for assertions
}
```

## Testing Patterns

- **DB tests**: Black-box (`package db_test`), real SQLite via `t.TempDir()`, sequential CRUD assertions
- **API tests**: `setupTestServer()` creates real DB + HTTP test server with `FakeKernel{}`/`FakeUPF{}` stubs
- **Client tests**: `fakeRequester` pattern — pre-set response JSON, call method, assert
- **Integration**: `INTEGRATION=1` env var guard, Docker Compose with UERANSIM/gNBSIM, Go client SDK to interact

## Logging

Zap structured logging. Component loggers: `logger.AMFLog`, `logger.SMFLog`, `logger.UPFLog`, `logger.APILog`, etc. **Audit logging** dual-writes to zap + database via `DBWriter` interface (breaks import cycles between `logger` and `db`).

## Configuration

Single YAML file parsed in `internal/config/`. Key sections: `interfaces` (n2/n3/n6/api), `db.path`, `xdp.attach-mode`, `logging`, `telemetry`. Validation is imperative (not struct tags). Network interface resolution supports either name or IP. Testable via swappable `var` function pointers.

## Background Jobs

- `internal/sessions/`: Cleans expired PDU sessions every 30 seconds
- `internal/jobs/`: Data retention worker runs every 24 hours, enforcing configurable retention for audit logs, radio events, and subscriber usage

## Key Dependencies

- **sqlair** (Canonical) — SQL ORM with struct binding
- **cilium/ebpf** — eBPF program loading + `bpf2go` code generation
- **free5gc** libraries — 3GPP protocol encoding (NGAP, NAS, PFCP)
- **go-pfcp** — PFCP message types
- **netlink/nftables** — Linux kernel networking
- **zap** — Structured logging
- **Prometheus client** — Metrics (prefix: `app_`)
