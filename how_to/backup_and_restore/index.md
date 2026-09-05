# Backup and Restore

Ella Core stores all persistent data in an embedded database. You can create backups of this database to protect your data and restore it in case of data loss.

1. Open Ella Core in your web browser.
1. Navigate to the **Backup and Restore** tab in the left-hand menu.
1. Click on the **Backup** button.
1. The backup file will be downloaded to your computer. Store this file in a safe location.

Warning

The backup archive contains sensitive secrets. Store and transfer it encrypted, and treat it as you would an admin credential.

Note

This operation can also be done using the API. Please see the [backup API documentation](https://docs.ellanetworks.com/reference/api/backup/index.md) for more information.

Warning

Restoring a backup will overwrite all existing data in your Ella Core installation. This path is **disabled in HA mode**. Clustered deployments use the disaster-recovery flow described below.

On a new installation of Ella Core, you can restore a backup to recover your data.

1. Open Ella Core in your web browser.
1. Navigate to the **Backup and Restore** tab in the left-hand menu.
1. Click on the **Upload File** button.
1. Select the backup file you want to restore.

Note

This operation can also be done using the API. Please see the [restore API documentation](https://docs.ellanetworks.com/reference/api/restore/index.md) for more information.

## Disaster recovery for HA clusters

Paths below assume the default data directory `/var/snap/ella-core/common/data` (the directory holding `db.path`).

1. Stop the daemon on every node in the cluster:

   ```
   sudo snap stop ella-core.cored
   ```

1. On the node you seed from the backup, delete the old cluster state:

   ```
   sudo rm -rf /var/snap/ella-core/common/data/ella.db \
               /var/snap/ella-core/common/data/ella.db-wal \
               /var/snap/ella-core/common/data/ella.db-shm \
               /var/snap/ella-core/common/data/raft \
               /var/snap/ella-core/common/data/cluster-tls
   ```

1. Remove `cluster.join-token` from that node's `core.yaml` if it is set.

1. Drop the backup archive into the data directory as `restore.bundle`:

   ```
   sudo mv backup.tar.gz /var/snap/ella-core/common/data/restore.bundle
   sudo chmod 600 /var/snap/ella-core/common/data/restore.bundle
   ```

1. Start the daemon on that node:

   ```
   sudo snap start --enable ella-core.cored
   ```

1. On each remaining node, repeat step 2, then add it via the [join-token flow](https://docs.ellanetworks.com/how_to/deploy_ha_cluster/index.md).
