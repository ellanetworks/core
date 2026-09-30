---
description: Step-by-step instructions to backup and restore Ella Core.
---

# Backup and Restore

Ella Core stores all persistent data in an embedded database. You can create backups of this database to protect your data and restore it in case of data loss.

## Backup

1. Open Ella Core in your web browser.
2. Navigate to the **Backup and Restore** tab in the left-hand menu.
3. Click on the **Backup** button.
4. The backup file will be downloaded to your computer. It contains sensitive data, store and transfer it encrypted.

!!! note
    This operation can also be done using the API. Please see the [backup API documentation](../reference/api/backup.md) for more information.

## Restore on same instance

Restoring on the same instance will overwrite all existing data in your Ella Core installation. It is only supported on standalone deployments. For High Availability deployment, see the disaster recovery procedure.

1. Open Ella Core in your web browser.
2. Navigate to the **Backup and Restore** tab in the left-hand menu.
3. Click on the **Upload File** button.
4. Select the backup file you want to restore.

!!! note
    This operation can also be done using the API. Please see the [restore API documentation](../reference/api/restore.md) for more information.

## Offline Restore (Disaster Recovery)

This procedure can be used on new instances.

Paths below assume the default data directory `/var/snap/ella-core/common/data` (the directory holding `db.path`).

1. Stop the daemon on every node in the cluster, or the single standalone node:

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

3. Drop the backup archive into the data directory as `restore.bundle`:

    ```shell
    sudo cp backup.tar.gz /var/snap/ella-core/common/data/restore.bundle
    sudo chmod 600 /var/snap/ella-core/common/data/restore.bundle
    ```

4. Start the daemon on that node:

    ```shell
    sudo snap start --enable ella-core.cored
    ```

5. For a Standalone deployment, stop here.

6. On each remaining node, repeat step 2, then add it via the [join-token flow](deploy_ha_cluster.md).
