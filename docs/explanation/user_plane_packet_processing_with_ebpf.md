---
description: Data Plane Packet Processing with eBPF explanation - Definitions, components, and workflow of packet processing.
---

# Data Plane Packet processing with eBPF

This document explains the key concepts behind packet Ella Core's packet processing. It covers the components, workflow, and technologies used in the data plane. We refer to the data plane as the part of Ella Core that processes subscriber data packets.

## eBPF and XDP

[eBPF](https://ebpf.io/) is a technology that allows running custom programs in the Linux kernel. eBPF is used in various networking, security, and performance monitoring applications.

[XDP](https://www.iovisor.org/technology/xdp) provides a framework for eBPF that enables high-performance programmable packet processing in the Linux kernel.

## Data Plane Packet processing in Ella Core

Ella Core's data plane is implemented using XDP to achieve high throughput and low latency. Key features include:

- **Packet filtering**: Applying rules to determine whether packets should be dropped, forwarded, or passed.
- **Encapsulation and decapsulation**: Managing GTP-U (GPRS Tunneling Protocol-User Plane) headers for data transmission.
- **Rate limiting**: Enforcing Quality of Service (QoS) with QER (QoS Enforcement Rules).
- **Statistics collection**: Monitoring metrics such as packet counts, drops, and processing times.

## Note on Configuration

Ella Core supports the following XDP attach modes:

- **Native**: The most performant option, only supported on [compatible drivers](https://github.com/iovisor/bcc/blob/master/docs/kernel-versions.md#xdp).
- **Generic**: A fallback option that works on most drivers, but with lower performance.

For more information on configuring XDP attach modes, refer to the [Configuration File](../reference/config_file.md) documentation.
