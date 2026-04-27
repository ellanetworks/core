# Policies

Policies define per-session QoS parameters for a specific (profile, slice, data network) combination. The Session AMBR caps the bitrate of a single PDU session and is enforced by Ella Core in the data plane. This is distinct from UE-AMBR (set on the profile), which caps aggregate throughput across all sessions and is enforced by the radio.

## List Policies

This path returns the list of policies.

| Method | Path               |
| ------ | ------------------ |
| GET    | `/api/v1/policies` |

### Query Parameters

| Name       | In    | Type | Default | Allowed | Description               |
| ---------- | ----- | ---- | ------- | ------- | ------------------------- |
| `page`     | query | int  | `1`     | `>= 1`  | 1-based page index.       |
| `per_page` | query | int  | `25`    | `1…100` | Number of items per page. |

### Sample Response

```
{
    "result": {
        "items": [
            {
                "name": "default",
                "profile_name": "enterprise",
                "slice_name": "default",
                "session_ambr_uplink": "200 Mbps",
                "session_ambr_downlink": "100 Mbps",
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
- `profile_name` (string): The name of the profile associated with this policy. Must be the name of an existing profile.
- `slice_name` (string): The name of the slice associated with this policy. Must be the name of an existing slice.
- `session_ambr_uplink` (string): Maximum uplink bitrate for a single PDU session (Session AMBR). Enforced by Ella Core. Format: `<number> <unit>` (e.g. `"100 Mbps"`). Allowed units: Mbps, Gbps.
- `session_ambr_downlink` (string): Maximum downlink bitrate for a single PDU session (Session AMBR). Enforced by Ella Core. Format: `<number> <unit>` (e.g. `"200 Mbps"`). Allowed units: Mbps, Gbps.
- `var5qi` (integer): 5G QoS Identifier. Signaled to the radio, which uses it for scheduling (latency budget, error rate, priority). Valid values: 5, 6, 7, 8, 9, 69, 70, 79, 80 (non-GBR only).
- `arp` (integer): Allocation and Retention Priority (1–15). Used by the radio at session setup for admission control and pre-emption decisions. Has no effect on traffic once the session is established. 1 = highest priority.
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

```
{
    "name": "my-policy",
    "profile_name": "enterprise",
    "slice_name": "default",
    "session_ambr_uplink": "100 Mbps",
    "session_ambr_downlink": "200 Mbps",
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
            },
            {
                "description": "Deny all",
                "protocol": 0,
                "port_low": 0,
                "port_high": 0,
                "remote_prefix": "0.0.0.0/0",
                "action": "deny"
            }
        ],
        "downlink": [
            {
                "description": "Allow DNS",
                "protocol": 17,
                "port_low": 53,
                "port_high": 53,
                "action": "allow"
            },
            {
                "description": "Deny all",
                "protocol": 0,
                "port_low": 0,
                "port_high": 0,
                "remote_prefix": "0.0.0.0/0",
                "action": "deny"
            }
        ]
    }
}
```

### Sample Response

```
{
    "result": {
        "message": "Policy created successfully"
    }
}
```

## Update a Policy

This path updates an existing policy. Network rules are always replaced on every update call.

| Method | Path                      |
| ------ | ------------------------- |
| PUT    | `/api/v1/policies/{name}` |

### Parameters

- `profile_name` (string): The name of the profile associated with this policy. Must be the name of an existing profile.
- `slice_name` (string): The name of the slice associated with this policy. Must be the name of an existing slice.
- `session_ambr_uplink` (string): Maximum uplink bitrate for a single PDU session (Session AMBR). Enforced by Ella Core. Format: `<number> <unit>` (e.g. `"100 Mbps"`). Allowed units: Mbps, Gbps.
- `session_ambr_downlink` (string): Maximum downlink bitrate for a single PDU session (Session AMBR). Enforced by Ella Core. Format: `<number> <unit>` (e.g. `"200 Mbps"`). Allowed units: Mbps, Gbps.
- `var5qi` (integer): 5G QoS Identifier. Signaled to the radio, which uses it for scheduling (latency budget, error rate, priority). Valid values: 5, 6, 7, 8, 9, 69, 70, 79, 80 (non-GBR only).
- `arp` (integer): Allocation and Retention Priority (1–15). Used by the radio at session setup for admission control and pre-emption decisions. Has no effect on traffic once the session is established. 1 = highest priority.
- `data_network_name` (string): The name of the data network associated with the policy. Must be the name of an existing data network.
- `rules` (object, optional): Network rules to set on the policy. Existing rules are always deleted first. If this field is omitted, all existing rules are deleted.

### Rules Behavior

- **Omit `rules` field**: all existing rules are deleted.
- **Provide `rules` with arrays**: existing rules are deleted and replaced with the new ones.
- **Provide empty arrays**: all existing rules are deleted and no new rules are created.

To keep existing rules, you must re-supply them in every update request.

### Sample Request to Update Rules

```
{
    "session_ambr_uplink": "100 Mbps",
    "session_ambr_downlink": "200 Mbps",
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
            },
            {
                "description": "Deny all",
                "protocol": 0,
                "port_low": 0,
                "port_high": 0,
                "remote_prefix": "0.0.0.0/0",
                "action": "deny"
            }
        ],
        "downlink": []
    }
}
```

### Sample Request to Delete All Rules

Omit the `rules` field (or provide empty arrays) to delete all existing rules:

```
{
    "session_ambr_uplink": "100 Mbps",
    "session_ambr_downlink": "200 Mbps",
    "var5qi": 9,
    "arp": 1,
    "data_network_name": "internet"
}
```

### Sample Response

```
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

```
{
    "result": {
        "name": "my-policy",
        "profile_name": "enterprise",
        "slice_name": "default",
        "session_ambr_uplink": "10 Mbps",
        "session_ambr_downlink": "10 Mbps",
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
                },
                {
                    "description": "Deny all",
                    "protocol": 0,
                    "port_low": 0,
                    "port_high": 0,
                    "remote_prefix": "0.0.0.0/0",
                    "action": "deny"
                }
            ],
            "downlink": [
                {
                    "description": "Allow DNS",
                    "protocol": 17,
                    "port_low": 53,
                    "port_high": 53,
                    "action": "allow"
                },
                {
                    "description": "Deny all",
                    "protocol": 0,
                    "port_low": 0,
                    "port_high": 0,
                    "remote_prefix": "0.0.0.0/0",
                    "action": "deny"
                }
            ]
        }
    }
}
```

### Sample Response without Rules

If a policy has no associated rules, the `rules` field will be omitted:

```
{
    "result": {
        "name": "simple-policy",
        "profile_name": "enterprise",
        "slice_name": "default",
        "session_ambr_uplink": "10 Mbps",
        "session_ambr_downlink": "10 Mbps",
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

```
{
    "result": {
        "message": "Policy deleted successfully"
    }
}
```
