# Backup and Restore

Ella Core stores all persistent data in an embedded database. You can create backups of this database to protect your data and restore it in case of data loss.

1. Open Ella Core in your web browser.
1. Navigate to the **Backup and Restore** tab in the left-hand menu.
1. Click on the **Backup** button.
1. The backup file will be downloaded to your computer. Store this file in a safe location.

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

1. Stop every voter in the cluster.

1. On one node, drop the backup archive into the data directory as `restore.bundle`:

   ```
   sudo mv backup.tar.gz /var/snap/ella-core/common/restore.bundle
   sudo chmod 600 /var/snap/ella-core/common/restore.bundle
   ```

1. Start the daemon on that node:

   ```
   sudo snap start --enable ella-core.cored
   ```

1. Add the remaining nodes via the [join-token flow](https://docs.ellanetworks.com/how_to/deploy_ha_cluster/index.md).
