---
description: Explanation of how high availability works in Ella Core.
---

# High Availability

High availability (HA) lets you run an Ella Core cluster so that the network keeps working when nodes fail. Each node is active and can accept radios and subscriber traffic.

HA is designed around the [Raft Consensus Algorithm](https://raft.github.io/): at any time one node is the leader, it is the only node that accepts writes, and every write replicates to a majority of nodes before it is considered committed. Nodes communicate together via mTLS to share changes.

<figure markdown="span">
  ![Ella Core HA cluster](../images/ha_raft.svg){ width="700" }
  <figcaption>High Availability in Ella Core</figcaption>
</figure>

## What HA covers

Deploy three or five nodes. A *voter* is a node that counts toward quorum, and a quorum is a majority of voters: 2 of 3, or 3 of 5. Three nodes tolerate one failure; five nodes tolerate two. Within those bounds, surviving voters keep accepting writes, radio traffic, and operator changes with no manual intervention.

Two things HA does not handle automatically:

- **Loss of quorum.** If more than half the voters fail at the same time, the cluster loses quorum and writes stall until enough nodes return — or the cluster is restored from backup via [Disaster recovery](#disaster-recovery).
- **In-flight UE sessions.** Sessions on a dead node drop; those UEs re-register on a surviving node.

## What replicates, and what does not

Network-wide resources — subscribers, profiles, policies, slices, data networks, network rules, IP leases, users, API tokens, audit logs, the operator configuration — replicate across the cluster. Every replicated record carries a globally-unique ID, so rows created on different nodes never collide. If a node dies, the survivors hold the same state, automatically elect a new leader, and keep accepting writes.

Per-node configuration does not replicate. This covers the local data-plane and routing settings each node owns: static routes, BGP settings, BGP peers and import prefixes, NAT, the N3 external address, and flow accounting. To configure these on an HA cluster, hit each node's API directly — a change made on one node does not propagate to its peers. This lets nodes in different racks or AZs run with different upstream gateways and BGP topologies.

Runtime state tied to a specific connection or session also does not replicate: SCTP associations with radios, UE contexts, active sessions and their User Plane state, GTP-U tunnels, and active BGP adjacencies.

Observability is per-node: each instance exposes its own Prometheus endpoint, radio events, and flow reports, so operators scrape every node for a cluster-wide view.

## User plane and routing

A UE's user-plane traffic flows through the node that handled its registration — that node runs its User Plane and terminates its GTP-U tunnel. Each data network has one cluster-wide IP pool; the replicated lease table guarantees no two UEs receive the same address, and each lease records the node currently serving it.

When BGP is enabled, each node advertises a `/32` route for every UE session it hosts (see [Advertising routes via BGP](bgp.md)). When a UE re-registers on a different node after failover, the lease's owning node is updated in place — the UE keeps its IP — and the new node's speaker begins advertising the same `/32` from its N6. The dead node's BGP session times out after the hold timer (90 s by default, configurable per peer), its routes are withdrawn, and upstream routing converges on the survivor without operator action.

## Failover and timing

Leader re-election completes within a few seconds; surviving nodes continue accepting NGAP, S1AP, and API calls the whole time.

Each Ella Core node presents as a distinct AMF in the same AMF Set (5G) and a distinct MME — a distinct GUMMEI — in a single MME Pool (4G). A UE's GUTI pins it to the node that handled its registration, and new UEs distribute across the nodes by advertised capacity. When a node dies, radios detect the loss via SCTP heartbeat timeout and reselect a surviving AMF/MME. UEs that were attached to the dead node then re-register from scratch, including a fresh authentication and a new session.

## Deployment scenarios

The HA cluster is the same regardless of how radios connect to it; the radio side determines how much HA reaches individual UEs.

### Radios Connected to Every Node (AMF Set / MME Pool)

When a Core dies, radios reselect within the Set/Pool automatically; affected UEs re-register on a surviving node without operator action.

<figure markdown="span">
  ![Radios Connected to Every Node (AMF Set / MME Pool)](../images/ha_scenario_1.svg){ width="700" }
  <figcaption>Radios Connected to Every Node (AMF Set / MME Pool)</figcaption>
</figure>

### Radios Pinned to Specific Nodes

Useful for site- or tenant-partitioned deployments. Network-wide state still replicates, so subscribers and policies stay consistent across nodes — but if a Core dies, its paired radios lose connectivity to the core and must be reconfigured to reach a surviving node. UE failover is manual, not automatic.

<figure markdown="span">
  ![Radios Pinned to Specific Nodes](../images/ha_scenario_2.svg){ width="700" }
  <figcaption>Radios Pinned to Specific Nodes</figcaption>
</figure>

## Draining a node

Draining prepares a node for removal without disrupting traffic on its peers. A drained node hands Raft leadership to another voter if it held it, signals connected radios that it is unavailable so new UEs attach elsewhere, and stops advertising user-plane routes so upstream routing shifts to the survivors. Existing flows keep running until the node is removed or shut down.

Drain is triggered by an operator via the cluster API. Removal requires a drained node.

## Scaling the cluster

A new node joins as a voter by default: once it has exchanged its join token and started, the leader adds it directly to the voting set. If you would rather have the node catch up on the Raft log before it counts toward quorum, set `initial-suffrage: nonvoter` in its config — it then joins as a non-voter and Autopilot promotes it to a voter automatically once it has been healthy and up-to-date for a short stabilization window (or you can promote it immediately via the promote endpoint).

Shrinking is symmetric. Drain the node, then remove it; the remaining voters continue serving writes while the configuration change commits.

## Inter-node communication using mTLS

Every inter-node connection is mutually authenticated over TLS. Each node owns a long-lived, self-signed cluster certificate. To admit a new node, an admin mints a single-use join token from the Cluster page; the node presents it once to register itself, after which every voter accepts its connections. Certificates are scoped to a single cluster, so credentials from one cluster cannot authenticate into another. Removing a node immediately revokes its access cluster-wide.

## Disaster recovery

HA clusters recover from total loss through an offline, backup-driven path. An operator stops every node, seeds one node from a backup archive, and starts it — it comes up as a single-voter cluster carrying the restored state. The remaining voters then rejoin with fresh join tokens. The backup archive carries the cluster's identity and its trusted-node registrations, so the restored leader trusts itself immediately; freshly-joined voters re-register through the standard join flow. The step-by-step procedure lives in [Backup and Restore](../how_to/backup_and_restore.md).

## Rolling upgrades

Upgrades proceed one node at a time: drain the node, refresh its binary, then resume. Each node retains its node-id, certificate, and Raft membership across the swap. Writes continue throughout; the cluster is briefly mixed-version during each step.

When the new binary carries schema changes, the cluster keeps running on the old schema until every voter reports support live via its cluster-internal status endpoint; only then does the migration commit through Raft. Migration progress is observable through the status endpoint.

Skip-version upgrades (`vN → vN+2`) and downgrades are not supported.

## Further reading

- [Deploy a High Availability Cluster](../how_to/deploy_ha_cluster.md) — step-by-step guide to bring up a cluster.
- [Scale Up a High Availability Cluster](../how_to/scale_up_ha_cluster.md) — add nodes to an existing cluster.
- [Perform a Rolling Upgrade](../how_to/rolling_upgrade.md) — upgrade every node without taking the cluster offline.
- [Cluster API reference](../reference/api/cluster.md) — cluster management endpoints.
