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

## List Network Logs

This path returns the list of network logs.

| Method | Path                         |
| ------ | ---------------------------- |
| GET    | `/api/v1/logs/networks`   |

### Query Parameters

| Name             | In    | Type | Default | Allowed           | Description                                                                                     |
| ----------       | ----- | ---- | ------- | ----------------- | ----------------------------------------------------------------------------------------------- |
| `page`           | query | int  | `1`     | `>= 1`            | 1-based page index.                                                                             |
| `per_page`       | query | int  | `25`    | `1…100`           | Number of items per page.                                                                       |
| `protocol`       | query | str  |         |                   | Filter by protocol.                                                                              |
| `direction`      | query | str  |         | inbound, outbound | Filter by log direction.                                                                        |
| `message_type`   | query | str  |         |                   | Filter by message type.                                                                          |
| `timestamp_from` | query | str  |         |                   | Filter logs from this timestamp (inclusive). RFC3339 format (e.g., 2006-01-02T15:04:05Z07:00).  |
| `timestamp_to`   | query | str  |         |                   | Filter logs up to this timestamp (inclusive). RFC3339 format (e.g., 2006-01-02T15:04:05Z07:00). |

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "id": 1,
                "timestamp": "2025-08-12T16:58:00.810-0400",
                "protocol": "NGAP",
                "message_type": "PDU Session Establishment Accept",
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

## Update Network Log Retention Policy

This path updates the network log retention policy.

| Method | Path                           |
| ------ | ------------------------------ |
| PUT    | `/api/v1/logs/networks/retention` |

### Parameters

- `days` (integer): The number of days to retain network logs. Must be a positive integer.

### Sample Response

```json
{
    "result": {
        "message": "Network log retention policy updated successfully"
    }
}
```

## Clear Network Logs

This path deletes all network logs.

| Method | Path                         |
| ------ | ---------------------------- |
| DELETE | `/api/v1/logs/networks`   |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "All network logs have been deleted successfully"
    }
}
```

## Get Network Log Retention Policy

This path returns the current network log retention policy.

| Method | Path                           |
| ------ | ------------------------------ |
| GET    | `/api/v1/logs/networks/retention` |

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
