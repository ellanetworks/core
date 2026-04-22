---
description: Explanation of how high availability works in Ella Core.
---

# High Availability (beta)

!!! info "Beta feature"
    High availability is currently in beta. It is available for testing and feedback in the `main` branch but not recommended for production use yet. Expect breaking changes as we iterate on the design and implementation.

High availability (HA) lets you run an Ella Core cluster so that the network keeps working when one node fails.

HA in Ella Core rests on two pillars. Consensus is handled by the [Raft algorithm](https://raft.github.io/): at any time one node is the leader, it is the only node that accepts writes, and every write replicates to a majority of nodes before it is considered committed. Inter-node traffic — Raft replication and the cluster HTTP port that carries the follower proxy — is mutually authenticated over TLS using certificates issued by a cluster CA that Ella Core generates itself on first-leader election.

A follower that receives a write request forwards it to the leader transparently, and waits until its own database has applied the commit before returning — so a subsequent read on the same follower sees its own write.

<figure markdown="span">
  ![Ella Core HA cluster](../images/ha_raft.svg){ width="700" }
  <figcaption>High Availability in Ella Core</figcaption>
</figure>

## Cluster size and quorum

Deploy three or five nodes. A quorum is a majority of voters: 2 of 3, or 3 of 5. Three nodes tolerate one failure; five nodes tolerate two. Even-sized clusters offer no additional fault tolerance over N−1 and should be avoided.

If the cluster loses quorum — for example, two of three nodes down simultaneously — writes on the survivor stall and return `503 Service Unavailable`; reads continue. Once enough nodes return to restore a majority, writes resume automatically; no operator intervention is needed.

## What replicates, and what does not

All persistent resources are replicated across the cluster, so if a node dies, the others have the same subscribers, policies, and operator configuration. The cluster automatically elects a new leader and keeps accepting operator changes with no manual intervention.

Runtime state tied to a specific connection or session does not replicate. This includes SCTP associations with gNBs, UE contexts, active PDU sessions and their User Plane state, GTP-U tunnels, and active BGP adjacencies.

Observability is also per-node. Each Ella Core instance exposes its own Prometheus endpoint at `/api/v1/metrics` and keeps its own `radio_events` and `flow_reports` tables. Operators scrape each node individually for a cluster-wide view. Audit logs are the exception: they replicate like other operator data.

Cluster health itself is not per-node. The leader continuously assesses every peer — reachability, applied-index lag, overall state, failure tolerance — and exposes the result at `/api/v1/cluster/autopilot`. The Cluster page in the UI renders this.

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

Draining prepares a node for removal without disrupting traffic on its peers. A drained node hands Raft leadership to another voter if it held it, signals connected radios that it is unavailable so new UEs attach elsewhere, and stops advertising user-plane routes so upstream routing shifts to the survivors. Existing flows keep running until the node is removed or shut down. A node receiving a shutdown signal (SIGTERM) also marks itself `drained` as part of a clean shutdown. Removal requires a drained node.

## Scaling the cluster

New voters join in two steps. The operator registers the node as a non-voter, which lets it catch up on the Raft log without counting toward quorum; once the node has been healthy and up-to-date for a short stabilization window, the cluster automatically promotes it to a voter. Operators who want to promote immediately can call the promote endpoint by hand.

Shrinking is symmetric. Drain the node, then remove it; the remaining voters continue serving writes while the configuration change commits.

## Inter-node communication using mTLS

Every inter-node connection is mutually authenticated over TLS 1.3.

On first-leader election, the cluster generates its own root CA and an intermediate, and signs itself a leaf. Private keys live on each voter's disk; public certs and a join-token HMAC key are replicated through Raft so every voter can validate chains.

Additional nodes join via a single-use HMAC token. An admin mints one through the Cluster page; the joining node puts it in its `cluster.join-token` config field and exchanges it for a leaf over a dedicated bootstrap ALPN.

Leaves are short-lived (24 hours). Each node renews its own leaf automatically at 60–90 % of the TTL, jittered. Removing a cluster member revokes every leaf it holds; the in-memory revocation cache on every voter stops accepting those certs within seconds.

Every leaf's URI SAN is `spiffe://cluster.ella/<cluster-id>/node/<n>`; the local cluster ID is checked at every handshake, so a leaf from one cluster cannot authenticate into another.

The CA private keys are part of the data directory. Back them up — see [Backup and Restore](../how_to/backup_and_restore.md).

## Rolling upgrades

Upgrades proceed one node at a time: drain, remove, upgrade, rejoin as non-voter, wait for auto-promotion (or promote manually). Writes continue throughout; the cluster is briefly mixed-version during each swap.

To keep a mixed-version cluster consistent, the leader applies a schema migration only once every voter's binary supports it, and refuses to admit a new voter whose schema trails what the cluster has already applied. Skip-version upgrades (`vN → vN+2`) and downgrades are not supported.

## Further reading

- [Deploy a High Availability Cluster](../how_to/deploy_ha_cluster.md) — step-by-step guide to bring up a cluster.
- [Scale Up a High Availability Cluster (beta)](../how_to/scale_up_ha_cluster.md) — add nodes to an existing cluster.
- [Cluster API reference](../reference/api/cluster.md) — cluster management endpoints.
