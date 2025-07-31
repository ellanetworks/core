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
            "name": "default-default",
            "ue-ip-pool": "172.250.0.0/24",
            "dns": "8.8.8.8",
            "bitrate-uplink": "200 Mbps",
            "bitrate-downlink": "100 Mbps",
            "var5qi": 8,
            "priority-level": 1
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
- `ue-ip-pool` (string): The IP pool of the policy in CIDR notation. Example: `172.250.0.0/24`.
- `dns` (string): The IP address of the DNS server of the policy. Example: `8.8.8.8`.
- `mtu` (integer): The MTU of the policy. Must be an integer between 0 and 65535.
- `bitrate-uplink` (string): The uplink bitrate of the policy. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `bitrate-downlink` (string): The downlink bitrate of the policy. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `var5qi` (integer): The QoS class identifier of the policy. Must be an integer between 1 and 255.
- `priority-level` (integer): The priority level of the policy. Must be an integer between 1 and 255.

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

- `ue-ip-pool` (string): The IP pool of the policy in CIDR notation. Example: `172.250.0.0/24`.
- `dns` (string): The IP address of the DNS server of the policy. Example: `8.8.8.8`.
- `mtu` (integer): The MTU of the policy. Must be an integer between 0 and 65535.
- `bitrate-uplink` (string): The uplink bitrate of the policy. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `bitrate-downlink` (string): The downlink bitrate of the policy. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `var5qi` (integer): The QoS class identifier of the policy. Must be an integer between 1 and 255.
- `priority-level` (integer): The priority level of the policy. Must be an integer between 1 and 255.


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
        "ue-ip-pool": "0.0.0.0/24",
        "dns": "8.8.8.8",
        "mtu": 1460,
        "bitrate-uplink": "10 Mbps",
        "bitrate-downlink": "10 Mbps",
        "var5qi": 1,
        "priority-level": 2
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
