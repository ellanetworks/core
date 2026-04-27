---
description: How rolling upgrades work in an Ella Core HA cluster.
---

# Rolling Upgrade

!!! info "Beta feature"
    HA is currently in beta. The rolling-upgrade contract described here is enforced by code today; the integration tests that validate it end-to-end are still pending. Expect breaking changes as we iterate.

A rolling upgrade replaces every Ella Core node in an HA cluster with a newer binary, one at a time, without taking the whole cluster offline. The cluster keeps accepting writes throughout — the leader stays up, followers serve reads, and the data path keeps forwarding traffic.

## Supported upgrade paths

- **Forward, one minor version at a time** — `vN → vN+1`. The cluster must reach `vN+1` everywhere before any node moves to `vN+2`.
- **Skip-version upgrades and binary downgrades are not supported.** Both can leave the cluster with a node holding state it doesn't understand. The contract assumes operators upgrade in order.

## What the operator sees

Every node exposes its rolling-upgrade state on `GET /api/v1/status`. Two fields under `cluster` are relevant:

- `appliedSchemaVersion` — the schema version the cluster has committed. Every node returns the same number once a migration has replicated.
- `pendingMigration` — present only when there is an unfinished migration. Shape:
  ```json
  {
    "currentSchema": 9,
    "targetSchema": 10,
    "laggardNodeId": 3
  }
  ```
  - `currentSchema` is the version applied today.
  - `targetSchema` is the highest version the cluster could move to right now, bounded by the local binary and every voter's `MaxSchemaVersion`.
  - `laggardNodeId`, when non-zero, identifies the voter holding the migration up — its `MaxSchemaVersion` is below `targetSchema`. Upgrade that node next.

The top-level `schemaVersion` field on the same response continues to mean *this binary's* maximum supported version. During a rolling upgrade window, `appliedSchemaVersion ≤ schemaVersion` for newly-upgraded nodes, and they're equal once every voter is on the new binary.

## What happens internally

When a v(N+1) binary starts on a v=N database, it does not run schema migrations locally. Migrations beyond `baselineVersion` are proposed through Raft by the leader once every voter's `MaxSchemaVersion` reaches the target. The newly-started node:

1. Self-announces `MaxSchemaVersion = N+1` to the leader.
2. Joins as a follower.
3. Operates on the v=N schema until the leader proposes `CmdMigrateShared(N+1)` — which only happens when *every* voter supports it.
4. Applies the migration when it arrives, in lockstep with the rest of the cluster.

While the cluster is mid-window, writes that depend on the new schema return `503 Service Unavailable` (`schema migration pending`). Clients retry; once the migration applies, the writes succeed.

## Recommended procedure

1. Drain and stop one node.
2. Start the new binary on the same data directory.
3. Watch `cluster.appliedSchemaVersion` and `cluster.pendingMigration` on `/api/v1/status` from any node.
4. When `pendingMigration` clears for that node (or `laggardNodeId` moves to the next), repeat with the next node.
5. After the last node restarts on the new binary, the leader proposes the pending migrations and `appliedSchemaVersion` advances cluster-wide.

If a node crashes mid-upgrade, restart it on either the old or new binary — both can join the cluster as long as the schema-version handshake passes (joiner's max ≥ leader's applied; leader's max ≥ joiner's applied).
