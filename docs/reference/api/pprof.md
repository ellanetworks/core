---
description: RESTful API reference for profiling analysis.
---

# Pprof

Ella core exposes a [pprof](https://pkg.go.dev/net/http/pprof) compatible API for profiling analysis. Profiling endpoints are only available to admin users and scraping requires an API token.

## Index

This endpoint returns an HTML page listing the available profiles.

| Method | Path              |
| ------ | ----------------- |
| GET    | `/debug/pprof/`   |

## Allocs

This endpoint returns a sampling of historical memory allocations over the life of the program.


| Method | Path                  |
| ------ | --------------------- |
| GET    | `/debug/pprof/allocs` |


## Block

This endpoint returns a sampling of goroutine blocking events.

| Method | Path                 |
| ------ | -------------------- |
| GET    | `/debug/pprof/block` |


## Cmdline

This endpoint returns the command line invocation of the program.

| Method | Path                   |
| ------ | ---------------------- |
| GET    | `/debug/pprof/cmdline` |

## Goroutine

This endpoint returns a stack trace of all current goroutines.

| Method | Path                     |
| ------ | ------------------------ |
| GET    | `/debug/pprof/goroutine` |

## Heap

This endpoint returns a sampling of memory allocations of live objects.

| Method | Path               |
| ------ | ------------------ |
| GET    | `/debug/pprof/heap` |

## Mutex

This endpoint returns a sampling of mutex contention events.

| Method | Path               |
| ------ | ------------------ |
| GET    | `/debug/pprof/mutex` |


## Profile

This endpoint returns a 30-second CPU profile.

| Method | Path                 |
| ------ | -------------------- |
| GET    | `/debug/pprof/profile` |

## Threadcreate

This endpoint returns a sampling of thread creation events.

| Method | Path                     |
| ------ | ------------------------ |
| GET    | `/debug/pprof/threadcreate` |

## Trace

This endpoint returns a 1-second execution trace.

| Method | Path                |
| ------ | ------------------- |
| GET    | `/debug/pprof/trace` |
