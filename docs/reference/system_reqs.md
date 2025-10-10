---
description: System requirements for running Ella Core.
---

# System Requirements

Ella Core is available on many Linux distributions, including Ubuntu, Ubuntu Core, Debian, Arch, and more. It supports both amd64 and arm64 architectures.

!!! note
    View the Ella Core snap on the [Snap Store](https://snapcraft.io/ella-core).

## Minimum Requirements

To run Ella Core, your system should meet the following minimum requirements:

- **CPU**: 2 CPU cores (amd64 or arm64 architecture)
- **Memory**: 2 GB of RAM
- **Storage**: 10 GB of available disk space
- **Network**: 4 network interfaces
- **Operating System**: Linux with kernel 6.8 or above, with support for those features:
  - eBPF
  - XDP
  - nftables
  - vrf
  - SCTP protocol

## Production Requirements

For production deployments, it is recommended to have the following specifications:

- **CPU**: 4 CPU cores (amd64)
- **Memory**: 8 GB of RAM
- **Storage**: 50 GB of available disk space
- **Network**: 4 network interfaces
- **Operating System**: Ubuntu 24.04 LTS
