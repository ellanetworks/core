# Perform a Rolling Upgrade

This guide walks through upgrading every node in a running Ella Core high-availability cluster, one at a time, without taking the cluster offline. For background on mixed-version clusters, draining, and schema coordination, see [High Availability](https://docs.ellanetworks.com/explanation/high_availability/index.md).

## Prerequisites

- A running cluster deployed via [Deploy a High Availability Cluster](https://docs.ellanetworks.com/how_to/deploy_ha_cluster/index.md).
- Admin credentials for the Ella Core UI, or an admin API token.
- The target Ella Core version available on the snap channel you track.

## Upgrade one node

Repeat these steps for each node, **upgrading the leader last**.

1. Identify the leader on the **Cluster** page of any healthy node.

1. Pick the next node to upgrade — a follower, unless this is the last pass.

1. Click **Drain** next to that node. Wait until its **Drain State** is `drained`.

1. On that host, refresh the snap:

   ```
   sudo snap refresh ella-core
   ```

1. On the **Cluster** page, wait for the node to return to **Healthy**.

1. Click **Resume** next to the node. Wait for **Drain State** to clear back to `active`.

1. Move to the next node.

## Verify the upgrade

After every node has been refreshed, open the **Cluster** page and confirm:

- Every node's **Version** column shows the target release.
- The mixed-version warning banner is gone.
- Every node is **Healthy** and its **Drain State** is `active`.

Note

All steps in this guide can also be performed via the REST API. See the [Cluster API reference](https://docs.ellanetworks.com/reference/api/cluster/index.md) for details.
