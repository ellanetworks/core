---
description: System requirements for running Ella Core.
---

# System Requirements

Ella Core runs on major Linux distributions including **Ubuntu**, **Ubuntu Core**, **Debian**, and **Arch Linux** (kernel 6.8 or later). It supports **amd64** and **arm64** architectures.

|  | 1,000 sessions | 100,000 sessions |
|------------|----------|-------------|
| **CPU** | 1 core | 4 cores |
| **Memory** | 1 GB RAM | 8 GB RAM |
| **Storage** | 10 GB disk space | 50 GB disk space |
| **`datapath.max-sessions`** | `2000` (default) | `100000` |

Each column assumes every subscriber holds an active PDU session. See the [configuration file reference](config_file.md) for `datapath.max-sessions`.

Ella Core requires network interfaces with XDP support and an appropriate MTU: 1 interface at minimum and 4 in production. See the official [XDP documentation][xdp] for driver support and the [connectivity reference](connectivity.md). The recommended operating system for production is Ubuntu 26.04 LTS.

[xdp]: https://docs.ebpf.io/linux/program-type/BPF_PROG_TYPE_XDP/
