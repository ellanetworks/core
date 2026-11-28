# v1.22

Breaking changes

This release contains breaking changes. Review the Breaking changes section below before upgrading. For clusters, follow the [upgrade guide](https://docs.ellanetworks.com/core/how_to/upgrade/index.md).

- **Release date**: November 11, 2026
- **GitHub release**: [v1.22.0](https://github.com/ellanetworks/core/releases/tag/v1.22.0)

## New features

- **Roaming (local breakout)**: Ella Core acts as a visited network for roaming partners over 4G and 5G. The subscriber's home network authenticates them and holds their subscription, and Ella Core carries their traffic to its local data networks. Configure roaming partners from the Integrations page and view roamers from the Subscribers page. See the [roaming guide](https://docs.ellanetworks.com/core/how_to/configure_roaming/index.md).
- **Integrations page**: A new page to configure IMS, SMS and Roaming.
- **Multiple network instances**: Each data network can leave the user plane on its own interface, VLAN or VRF. Declare network instances in the config file and pick one per slice and data network. For example, when co-hosting Ella Core with [Ella IMS](https://github.com/ellanetworks/ims) and [Ella SMSC](https://github.com/ellanetworks/smsc), the `ims` data network leaves through a veth into the IMS namespace while `internet` leaves through N6. See the [co-hosting guide](https://docs.ellanetworks.com/core/how_to/co_host_with_ims_and_smsc/index.md).
- **DSCP marking**: Downlink GTP-U packets on N3/S1-U are marked with a DSCP value derived from the QoS flow's 5QI and ARP (ex. EF for voice).
- **RAN trace correlation**: Ella Core sends each UE's 3GPP trace ID to the radio and uses it as the OpenTelemetry trace ID, allowing distributed tracing through the RAN and the core.
- **Kubernetes hardening**: New `/api/v1/livez` and `/api/v1/readyz` health endpoints. The Kubernetes manifests no longer run Ella Core as a privileged container, and add explicit capabilities, resource limits and startup, readiness and liveness probes, in line with CNTi best practices.

## Maintenance

- Performance improvements for large deployments.
- Fixed migrating a standalone deployment to a cluster.
- Expose Raft metrics to Prometheus.
- Turn off packet merging (GRO, hardware GRO and LRO) on its datapath interfaces in `tcx` attach mode.
- Bug fixes
- Dependency updates

## Breaking changes

- SMS, IMS and Diameter endpoints moved under `/api/v1/integrations`.
- NAT and local switch are now set per slice and data network.

If you depend directly on the API, you may need to adapt your integration.
