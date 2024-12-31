# Restore

This path restores the database from a provided backup file. The backup file must be uploaded as part of the request.

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