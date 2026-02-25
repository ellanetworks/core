---
description: RESTful API reference for flow reports.
---

# Flow Reports

Flow reports provide visibility into network traffic patterns and usage statistics for subscribers. These reports are stored in the database and can be queried with various filters and pagination options.

## Get Flow Reports

This path returns a paginated list of flow reports with optional filtering.

| Method | Path                   |
| ------ | ---------------------- |
| GET    | `/api/v1/flow-reports` |

### Query Parameters

| Name           | In    | Type   | Default  | Allowed              | Description                                 |
| -------------- | ----- | ------ | -------- | -------------------- | ------------------------------------------- |
| `page`         | query | int    | `1`      | `>= 1`               | 1-based page index.                         |
| `per_page`     | query | int    | `25`     | `1…100`              | Number of items per page.                   |
| `subscriber_id`| query | string | ``       |                      | Filter by subscriber ID.                    |
| `protocol`     | query | int    | ``       | `1…255`              | Filter by protocol number.                  |
| `source_ip`    | query | string | ``       |                      | Filter by source IP address.                |
| `destination_ip`| query | string| ``       |                      | Filter by destination IP address.           |
| `start`        | query | string | `now-7d` |                      | Start date for flow reports. Format: YYYY-MM-DD. |
| `end`          | query | string | `now`    |                      | End date for flow reports. Format: YYYY-MM-DD. |
| `group_by`     | query | string | ``       | `day`, `subscriber`  | Grouping method for flow reports. When set, returns aggregated data instead of paginated list. |

### Sample Response (default, no `group_by`)

```json
{
  "result": {
    "items": [
      {
        "id": 1,
        "subscriber_id": "001019756139935",
        "source_ip": "192.168.1.100",
        "destination_ip": "8.8.8.8",
        "source_port": 10000,
        "destination_port": 53,
        "protocol": 17,
        "packets": 100,
        "bytes": 5000,
        "start_time": "2025-02-22T10:30:00.000Z",
        "end_time": "2025-02-22T10:30:01.000Z"
      }
    ],
    "page": 1,
    "per_page": 25,
    "total_count": 1
  }
}
```

### Sample Response (`group_by=day`)

```json
{
  "result": [
    {
      "2025-02-22": [
        {
          "id": 1,
          "subscriber_id": "001019756139935",
          "source_ip": "192.168.1.100",
          "destination_ip": "8.8.8.8",
          "source_port": 10000,
          "destination_port": 53,
          "protocol": 17,
          "packets": 100,
          "bytes": 5000,
          "start_time": "2025-02-22T10:30:00.000Z",
          "end_time": "2025-02-22T10:30:01.000Z"
        }
      ]
    },
    {
      "2025-02-23": [
        {
          "id": 2,
          "subscriber_id": "001019756139935",
          "source_ip": "192.168.1.100",
          "destination_ip": "1.1.1.1",
          "source_port": 20000,
          "destination_port": 443,
          "protocol": 6,
          "packets": 800,
          "bytes": 40000,
          "start_time": "2025-02-23T08:00:00.000Z",
          "end_time": "2025-02-23T08:05:00.000Z"
        }
      ]
    }
  ]
}
```

### Sample Response (`group_by=subscriber`)

```json
{
  "result": [
    {
      "001019756139935": [
        {
          "id": 1,
          "subscriber_id": "001019756139935",
          "source_ip": "192.168.1.100",
          "destination_ip": "8.8.8.8",
          "source_port": 10000,
          "destination_port": 53,
          "protocol": 17,
          "packets": 100,
          "bytes": 5000,
          "start_time": "2025-02-22T10:30:00.000Z",
          "end_time": "2025-02-22T10:30:01.000Z"
        }
      ]
    },
    {
      "001019756139936": [
        {
          "id": 3,
          "subscriber_id": "001019756139936",
          "source_ip": "192.168.1.101",
          "destination_ip": "1.1.1.1",
          "source_port": 30000,
          "destination_port": 443,
          "protocol": 6,
          "packets": 500,
          "bytes": 25000,
          "start_time": "2025-02-22T12:00:00.000Z",
          "end_time": "2025-02-22T12:01:00.000Z"
        }
      ]
    }
  ]
}
```

## Clear Flow Reports

This path deletes all flow reports from the database.

| Method | Path                   |
| ------ | ---------------------- |
| DELETE | `/api/v1/flow-reports` |

### Sample Response

```json
{
  "result": {
    "message": "All flow reports cleared successfully"
  }
}
```

## Get Flow Reports Retention Policy

This path returns the current flow reports retention policy.

| Method | Path                           |
| ------ | ------------------------------ |
| GET    | `/api/v1/flow-reports/retention` |

### Sample Response

```json
{
  "result": {
    "days": 7
  }
}
```

## Update Flow Reports Retention Policy

This path updates the flow reports retention policy.

| Method | Path                           |
| ------ | ------------------------------ |
| PUT    | `/api/v1/flow-reports/retention` |

### Parameters

- `days` (integer): The number of days to retain flow reports. Must be a positive integer.

### Sample Response

```json
{
  "result": {
    "message": "Flow reports retention policy updated successfully"
  }
}
```
