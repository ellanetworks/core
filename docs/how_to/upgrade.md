---
description: Upgrade Ella Core to the next release, on a standalone node or a high-availability cluster.
---

# Upgrade

## Prerequisites

- Ella Core installed with [Install](install.md) or [Deploy a High Availability Cluster](deploy_ha_cluster.md).
- The target release is the next minor release after the running one.

## 1. Review the release notes

Read the [release notes](../reference/releases/index.md) of the target release. Apply any action listed under **Breaking changes**.

## 2. Back up

Create a backup with [Backup and Restore](backup_and_restore.md#backup).

## 3. Upgrade

=== "Standalone"

    === "Snap"

        ```shell
        sudo snap refresh ella-core
        ```

    === "Docker"

        In `docker-compose.yaml`, set the image tag to the target release:

        ```yaml
        image: ghcr.io/ellanetworks/ella-core:<target-version>
        ```

        Recreate the container:

        ```shell
        docker compose up -d
        ```

    === "Kubernetes"

        Download the manifests of the target release:

        ```bash
        git clone --depth 1 --branch <target-version> https://github.com/ellanetworks/core.git
        ```

        Reapply your changes to `core/k8s/core-ran-nad.yaml` and `core/k8s/core-configmap.yaml`.

        Apply the manifests:

        ```bash
        kubectl apply -k core/k8s
        ```

        Wait for the rollout to complete:

        ```bash
        kubectl -n ella rollout status statefulset/ella-core
        ```

=== "High Availability"

    Upgrade the followers first and the leader last. Repeat these steps for each node:

    1. Open the **Cluster** page on any healthy node other than the one you are about to upgrade.
    2. Click **Drain** next to the node. Wait until its **Drain State** is `drained`.
    3. On that host, refresh the snap:

        ```shell
        sudo snap refresh ella-core
        ```

    4. On the **Cluster** page, wait for the node to return to **Healthy**.
    5. Click **Resume** next to the node. Wait until its **Drain State** is `active`.

## 4. Verify

=== "Standalone"

    ```shell
    curl <ella-core-api-url>/api/v1/status
    ```

    Confirm `version` is the target release and `ready` is `true`.

=== "High Availability"

    On the **Cluster** page, confirm:

    - Every node's **Version** is the target release.
    - Every node is **Healthy** and its **Drain State** is `active`.
    - **Schema** shows a single version.

## Roll back

=== "Standalone"

    1. Reinstall the previous release with [Install](install.md).
    2. Restore the backup with [Offline Restore](backup_and_restore.md#offline-restore-disaster-recovery).

=== "High Availability"

    Follow [Offline Restore](backup_and_restore.md#offline-restore-disaster-recovery) with the previous release installed on every node.
