---
description: Upgrade a running Ella Core high-availability cluster one node at a time without taking it offline.
---

# Perform a Rolling Upgrade

This guide walks through upgrading every node in a running Ella Core high-availability cluster, one at a time, without taking the cluster offline.

## Prerequisites

- A running cluster deployed via [Deploy a High Availability Cluster](deploy_ha_cluster.md).
- Admin credentials for the Ella Core UI, or an admin API token.
- The target Ella Core version available on the snap channel you track.

## Upgrade one node

Repeat these steps for each node, **upgrading the leader last**.

1. Open the **Cluster** page on any healthy node other than the one you are about to upgrade.
2. Pick the next node to upgrade, a follower, unless this is the last pass.
3. Click **Drain** next to that node. Wait until its **Drain State** is `drained`.
4. On that host, refresh the snap:

    ```shell
    sudo snap refresh ella-core
    ```

5. On the **Cluster** page, wait for the node to return to **Healthy**.
6. Click **Resume** next to the node. Wait for **Drain State** to clear back to `active`.
7. Move to the next node.

## Verify the upgrade

After every node has been refreshed, open the **Cluster** page and confirm:

- Every node's **Version** column shows the target release.
- The mixed-version warning banner is gone.
- Every node is **Healthy** and its **Drain State** is `active`.
