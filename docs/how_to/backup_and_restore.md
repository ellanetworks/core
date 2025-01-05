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
        This operation can also be done using the API. Please see the [backup API documentation](../reference/api/restore.md) for more information.