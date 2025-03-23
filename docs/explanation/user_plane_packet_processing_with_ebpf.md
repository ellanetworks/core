---
description: Data Plane Packet Processing with eBPF explanation - Definitions, components, and workflow of packet processing.
---

# Data Plane Packet processing with eBPF

This document explains the key concepts behind packet Ella Core's packet processing. It covers the components, workflow, and technologies used in the data plane. We refer to the data plane as the part of Ella Core that processes subscriber data packets.

## eBPF and XDP

[eBPF](https://ebpf.io/) is a technology that allows custom programs to run in the Linux kernel. eBPF is used in various networking, security, and performance monitoring applications.

[XDP](https://www.iovisor.org/technology/xdp) provides a framework for eBPF that enables high-performance programmable packet processing in the Linux kernel.

## Data Plane Packet processing in Ella Core

Ella Core's data plane uses XDP to achieve high throughput and low latency. Key features include:

- **Packet filtering**: Applying rules to determine whether packets should be dropped, forwarded, or passed.
- **Encapsulation and decapsulation**: Managing GTP-U (GPRS Tunneling Protocol-User Plane) headers for data transmission.
- **Rate limiting**: Enforcing Quality of Service (QoS) with QER (QoS Enforcement Rules).
- **Statistics collection**: Monitoring metrics such as packet counts, drops, and processing times.

Data plane processing in Ella Core occurs between the **n3** and **n6** interfaces.

### Routing

Ella Core currently relies on the kernel to make routing decisions for incoming network packets. Kernel routes can be configured using the [Routes API](../reference/api/routes.md) or the user interface.

### NATing

Ella Core currently does not support Network Address Translation (NAT). If a subscriber is assigned a private IP address, the subscriber's packets will not be translated to a public IP address when sent to the Internet. In the [End-to-End Network tutorial](../tutorials/end_to_end_network_snap.md), we use an external router to enable NATting using the `iptables` command so that subscribers can use publicly routable addresses.

### Performance

Detailed performance results are available [here](../reference/performance.md).

### Configuration

Ella Core supports the following XDP attach modes:

- **Native**: This is the most performant option, but it is only supported on [compatible drivers](https://github.com/iovisor/bcc/blob/master/docs/kernel-versions.md#xdp).
- **Generic**: A fallback option that works on most drivers but with lower performance.

For more information on configuring XDP attach modes, refer to the [Configuration File](../reference/config_file.md) documentation.
