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
   
    !!! note
        This operation can also be done using the API. Please see the [backup API documentation](../reference/api/backup.md) for more information.

=== "Restore"

    !!! warning
        Restoring a backup will overwrite all existing data in your Ella Core installation.

    On a new installation of Ella Core, you can restore a backup to recover your data.

    1. Open Ella Core in your web browser.
    2. Navigate to the **Backup and Restore** tab in the left-hand menu.
    3. Click on the **Upload File** button.
    4. Select the backup file you want to restore.

    !!! note
        This operation can also be done using the API. Please see the [restore API documentation](../reference/api/restore.md) for more information.

## Disaster recovery for HA clusters

HA backup archives carry the cluster CA signing keys inside `ella.db`.
If every voter is lost, reconstruct the cluster by dropping the bundle
under a fresh data directory before starting the daemon:

```shell
sudo mv backup.tar.gz /var/snap/ella-core/common/restore.bundle
sudo chmod 600 /var/snap/ella-core/common/restore.bundle
sudo snap start --enable ella-core.cored
```

The daemon extracts the bundle on first start, deletes it, and comes
up as a single-node cluster. Add the remaining nodes via the usual
[join-token flow](deploy_ha_cluster.md).

Backup bundles carry signing material — protect them at rest like the
data directory itself.