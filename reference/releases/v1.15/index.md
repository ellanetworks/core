# v1.15

- **Release date**: July 28, 2026
- **GitHub release**: [v1.15.0](https://github.com/ellanetworks/core/releases/tag/v1.15.0)

## New features

- **Direction-aware NAT**: NAT connection tracking now records the direction a flow was initiated in, so only downlink traffic that belongs to a connection opened by a subscriber is forwarded. Unsolicited packets from the data network are dropped at the NAT boundary. The NAT port range was also moved outside the host's ephemeral range to avoid collisions with host-initiated traffic.
- **Unified eBPF datapath**: The eBPF program was simplified and consumes less memory.
- **Location (beta)**: Stale Cell ID locations are now automatically refreshed.

## Maintenance

- Security fixes
- Bug fixes
- Dependency updates
