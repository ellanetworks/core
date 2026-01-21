---
description: RESTful API reference for profiling analysis.
---

# Pprof

Ella Core exposes a [pprof](https://pkg.go.dev/net/http/pprof) compatible API for profiling analysis. Profiling endpoints are only available to admin users and scraping requires an API token.

## Index

This endpoint returns an HTML page listing the available profiles.

| Method | Path              |
| ------ | ----------------- |
| GET    | `/api/v1/pprof/`   |

## Allocs

This endpoint returns a sampling of historical memory allocations over the life of the program.


| Method | Path                  |
| ------ | --------------------- |
| GET    | `/api/v1/pprof/allocs` |


## Block

This endpoint returns a sampling of goroutine blocking events.

| Method | Path                 |
| ------ | -------------------- |
| GET    | `/api/v1/pprof/block` |


## Cmdline

This endpoint returns the command line invocation of the program.

| Method | Path                   |
| ------ | ---------------------- |
| GET    | `/api/v1/pprof/cmdline` |

## Goroutine

This endpoint returns a stack trace of all current goroutines.

| Method | Path                     |
| ------ | ------------------------ |
| GET    | `/api/v1/pprof/goroutine` |

## Heap

This endpoint returns a sampling of memory allocations of live objects.

| Method | Path               |
| ------ | ------------------ |
| GET    | `/api/v1/pprof/heap` |

## Mutex

This endpoint returns a sampling of mutex contention events.

| Method | Path               |
| ------ | ------------------ |
| GET    | `/api/v1/pprof/mutex` |


## Profile

This endpoint returns a 30-second CPU profile.

| Method | Path                 |
| ------ | -------------------- |
| GET    | `/api/v1/pprof/profile` |

## Threadcreate

This endpoint returns a sampling of thread creation events.

| Method | Path                     |
| ------ | ------------------------ |
| GET    | `/api/v1/pprof/threadcreate` |

## Trace

This endpoint returns a 1-second execution trace.

| Method | Path                |
| ------ | ------------------- |
| GET    | `/api/v1/pprof/trace` |

## Symbol

This endpoint is used to look up program counter (PC) addresses and return symbol information (for example, function names). It is primarily used by pprof tooling to map raw addresses in profiles back to human-readable symbols.

| Method | Path                  |
| ------ | --------------------- |
| POST   | `/api/v1/pprof/symbol` |
