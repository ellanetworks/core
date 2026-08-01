---
description: How Ella Core's data plane is built, and what the choice of kernel hook implies.
---

# Data Plane Packet processing with eBPF

The data plane is the part of Ella Core that carries subscriber traffic, between the **N3 / S1-U** and **N6 / SGi** interfaces. This page explains how it is built and why, so that you can reason about the choices it exposes — chiefly which kernel hook it attaches to.

## eBPF, XDP, and TCX

[eBPF](https://ebpf.io/) is a technology that allows custom programs to run in the Linux kernel. eBPF is used in various networking, security, and performance monitoring applications.

[XDP](https://www.iovisor.org/technology/xdp) provides a framework for eBPF that enables high-performance programmable packet processing in the Linux kernel. XDP runs in the network driver, before the kernel builds a socket buffer for the packet.

TCX is a second attach point, added in kernel 6.6. It runs later, on the socket buffer, and is available on every interface regardless of driver support.

## Data Plane Packet processing in Ella Core

Ella Core's data plane uses eBPF to achieve high throughput and low latency. Key features include:

- **Policy rules enforcement**: Evaluating ordered per-policy uplink and downlink rules to allow or deny traffic based on remote prefix, protocol, and port range.
- **Encapsulation and decapsulation**: Managing GTP-U (GPRS Tunneling Protocol-User Plane) headers for data transmission.
- **Rate limiting**: Enforcing Quality of Service (QoS) with QER (QoS Enforcement Rules).
- **Flow reporting**: Recording per-flow traffic details including source, destination, protocol, port, and whether the flow was allowed or dropped.
- **Usage reporting**: Aggregating per-subscriber byte counts for data usage tracking.
- **Statistics collection**: Monitoring metrics such as packet counts, drops, and processing times.

<figure markdown="span">
  ![eBPF Ella Core](../images/ebpf.svg){ width="800" }
  <figcaption>Packet processing in Ella Core with eBPF and XDP (Simplified to only show N3->N6).</figcaption>
</figure>

### Routing

Ella Core currently relies on the kernel to make routing decisions for incoming network packets. Kernel routes can be configured using the [Networking API](../reference/api/networking.md) or the user interface.

### NAT

Network Address Translation (NAT) lets subscribers use private addresses without an external router: uplink traffic leaves N6 sourced from Ella Core's own address, and the return traffic is translated back. The trade-off is reachability. A subscriber is reachable only through a flow it started itself, and traffic that cannot be translated is dropped rather than forwarded. To reach subscribers from the data network, turn NAT off and route the UE pool instead.

See [Connectivity](../reference/connectivity.md#nat) for what NAT translates and what it drops.

### Performance

Detailed performance results are available [here](../reference/performance.md).

### Attach modes

The data plane can attach at either of two kernel hooks, and the choice decides
how much of the kernel's own processing has already happened when it sees a
packet.

**XDP** runs in the network driver, before the kernel builds a socket buffer.
Nothing has been merged, offloaded or annotated yet, so the data plane sees
exactly what arrived on the wire. That is why it is the fastest option, and why
it needs a driver that supports the hook.

**TCX** runs on the socket buffer, after the kernel's receive path. It is
available on every interface, including the veth pairs used in containers and
[co-hosted deployments](../how_to/co_host_with_ocudu.md), which is what makes it
the option that always works. The cost is that the kernel may already have
merged several received packets into one buffer larger than the MTU. Such a
frame cannot be encapsulated into GTP-U that is valid on the wire — the tunnel
headers are copied unchanged into each packet the kernel later splits it back
into, so their lengths and checksums describe the merged frame rather than
themselves. Ella Core drops those frames rather than emit them, which is why
`tcx` requires generic receive offload to be turned off on N6.

There is also a **generic XDP** hook, which presents the XDP interface but runs
late in the receive path like TCX. It inherits TCX's exposure without exposing
the socket-buffer metadata needed to detect it, and adds a checksum hazard of
its own on veth pairs. It exists for development on hardware that supports
neither of the other two.

Each mode's requirements are listed under [Attach
modes](../reference/config_file.md#attach-modes). The mode a running node
actually attached with is reported by [`GET
/api/v1/status`](../reference/api/status.md), which matters because the default
setting resolves to driver XDP or TCX depending on the hardware it finds.

### XDP redirect on veth pairs

When Ella Core's N3 interface is a veth pair, the data plane forwards downlink packets from N6 to N3 with `bpf_redirect()`. In `xdp-native` mode the veth driver delivers redirected frames through the native path only when the receiving peer also has an XDP program attached; without one, the frames are dropped. Attaching a minimal `XDP_PASS` program to the peer satisfies that requirement — see [Use native XDP with veth interfaces](../how_to/native_xdp_veth.md).

### IPv6 GTP-U transport

Ella Core supports GTP-U encapsulation with either an IPv4 or IPv6 outer header on the N3 / S1-U interface. The inner UE payload can be IPv4 or IPv6, independent of the transport address family. The chosen transport address family depends on how the N3 / S1-U interface is configured, and what the radio advertises. If both sides are dual-stack, Ella Core prefers IPv6.

**GTP echo:** Echo Request/Response messages are handled for both IPv4 and IPv6 transport, as required for GTP-U path management.
