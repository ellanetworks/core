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
            "actor": "guillaume@ellanetworks.com",
            "action": "create_user",
            "ip": "127.0.0.1",
            "details": "User created user: newuser@ellanetworks.com with role: 1"
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

## List Subscriber Logs

This path returns the list of subscriber logs.

| Method | Path                         |
| ------ | ---------------------------- |
| GET    | `/api/v1/logs/subscribers`   |

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "id": 1,
            "timestamp": "2025-08-12T16:58:00.810-0400",
            "imsi": "001010100007487",
            "event": "PDU Session Establishment Accept",
            "details": "{\"pduSessionID\":1}"
        }
    ]
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
