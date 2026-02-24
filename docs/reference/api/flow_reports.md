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

| Name           | In    | Type   | Default | Allowed     | Description                                 |
| -------------- | ----- | ------ | ------- | ----------- | ------------------------------------------- |
| `page`         | query | int    | `1`     | `>= 1`      | 1-based page index.                         |
| `per_page`     | query | int    | `25`    | `1…100`     | Number of items per page.                   |
| `subscriber_id`| query | string | ``      |             | Filter by subscriber ID.                    |
| `protocol`     | query | int    | ``      | `1…255`     | Filter by protocol number.                  |
| `source_ip`    | query | string | ``      |             | Filter by source IP address.                |
| `destination_ip`| query | string| ``      |             | Filter by destination IP address.           |

### Sample Response

```json
{
  "result": {
    "items": [
      {
        "id": 1,
        "subscriber_id": "001019756139935",
        "timestamp": "2025-02-22T10:30:00.000Z",
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
