# Perform a Rolling Upgrade

This guide walks through upgrading every node in a running Ella Core high-availability cluster, one at a time, without taking the cluster offline.

## Prerequisites

- A running cluster deployed via [Deploy a High Availability Cluster](https://docs.ellanetworks.com/how_to/deploy_ha_cluster/index.md).
- A new version of `ella-core` available.

## Upgrade one node

Repeat these steps for each node.

1. Open the **Cluster** page on any healthy node other than the one you are about to upgrade.

1. Click **Drain** next to the node you want to upgrade. Wait until its **Drain State** is `drained`.

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
- Every node is **Healthy** and its **Drain State** is `active`.
- **Schema** shows a single version.
