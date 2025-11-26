---
description: RESTful API reference for logs.
---

# Audit Logs

In addition to system logs output, Ella Core exposes audit logs through its API. These logs are useful in the day-to-day operation of the network.

## List Audit Logs

This path returns the list of audit logs.

| Method | Path                    |
| ------ | ----------------------- |
| GET    | `/api/v1/logs/audit` |

### Query Parameters

| Name       | In    | Type | Default | Allowed | Description                   |
| ---------- | ----- | ---- | ------- | ------- | ----------------------------- |
| `page`     | query | int  | `1`     | `>= 1`  | 1-based page index.           |
| `per_page` | query | int  | `25`    | `1…100` | Number of items per page.     |

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "id": 1,
                "timestamp": "2025-08-12T16:58:00.810-0400",
                "level": "INFO",
                "actor": "guillaume@ellanetworks.com",
                "action": "create_user",
                "ip": "127.0.0.1",
                "details": "User created user: newuser@ellanetworks.com with role: 1"
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
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
