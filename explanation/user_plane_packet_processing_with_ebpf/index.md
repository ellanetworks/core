# Data Plane Packet processing with eBPF

This document explains the key concepts behind packet Ella Core's subscriber data packet processing, between the **N3 / S1-U** and **N6 / SGi** interfaces. It covers the components, workflow, and technologies used in the data plane.

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

Packet processing in Ella Core with eBPF and XDP (Simplified to only show N3->N6).

### Routing

Ella Core currently relies on the kernel to make routing decisions for incoming network packets. Kernel routes can be configured using the [Networking API](https://docs.ellanetworks.com/reference/api/networking/index.md) or the user interface.

### NAT

Network Address Translation (NAT) lets subscribers use private addresses without an external router: uplink traffic leaves N6 sourced from Ella Core's own address, and the return traffic is translated back. The trade-off is reachability. A subscriber is reachable only through a flow it started itself, and traffic that cannot be translated is dropped rather than forwarded. To reach subscribers from the data network, turn NAT off and route the UE pool instead.

See [Connectivity](https://docs.ellanetworks.com/reference/connectivity/#nat) for what NAT translates and what it drops.

### Performance

Detailed performance results are available [here](https://docs.ellanetworks.com/reference/performance/index.md).

### Attach modes

The data plane can attach at either of two kernel hooks, and the choice decides how much of the kernel's own processing has already happened when it sees a packet.

- **xdp-native** runs in the network driver, before the kernel builds a socket buffer. Nothing has been merged, offloaded or annotated yet, so the data plane sees exactly what arrived on the wire. That is why it is the fastest option, and why it needs a driver that supports the hook.
- **tcx** runs on the socket buffer, after the kernel's receive path. It is available on every interface, including the veth pairs used in containers and [co-hosted deployments](https://docs.ellanetworks.com/how_to/co_host_with_ocudu/index.md), which is what makes it the option that always works.
- **xdp-generic** presents the XDP interface but runs late in the receive path, like TCX. It should only be used for testing and prototyping.

### Merged packets

Both hooks that run on the socket buffer can be handed one holding several packets merged together — merged by the kernel's receive path, or handed over already merged by a veth or virtio peer that offloads segmentation. Neither encapsulation nor decapsulation can produce valid GTP-U from that. Encapsulation writes one tunnel header for the whole buffer, and when the kernel splits it back into wire-sized packets it copies that header onto each one unchanged: every packet then claims the merged buffer's GTP-U payload length rather than its own, and with an IPv6 outer header its checksum as well. Decapsulation strips only the first packet's outer headers and leaves the rest in the payload.

TCX can see that the buffer is merged and drops it. Generic XDP cannot: a merged buffer over the MTU is answered with a spurious ICMP "fragmentation needed" and its payload is destroyed, and one under the MTU leaves as the malformed GTP-U above. Either way the remedy is to [disable merged packets](https://docs.ellanetworks.com/how_to/disable_merged_packets/index.md) on N3 and N6.

### Checksum offload on veth pairs

An application transmitting over a veth leaves the transport checksum for the egress NIC to complete, recording where to write it in the packet's metadata. In `xdp-generic` mode the kernel does not update that metadata when the data plane removes the GTP-U header, so the checksum is later written at the stale offset, corrupting the decapsulated packet at a position that depends on the header removed. Nothing detects it: neither the data plane counters nor a capture on the host. Disabling TX checksum offload on both ends of the pair (`ethtool -K <veth> tx off`) forces the checksum to be completed before the packet reaches the data plane.

`xdp-native` forwards redirected frames as raw packets, which carry no such metadata. TCX drops the request when it removes the header, because the kernel invalidates it once the checksum's start offset falls outside the packet — that covers decapsulation only, not a frame the data plane encapsulates.

### XDP redirect on veth pairs

When Ella Core's N3 interface is a veth pair, the data plane forwards downlink packets from N6 to N3 with `bpf_redirect()`. In `xdp-native` mode the veth driver delivers redirected frames through the native path only when the receiving peer also has an XDP program attached; without one, the frames are dropped. Attaching a minimal `XDP_PASS` program to the peer satisfies that requirement — see [Use native XDP with veth interfaces](https://docs.ellanetworks.com/how_to/native_xdp_veth/index.md).

### IPv6 GTP-U transport

Ella Core supports GTP-U encapsulation with either an IPv4 or IPv6 outer header on the N3 / S1-U interface. The inner UE payload can be IPv4 or IPv6, independent of the transport address family. The chosen transport address family depends on how the N3 / S1-U interface is configured, and what the radio advertises. If both sides are dual-stack, Ella Core prefers IPv6.

**GTP echo:** Echo Request/Response messages are handled for both IPv4 and IPv6 transport, as required for GTP-U path management.
