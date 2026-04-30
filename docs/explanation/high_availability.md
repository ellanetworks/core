---
description: Explanation of how high availability works in Ella Core.
---

# High Availability

High availability (HA) lets you run an Ella Core cluster so that the network keeps working when nodes fail. Each node is active and can accept 5G radios and subscriber traffic.

HA is designed around the [Raft Consensus Algorithm](https://raft.github.io/): at any time one node is the leader, it is the only node that accepts writes, and every write replicates to a majority of nodes before it is considered committed. Nodes communicate together via mTLS to share changes.

<figure markdown="span">
  ![Ella Core HA cluster](../images/ha_raft.svg){ width="700" }
  <figcaption>High Availability in Ella Core</figcaption>
</figure>

## What HA covers

Deploy three or five nodes. A quorum is a majority of voters: 2 of 3, or 3 of 5. Three nodes tolerate one failure; five nodes tolerate two. Within those bounds, surviving voters keep accepting writes, gNB traffic, and operator changes with no manual intervention.

Two things HA does not handle automatically. If more than half the voters fail at the same time, the cluster loses quorum and writes stall until enough nodes return — or the cluster is restored from backup via [Disaster recovery](#disaster-recovery). And UE sessions on a dead node drop; those UEs re-register on a surviving node.

## What replicates, and what does not

All persistent resources are replicated across the cluster, so if a node dies, the others have the same subscribers, policies, and operator configuration. The cluster automatically elects a new leader and keeps accepting operator changes with no manual intervention.

Runtime state tied to a specific connection or session does not replicate. This includes SCTP associations with gNBs, UE contexts, active PDU sessions and their User Plane state, GTP-U tunnels, and active BGP adjacencies.

Observability is also per-node: each instance exposes its own Prometheus endpoint and radio events and flow reports, so operators scrape every node for a cluster-wide view.

## User plane and routing

A UE's user-plane traffic flows through the node that handled its registration — that node runs its User Plane and terminates its GTP-U tunnel. Each data network has one cluster-wide IP pool; the replicated lease table guarantees no two UEs receive the same address, and each lease records the node currently serving it.

When BGP is enabled, each node advertises a `/32` route for every UE session it hosts (see [Advertising routes via BGP](bgp.md)). When a UE re-registers on a different node after failover, the lease's owning node is updated in place — the UE keeps its IP — and the new node's speaker begins advertising the same `/32` from its N6. The dead node's BGP session times out after the hold timer (30–180 s, peer-dependent), its routes are withdrawn, and upstream routing converges on the survivor without operator action.

## Failover and timing

Leader re-election completes within a few seconds; surviving nodes continue accepting NGAP and API calls the whole time.

Each Ella Core node presents as a distinct AMF in the same AMF Set. A UE's 5G-GUTI pins it to the AMF that handled its registration, and new UEs distribute across the Set. When a node dies, gNBs detect the loss via SCTP heartbeat timeout and reselect a surviving AMF. UEs that were attached to the dead node then re-register from scratch, including a fresh authentication and a new PDU session.

## Deployment scenarios

The HA cluster is the same regardless of how gNBs connect to it; the gNB side determines how much HA reaches individual UEs.

### Radios Connected to Every Node (AMF Set)

When a Core dies, gNBs reselect within the Set automatically; affected UEs re-register on a surviving node without operator action.

<figure markdown="span">
  ![Radios Connected to Every Node (AMF Set)](../images/ha_scenario_1.svg){ width="700" }
  <figcaption>Radios Connected to Every Node (AMF Set)</figcaption>
</figure>

### Radios Pinned to Specific Nodes

Useful for site- or tenant-partitioned deployments. The cluster still replicates operator state across all nodes, so changes made anywhere are visible everywhere — but if a Core dies, its paired gNBs lose N2 and must be reconfigured to reach a surviving node. UE failover is manual, not automatic.

<figure markdown="span">
  ![Radios Pinned to Specific Nodes](../images/ha_scenario_2.svg){ width="700" }
  <figcaption>Radios Pinned to Specific Nodes</figcaption>
</figure>

## Draining a node

Draining prepares a node for removal without disrupting traffic on its peers. A drained node hands Raft leadership to another voter if it held it, signals connected radios that it is unavailable so new UEs attach elsewhere, and stops advertising user-plane routes so upstream routing shifts to the survivors. Existing flows keep running until the node is removed or shut down.

Drain is triggered by an operator via the cluster API. Removal requires a drained node.

## Scaling the cluster

New voters join in two steps. The operator registers the node as a non-voter, which lets it catch up on the Raft log without counting toward quorum; once the node has been healthy and up-to-date for a short stabilization window, the cluster automatically promotes it to a voter. Operators who want to promote immediately can call the promote endpoint by hand.

Shrinking is symmetric. Drain the node, then remove it; the remaining voters continue serving writes while the configuration change commits.

## Inter-node communication using mTLS

Every inter-node connection is mutually authenticated over TLS. The cluster runs its own CA, generated at first-leader election and replicated through Raft so any voter can issue certificates once it becomes leader. New nodes join by exchanging a single-use token, minted by an admin from the Cluster page, for a certificate at startup. Certificates are bound to the cluster's identity, so credentials from one cluster cannot authenticate into another.

## Disaster recovery

HA clusters recover from total loss through an offline, backup-driven path. An operator stops every node, seeds one node from a backup archive, and starts it — it comes up as a single-voter cluster carrying the restored state. The remaining voters then rejoin with fresh join tokens. Because the backup archive carries the cluster CA signing material, the restored cluster keeps its original identity. The step-by-step procedure lives in [Backup and Restore](../how_to/backup_and_restore.md).

## Rolling upgrades

Upgrades proceed one node at a time: drain the node, refresh its binary, then resume. Each node retains its node-id, certificate, and Raft membership across the swap. Writes continue throughout; the cluster is briefly mixed-version during each step.

When the new binary carries schema changes, the cluster keeps running on the old schema until every voter has self-announced support; only then does the migration commit through Raft. Migration progress is observable through the status endpoint.

Skip-version upgrades (`vN → vN+2`) and downgrades are not supported.

## Further reading

- [Deploy a High Availability Cluster](../how_to/deploy_ha_cluster.md) — step-by-step guide to bring up a cluster.
- [Scale Up a High Availability Cluster](../how_to/scale_up_ha_cluster.md) — add nodes to an existing cluster.
- [Perform a Rolling Upgrade](../how_to/rolling_upgrade.md) — upgrade every node without taking the cluster offline.
- [Cluster API reference](../reference/api/cluster.md) — cluster management endpoints.
