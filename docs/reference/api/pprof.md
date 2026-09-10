---
description: RESTful API reference for profiling analysis.
---

# Pprof

Ella Core exposes a [pprof](https://pkg.go.dev/net/http/pprof) compatible API for profiling analysis. Profiling endpoints are only available to admin users and scraping requires an API token.

## Profiling Endpoints

| Method | Path | Description |
| ------ | ---- | ----------- |
| GET | `/api/v1/pprof/` | An HTML page listing the available profiles. |
| GET | `/api/v1/pprof/allocs` | A sampling of historical memory allocations over the life of the program. |
| GET | `/api/v1/pprof/block` | A sampling of goroutine blocking events. |
| GET | `/api/v1/pprof/cmdline` | The command line invocation of the program. |
| GET | `/api/v1/pprof/goroutine` | A stack trace of all current goroutines. |
| GET | `/api/v1/pprof/heap` | A sampling of memory allocations of live objects. |
| GET | `/api/v1/pprof/mutex` | A sampling of mutex contention events. |
| GET | `/api/v1/pprof/profile` | A 30-second CPU profile. |
| GET | `/api/v1/pprof/threadcreate` | A sampling of thread creation events. |
| GET | `/api/v1/pprof/trace` | A 1-second execution trace. |
| POST | `/api/v1/pprof/symbol` | Symbol information for program counter (PC) addresses, used by pprof tooling to map raw addresses back to function names. |
