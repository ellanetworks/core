---
description: Upgrade a running Ella Core high-availability cluster one node at a time without taking it offline.
---

# Perform a Rolling Upgrade

This guide walks through upgrading every node in a running Ella Core high-availability cluster, one at a time, without taking the cluster offline. For background on mixed-version clusters, draining, and schema coordination, see [High Availability](../explanation/high_availability.md).

## Prerequisites

- A running cluster deployed via [Deploy a High Availability Cluster](deploy_ha_cluster.md).
- Admin credentials for the Ella Core UI, or an admin API token.
- The target Ella Core version available on the snap channel you track.

## Upgrade one node

Repeat these steps for each node, **upgrading the leader last**.

1. Identify the leader on the **Cluster** page of any healthy node.
2. Pick the next node to upgrade — a follower, unless this is the last pass.
3. Click **Drain** next to that node. Wait until its **Drain State** is `drained`.
4. On that host, refresh the snap:

    ```shell
    sudo snap refresh ella-core
    ```

5. On the **Cluster** page, wait for the node to return to **Healthy**.
6. Click **Resume** next to the node. Wait for **Drain State** to clear back to `active`.
7. Move to the next node.

## Verify the upgrade

After every node has been refreshed, confirm on `GET /api/v1/status` from each node:

- `version` and `revision` match the target release.
- `cluster.appliedSchemaVersion` equals the top-level `schemaVersion`.
- `cluster.pendingMigration` is absent.

If `cluster.pendingMigration` is still present, the `laggardNodeId` field identifies the node that must be upgraded next.

!!! note
    All steps in this guide can also be performed via the REST API. See the [Cluster API reference](../reference/api/cluster.md) for details.
