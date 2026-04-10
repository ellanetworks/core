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

IPv6 GTP-U transport (dual-stack N3)
=====================================

The XDP datapath supports GTP-U encapsulation where the **outer** IP header is
either IPv4 or IPv6. The encapsulated payload (UE traffic) remains IPv4 or IPv6
regardless of the transport address family.

### FAR struct layout

`far_info` in `xdp/utils/pdr.h` stores tunnel addresses as `struct in6_addr`
(16 bytes) for both `remoteip` and `localip`:

- **IPv4 transport** — address stored as IPv4-mapped IPv6 (`::ffff:x.x.x.x`),
  identical to `AF_INET6` with `IN6_IS_ADDR_V4MAPPED`.
- **IPv6 transport** — address stored natively.

The `outer_header_creation` field uses the 3GPP-defined bit values:

| Value | Meaning |
|-------|---------|
| `0x01` (`OHC_GTP_U_UDP_IPv4`) | GTP-U/UDP over IPv4 outer header |
| `0x02` (`OHC_GTP_U_UDP_IPv6`) | GTP-U/UDP over IPv6 outer header |

### Address selection at session establishment

When the SMF programs a FAR, it sets `outer_header_creation` based on the
address family of the gNB's `TransportLayerAddress` received in the NGAP
`PDUSessionResourceSetupResponse`. The UPF picks the corresponding local N3
address (`localip`) from the session engine, which holds separate IPv4 and IPv6
N3 addresses resolved from the N3 interface at startup.

### Uplink decapsulation

`handle_ip6()` in `n3n6_bpf.c` mirrors `handle_ip4()` — it checks for
UDP port 2152 and dispatches to `handle_gtpu()`. `remove_gtp_header()` uses the
`outer_header_removal` value to determine whether to strip an IPv4 (20 B) or
IPv6 (40 B) outer header.

### GTP echo

`handle_echo_request()` in `gtp.h` handles echo requests over both IPv4
(`swap_ip`) and IPv6 (`swap_ip6`) outer headers.

### ICMPv6 Packet Too Big

When a downlink packet exceeds the path MTU on an IPv6 transport path,
`frag_needed_ipv6()` in `frag_needed.h` generates an ICMPv6 Packet Too Big
message (Type 2, Code 0) and sends it back via `XDP_TX`. The ICMPv6 checksum
is computed with `icmpv6_ptb_csum()` in `csum.h` using the standard IPv6
pseudo-header.

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
