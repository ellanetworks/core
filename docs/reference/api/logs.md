---
description: RESTful API reference for logs.
---

# Logs

In addition to system logs output, Ella Core exposes some logs through its API. These logs are useful in the day-to-day operation of the network.

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

## List Subscriber Logs

This path returns the list of subscriber logs.

| Method | Path                         |
| ------ | ---------------------------- |
| GET    | `/api/v1/logs/subscribers`   |

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
                "imsi": "001010100007487",
                "event": "PDU Session Establishment Accept",
                "direction": "inbound",
                "raw": "ABUAOQAABAAbAAkAAPEQMAASNFAAUkAMBIBnbmIwMDEyMzQ1AGYAEAAAAAABAADxEAAAEAgQIDAAFUABQA",
                "details": "{\"pduSessionID\":1}"
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
}
```

## Update Subscriber Log Retention Policy

This path updates the subscriber log retention policy.

| Method | Path                           |
| ------ | ------------------------------ |
| PUT    | `/api/v1/logs/subscribers/retention` |

### Parameters

- `days` (integer): The number of days to retain subscriber logs. Must be a positive integer.

### Sample Response

```json
{
    "result": {
        "message": "Subscriber log retention policy updated successfully"
    }
}
```

## Clear Subscriber Logs

This path deletes all subscriber logs.

| Method | Path                         |
| ------ | ---------------------------- |
| DELETE | `/api/v1/logs/subscribers`   |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "All subscriber logs have been deleted successfully"
    }
}
```

## Get Subscriber Log Retention Policy

This path returns the current subscriber log retention policy.

| Method | Path                           |
| ------ | ------------------------------ |
| GET    | `/api/v1/logs/subscribers/retention` |

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

## List Radio Logs

This path returns the list of radio logs.

| Method | Path                    |
| ------ | ----------------------- |
| GET    | `/api/v1/logs/radio`   |

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
                "level": "info",
                "ran_id": "001:01:000008",
                "event": "PDU Session Resource Setup Response",
                "direction": "outbound",
                "raw": "ABUAOQAABAAbAAkAAPEQMAASNFAAUkAMBIBnbmIwMDEyMzQ1AGYAEAAAAAABAADxEAAAEAgQIDAAFUABQA",
                "details": "{\"ranID\":\"001:01:000008\",\"ranIP\":\"192.168.40.14:9487\",\"ranName\":\"my ran name\"}"
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
}
```

## Clear Radio Logs

This path deletes all radio logs.

| Method | Path                    |
| ------ | ----------------------- |
| DELETE | `/api/v1/logs/radio`   |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "All radio logs have been deleted successfully"
    }
}
```

## Update Radio Log Retention Policy

```json
{
    "days": 30
}
```

### Sample Response

```json
{
    "result": {
        "message": "Radio log retention policy updated successfully"
    }
}
```

## Get Radio Log Retention Policy

This path returns the current radio log retention policy.

| Method | Path                           |
| ------ | ------------------------------ |
| GET    | `/api/v1/logs/radio/retention` |

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
