# High Availability

High availability (HA) lets you run an Ella Core cluster so that the network keeps working when nodes fail. Each node is active and can accept radios and subscriber traffic.

HA is designed around the [Raft Consensus Algorithm](https://raft.github.io/): at any time one node is the leader, it is the only node that commits writes, and every write replicates to a majority of voters before it is considered committed. Nodes communicate together via mTLS to share changes.

High Availability in Ella Core

## Failure tolerance

A cluster keeps accepting writes as long as a majority of its *voters* — the nodes that count toward quorum — are alive. Deploy an odd number of nodes. We recommend 3 or 5, since an even count adds a node without adding tolerance.

| Voters | Quorum | Failures tolerated |
| ------ | ------ | ------------------ |
| 3      | 2      | 1                  |
| 5      | 3      | 2                  |

Within those bounds, surviving voters keep accepting writes, radio traffic, and operator changes with no manual intervention.

Two things HA does not handle automatically:

- **Loss of quorum.** If a majority of voters is lost, the cluster loses quorum and writes stall until enough nodes return or the cluster is restored from backup via [Disaster recovery](#disaster-recovery).
- **In-flight UE sessions.** Sessions on a dead node drop; those UEs re-register on a surviving node, as described in [Failover](#failover).

## What replicates, and what does not

Network-wide resources (subscribers, profiles, policies, slices, data networks, network rules, IP leases, users, API tokens, the operator configuration) replicate across the cluster. If a node dies, the survivors hold the same state, automatically elect a new leader, and keep accepting writes.

Per-node configuration does not replicate. This covers the local data-plane and routing settings each node owns. To configure these on an HA cluster, hit each node's API directly.

Runtime state tied to a specific connection or session also does not replicate: SCTP associations with radios, UE contexts, active sessions and their User Plane state, GTP-U tunnels, and active BGP adjacencies.

Observability is per-node: each instance exposes its own Prometheus endpoint, radio events, flow reports, and audit logs, so operators scrape every node for a cluster-wide view.

## Failover

Each Ella Core node presents as a distinct AMF in the same AMF Set (5G) and a distinct MME in a single MME Pool (4G). A UE's GUTI pins it to the node that handled its registration. When a node dies, radios detect the loss and reselect a surviving AMF/MME. UEs that were attached to the dead node then re-register from scratch, including a fresh authentication and a new session.

## User plane and routing

A UE's user-plane traffic flows through the node that handled its registration. In an HA cluster, no two UEs receive the same IP address.

When BGP is enabled, each node advertises a `/32` (IPv4) or `/64` (IPv6) route for every UE session it hosts (see [Advertising routes via BGP](https://docs.ellanetworks.com/explanation/bgp/index.md)).

## Deployment scenarios

The HA cluster is the same regardless of how radios connect to it. The radio side determines how much HA reaches individual UEs.

### Radios connected to every node (AMF Set / MME Pool)

When a node dies, radios reselect within the AMF Set or MME Pool automatically. Affected UEs re-register on a surviving node without operator action.

Radios connected to every node (AMF Set / MME Pool)

### Radios pinned to specific nodes

Useful for site- or tenant-partitioned deployments. Network-wide state still replicates but if a node dies, its paired radios lose connectivity to the core and must be reconfigured to reach a surviving node.

Radios pinned to specific nodes

## Draining a node

Draining prepares a node for removal without disrupting traffic on its peers. A draining node signals connected radios so new UEs attach elsewhere, and stops advertising user-plane routes so upstream routing shifts to the survivors.

Drain is triggered by an operator via the cluster API or UI.

## Scaling the cluster

To scale up, follow the steps in [Scale Up a High Availability Cluster](https://docs.ellanetworks.com/how_to/scale_up_ha_cluster/index.md).

Shrinking is symmetric. Drain the node, then remove it.

## Inter-node communication using mTLS

Every inter-node connection is mutually authenticated over TLS. Each node owns a long-lived, self-signed cluster certificate. Removing a node immediately revokes its access cluster-wide.

## Disaster recovery

HA clusters recover from total loss through an offline, backup-driven path. An operator stops every node, seeds one node from a backup archive, and starts it. The remaining voters then rejoin with fresh join tokens. The step-by-step procedure lives in [Backup and Restore](https://docs.ellanetworks.com/how_to/backup_and_restore/index.md).

## Rolling upgrades

Upgrades proceed one node at a time: drain the node, refresh its binary, then resume.

Skip-version upgrades (`vN → vN+2`) and downgrades are not supported.

## Further reading

- [Deploy a High Availability Cluster](https://docs.ellanetworks.com/how_to/deploy_ha_cluster/index.md) — step-by-step guide to bring up a cluster.
- [Scale Up a High Availability Cluster](https://docs.ellanetworks.com/how_to/scale_up_ha_cluster/index.md) — add nodes to an existing cluster.
- [Perform a Rolling Upgrade](https://docs.ellanetworks.com/how_to/rolling_upgrade/index.md) — upgrade every node without taking the cluster offline.
- [Cluster API reference](https://docs.ellanetworks.com/reference/api/cluster/index.md) — cluster management endpoints.
