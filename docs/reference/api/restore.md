---
description: RESTful API reference for managing database restores.
---

# Restore

This path restores the database from a provided backup file. The backup file must be uploaded as part of the request.

!!! warning "Standalone only"
    Online restore is available in standalone deployments only. Clustered deployments reject this endpoint with `409 Conflict`; use the offline `restore.bundle` disaster-recovery flow instead. See [Backup and Restore](../../how_to/backup_and_restore.md).

## Restore a Backup

| Method | Path              |
| ------ | ----------------- |
| POST   | `/api/v1/restore` |

### Parameters

- `backup` (file): The backup file to restore the database from. It must be a valid backup of the database.

### Sample Response

```json
{
    "result": {
        "message": "Database restored successfully"
    }
}
```
