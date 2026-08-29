---
description: Step-by-step instructions to backup and restore Ella Core.
---

# Backup and Restore

Ella Core stores all persistent data in an embedded database. You can create backups of this database to protect your data and restore it in case of data loss.

=== "Backup"

    1. Open Ella Core in your web browser.
    2. Navigate to the **Backup and Restore** tab in the left-hand menu.
    3. Click on the **Backup** button.
    4. The backup file will be downloaded to your computer. Store this file in a safe location.

    !!! warning
        The backup archive contains sensitive secrets. Store and transfer it encrypted, and treat it as you would an admin credential.

    !!! note
        This operation can also be done using the API. Please see the [backup API documentation](../reference/api/backup.md) for more information.

=== "Restore (standalone only)"

    !!! warning
        Restoring a backup will overwrite all existing data in your Ella Core installation. This path is **disabled in HA mode**. Clustered deployments use the disaster-recovery flow described below.

    On a new installation of Ella Core, you can restore a backup to recover your data.

    1. Open Ella Core in your web browser.
    2. Navigate to the **Backup and Restore** tab in the left-hand menu.
    3. Click on the **Upload File** button.
    4. Select the backup file you want to restore.

    !!! note
        This operation can also be done using the API. Please see the [restore API documentation](../reference/api/restore.md) for more information.

## Disaster recovery for HA clusters

Paths below assume the default data directory `/var/snap/ella-core/common/data` (the directory holding `db.path`).

1. Stop the daemon on every node in the cluster:

    ```shell
    sudo snap stop ella-core.cored
    ```

2. On the node you seed from the backup, delete the old cluster state:

    ```shell
    sudo rm -rf /var/snap/ella-core/common/data/ella.db \
                /var/snap/ella-core/common/data/ella.db-wal \
                /var/snap/ella-core/common/data/ella.db-shm \
                /var/snap/ella-core/common/data/raft \
                /var/snap/ella-core/common/data/cluster-tls
    ```

3. Remove `cluster.join-token` from that node's `core.yaml` if it is set.

4. Drop the backup archive into the data directory as `restore.bundle`:

    ```shell
    sudo mv backup.tar.gz /var/snap/ella-core/common/data/restore.bundle
    sudo chmod 600 /var/snap/ella-core/common/data/restore.bundle
    ```

5. Start the daemon on that node:

    ```shell
    sudo snap start --enable ella-core.cored
    ```

6. On each remaining node, repeat step 2, then add it via the [join-token flow](deploy_ha_cluster.md).
