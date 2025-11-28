---
description: RESTful API reference for managing subscriber usage data.
---

# Usage

This section describes the RESTful API for managing subscriber usage data.

## Get Subscriber Usage

This path retrieves usage data for network subscribers.

| Method | Path                  |
| ------ | --------------------- |
| GET    | `/api/v1/subscriber-usage` |

### Query Parameters

| Name       | In    | Type | Default | Allowed | Description                   |
| ---------- | ----- | ---- | ------- | ------- | ----------------------------- |
| `start`      | query | string  | `now-7d` |         | Start date for usage data. Format: YYYY-MM-DD.   |
| `end`        | query | string  | `now`    |         | End date for usage data. Format: YYYY-MM-DD.     |
| `group_by`   | query | string  | `day`    | `day`, `subscriber` | Grouping method for usage data.  |
| `subscriber` | query | string  | ``     |          | Filter usage data for a specific subscriber.     |

### Sample Response

```json
{
  "result": [
    {
      "2025-02-22": {
        "uplink_bytes": 1048576,
        "downlink_bytes": 2097152,
        "total_bytes": 3145728
      }
    },
    {
      "2025-02-23": {
        "uplink_bytes": 524288,
        "downlink_bytes": 1048576,
        "total_bytes": 1572864
      }
    }
  ]
}
```

## Clear Subscriber Usage

This path clears usage data for all network subscribers.

| Method | Path                  |
| ------ | --------------------- |
| DELETE | `/api/v1/subscriber-usage` |

### Sample Response

```json
{
    "result": {
        "message": "All subscriber usage cleared successfully"
    }
}
```

## Get Subscriber Usage Retention Policy

This path returns the current subscriber usage retention policy.

| Method | Path                                    |
| ------ | --------------------------------------- |
| GET    | `/api/v1/subscriber-usage/retention` |

### Sample Response

```json
{
    "result": {
        "days": 365
    }
}
```
## Update Subscriber Usage Retention Policy

This path updates the subscriber usage retention policy.

| Method | Path                           |
| ------ | ------------------------------ |
| PUT    | `/api/v1/subscriber-usage/retention` |

### Parameters

- `days` (integer): The number of days to retain subscriber usage data. Must be a positive integer.

### Sample Response

```json
{
    "result": {
        "message": "Subscriber usage retention policy updated successfully"
    }
}
```
