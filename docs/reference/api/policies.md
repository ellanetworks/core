---
description: RESTful API reference for managing policies.
---

# Policies

## List Policies

This path returns the list of policies.


| Method | Path               |
| ------ | ------------------ |
| GET    | `/api/v1/policies` |

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "name": "default",
            "bitrate-uplink": "200 Mbps",
            "bitrate-downlink": "100 Mbps",
            "var5qi": 8,
            "priority-level": 1,
            "data-network-name": "internet"
        }
    ]
}
```

## Create a Policy

This path creates a new policy.

| Method | Path               |
| ------ | ------------------ |
| POST   | `/api/v1/policies` |

### Parameters

- `name` (string): The Name of the policy.
- `bitrate-uplink` (string): The uplink bitrate of the policy. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `bitrate-downlink` (string): The downlink bitrate of the policy. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `var5qi` (integer): The QoS class identifier of the policy. Must be an integer between 1 and 255.
- `priority-level` (integer): The priority level of the policy. Must be an integer between 1 and 255.
- `data-network-name` (string): The name of the data network associated with the policy. Must be the name of an existing data network.

### Sample Response

```json
{
    "result": {
        "message": "Policy created successfully"
    }
}
```

## Update a Policy

This path updates an existing policy.

| Method | Path                      |
| ------ | ------------------------- |
| PUT    | `/api/v1/policies/{name}` |

### Parameters

- `bitrate-uplink` (string): The uplink bitrate of the policy. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `bitrate-downlink` (string): The downlink bitrate of the policy. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `var5qi` (integer): The QoS class identifier of the policy. Must be an integer between 1 and 255.
- `priority-level` (integer): The priority level of the policy. Must be an integer between 1 and 255.
- `data-network-name` (string): The name of the data network associated with the policy. Must be the name of an existing data network.

### Sample Response

```json
{
    "result": {
        "message": "Policy updated successfully"
    }
}
```

## Get a Policy

This path returns the details of a specific policy.

| Method | Path                      |
| ------ | ------------------------- |
| GET    | `/api/v1/policies/{name}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "name": "my-policy",
        "bitrate-uplink": "10 Mbps",
        "bitrate-downlink": "10 Mbps",
        "var5qi": 1,
        "priority-level": 2,
        "data-network-name": "internet"
    }
}
```

## Delete a Policy

This path deletes a policy from Ella Core.

| Method | Path                      |
| ------ | ------------------------- |
| DELETE | `/api/v1/policies/{name}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "Policy deleted successfully"
    }
}
```
