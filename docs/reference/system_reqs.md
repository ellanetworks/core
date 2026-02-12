---
description: System requirements for running Ella Core.
---

# System Requirements

Ella Core runs on major Linux distributions including **Ubuntu**, **Ubuntu Core**, **Debian**, and **Arch Linux** (kernel 6.8 or later). It supports **amd64** and **arm64** architectures.

|  | Minimum | Production |
|------------|----------|-------------|
| **CPU** | 1 core | 4 cores |
| **Memory** | 1 GB RAM | 8 GB RAM |
| **Storage** | 10 GB disk space | 50 GB disk space |
| **Network** | 1 interface with XDP support and appropriate MTU (see the official [XDP documentation][xdp] for driver support and the [connectivity reference](connectivity.md)) | 4 interfaces with XDP support and appropriate MTU (see the official [XDP documentation][xdp] for driver support and the [connectivity reference](connectivity.md)) |
| **Operating System** | — | Ubuntu 24.04 LTS |

[xdp]: https://docs.ebpf.io/linux/program-type/BPF_PROG_TYPE_XDP/
