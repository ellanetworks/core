---
description: RESTful API reference for managing policies.
---

# Policies

## List Policies

This path returns the list of policies.


| Method | Path               |
| ------ | ------------------ |
| GET    | `/api/v1/policies` |

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
                "name": "default",
                "bitrate_uplink": "200 Mbps",
                "bitrate_downlink": "100 Mbps",
                "var5qi": 9,
                "arp": 1,
                "data_network_name": "internet"
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
}
```

## Create a Policy

This path creates a new policy. Optionally, you can create network rules as part of the policy.

| Method | Path               |
| ------ | ------------------ |
| POST   | `/api/v1/policies` |

### Parameters

- `name` (string): The Name of the policy.
- `bitrate_uplink` (string): The uplink bitrate of the policy. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `bitrate_downlink` (string): The downlink bitrate of the policy. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `var5qi` (integer): The QoS class identifier of the policy. Must be an integer between 1 and 255.
- `arp` (integer): The Allocation and Retention Priority (ARP) of the policy. Must be an integer between 1 and 15.
- `data_network_name` (string): The name of the data network associated with the policy. Must be the name of an existing data network.
- `rules` (object, optional): Network rules to create with the policy, organized by direction. Rules are created in the order provided.

### Rules Object Structure

The `rules` object contains:
- `uplink` (array, optional): Array of uplink rules
- `downlink` (array, optional): Array of downlink rules

Each rule contains:
- `description` (string): Description of the rule
- `remote_prefix` (string, optional): CIDR notation for remote prefix (e.g., "10.0.0.0/24") or null
- `protocol` (integer): Protocol number (0-255)
- `port_low` (integer): Low port number (0-65535)
- `port_high` (integer): High port number (0-65535)
- `action` (string): "allow" or "deny"

### Sample Request with Rules

```json
{
    "name": "my-policy",
    "bitrate_uplink": "100 Mbps",
    "bitrate_downlink": "200 Mbps",
    "var5qi": 9,
    "arp": 1,
    "data_network_name": "internet",
    "rules": {
        "uplink": [
            {
                "description": "Allow HTTP/HTTPS",
                "protocol": 6,
                "port_low": 80,
                "port_high": 443,
                "action": "allow"
            }
        ],
        "downlink": [
            {
                "description": "Allow DNS",
                "protocol": 17,
                "port_low": 53,
                "port_high": 53,
                "action": "allow"
            }
        ]
    }
}
```

### Sample Response

```json
{
    "result": {
        "message": "Policy created successfully"
    }
}
```

## Update a Policy

This path updates an existing policy. You can optionally update network rules as part of the policy update.

| Method | Path                      |
| ------ | ------------------------- |
| PUT    | `/api/v1/policies/{name}` |

### Parameters

- `bitrate_uplink` (string): The uplink bitrate of the policy. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `bitrate_downlink` (string): The downlink bitrate of the policy. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `var5qi` (integer): The QoS class identifier of the policy. Must be an integer between 1 and 255.
- `arp` (integer): The Allocation and Retention Priority (ARP) of the policy. Must be an integer between 1 and 15.
- `data_network_name` (string): The name of the data network associated with the policy. Must be the name of an existing data network.
- `rules` (object, optional): Network rules to replace existing rules. If provided, all existing rules are deleted and replaced with the new ones. Omit this field to keep existing rules unchanged.

### Rules Behavior

- **Omit `rules` field**: Existing rules remain unchanged
- **Provide `rules` with arrays**: Existing rules are deleted and replaced with new ones
- **Delete all rules**: Provide empty arrays: `{"uplink": [], "downlink": []}`

### Sample Request to Update Rules

```json
{
    "bitrate_uplink": "100 Mbps",
    "bitrate_downlink": "200 Mbps",
    "var5qi": 9,
    "arp": 1,
    "data_network_name": "internet",
    "rules": {
        "uplink": [
            {
                "description": "Allow SSH",
                "protocol": 6,
                "port_low": 22,
                "port_high": 22,
                "action": "allow"
            }
        ],
        "downlink": []
    }
}
```

### Sample Request to Delete All Rules

```json
{
    "bitrate_uplink": "100 Mbps",
    "bitrate_downlink": "200 Mbps",
    "var5qi": 9,
    "arp": 1,
    "data_network_name": "internet",
    "rules": {
        "uplink": [],
        "downlink": []
    }
}
```

### Sample Response

```json
{
    "result": {
        "message": "Policy updated successfully"
    }
}
```

## Get a Policy

This path returns the details of a specific policy, including any associated network rules.

| Method | Path                      |
| ------ | ------------------------- |
| GET    | `/api/v1/policies/{name}` |

### Parameters

None

### Sample Response with Rules

```json
{
    "result": {
        "name": "my-policy",
        "bitrate_uplink": "10 Mbps",
        "bitrate_downlink": "10 Mbps",
        "var5qi": 9,
        "arp": 1,
        "data_network_name": "internet",
        "rules": {
            "uplink": [
                {
                    "description": "Allow HTTP/HTTPS",
                    "protocol": 6,
                    "port_low": 80,
                    "port_high": 443,
                    "action": "allow"
                }
            ],
            "downlink": [
                {
                    "description": "Allow DNS",
                    "protocol": 17,
                    "port_low": 53,
                    "port_high": 53,
                    "action": "allow"
                }
            ]
        }
    }
}
```

### Sample Response without Rules

If a policy has no associated rules, the `rules` field will be omitted:

```json
{
    "result": {
        "name": "simple-policy",
        "bitrate_uplink": "10 Mbps",
        "bitrate_downlink": "10 Mbps",
        "var5qi": 9,
        "arp": 1,
        "data_network_name": "internet"
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
