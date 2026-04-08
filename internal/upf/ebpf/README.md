Ella Core UPF XDP implementation
================================

This directory contains the XDP implementation of the UPF. It contains
both the userspace go code and the kernel space XDP code.

Compilation
===========

In most cases, you should use `go generate` from the top-level directory
to build the project.

For development of the C XDP code, cmake can be used to create files to
support the `clangd` LSP. Run the following command in this directory to
generate the files:

`cmake .`

Build flags
===========

Optional build flags can be passed via the `BPF_CFLAGS` environment variable
when running `go generate`:

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `-DENABLE_LOG`        | Enables debug output to the BPF trace pipe (`bpftool prog tracelog`).       |
| `-DENABLE_PROFILING`  | Enables per-packet latency profiling. Measures elapsed nanoseconds for each pipeline sub-stage (PDR lookup, MTU check, QER rate-limit, GTP manipulation, SDF filter, NAT, FIB routing) independently for N3 uplink and N6 downlink. Results are stored in `profiling_map` (a per-CPU array) and can be read from Go with `ReadProfilingStats()` in `stats.go`. **Note**: each `bpf_ktime_get_ns()` call adds approximately 600–900 ns of overhead; with all sections enabled this is roughly 9–14 µs per packet. |

Examples:

```shell
# Enable debug logging only
export BPF_CFLAGS="-DENABLE_LOG"
go generate ./internal/upf/ebpf/

# Enable profiling only
export BPF_CFLAGS="-DENABLE_PROFILING"
go generate ./internal/upf/ebpf/

# Enable both
export BPF_CFLAGS="-DENABLE_LOG -DENABLE_PROFILING"
go generate ./internal/upf/ebpf/
```

Inspecting UPF state
====================

Maps used to drive the XDP code can be inspected with `bpftool`.

For example, it is possible to list all the uplink PDRs with the following
command:


```shell
sudo bpftool map dump name pdrs_uplink
```

When profiling is enabled, the profiling map can be inspected with:

```shell
sudo bpftool map dump name profiling_map
```
