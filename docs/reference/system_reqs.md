---
description: System requirements for running Ella Core.
---

# System Requirements

Ella Core runs on major Linux distributions including **Ubuntu**, **Ubuntu Core**, **Debian**, and **Arch Linux** (kernel 6.8 or later). It supports amd64 and arm64 architectures.

|  | Minimum | Production |
|------------|----------|-------------|
| **CPU** | 2 cores | 4 cores |
| **Memory** | 2 GB RAM | 8 GB RAM |
| **Storage** | 10 GB disk space | 50 GB disk space |
| **Network** | 4 interfaces:<br>• 2 user-plane (**n3**, **n6**) — driver with XDP support and appropriate MTU ([XDP docs][xdp])<br>• 1 control-plane (**n2**)<br>• 1 management | 4 interfaces:<br>• 2 user-plane (**n3**, **n6**) — driver with XDP support and appropriate MTU ([XDP docs][xdp])<br>• 1 control-plane (**n2**)<br>• 1 management |
| **Operating System** | — | Ubuntu 24.04 LTS |

[xdp]: https://docs.ebpf.io/linux/program-type/BPF_PROG_TYPE_XDP/
