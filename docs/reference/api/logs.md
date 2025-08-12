---
description: RESTful API reference for logs.
---

# Logs

## List Audit Logs

This path returns the list of audit logs.

| Method | Path                    |
| ------ | ----------------------- |
| GET    | `/api/v1/logs/audit` |

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "id": 1,
            "timestamp": "2025-08-12T16:58:00.810-0400",
            "level": "info",
            "actor": "First User",
            "action": "create_user",
            "ip": "127.0.0.1",
            "details": "User created user: admin@ellanetworks.com with role: 1"
        }
    ]
}
```

## Update Audit Log Retention Policy

This path update the audit log retention policy.

| Method | Path                           |
| ------ | ------------------------------ |
| PUT    | `/api/v1/logs/audit/retention` |

### Parameters

- `days` (integer): The number of days to retain audit logs. Must be a positive integer.

### Sample Response

```json
{
    "result": {
        "message": "Audit log retention policy updated successfully"
    }
}
```

## Get Audit Log Retention Policy

This path returns the current audit log retention policy.

| Method | Path                           |
| ------ | ------------------------------ |
| GET    | `/api/v1/logs/audit/retention` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "days": 30
    }
}
```
