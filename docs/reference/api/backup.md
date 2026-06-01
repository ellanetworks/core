---
description: RESTful API reference for managing database backups.
---

# Backup

This path creates a backup of the Ella Core database.

The backup archive contains sensitive secrets. Store and transfer it encrypted, and treat it as you would an admin credential.

## Create a Backup

| Method | Path             |
| ------ | ---------------- |
| POST   | `/api/v1/backup` |

### Parameters

None

### Sample Response

The response contains the backup file as a downloadable attachment. 
