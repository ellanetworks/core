---
description: RESTful API reference for managing profiles.
---

# Profiles

## List Profiles

This path returns the list of profiles.


| Method | Path               |
| ------ | ------------------ |
| GET    | `/api/v1/profiles` |

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "name": "default-default",
            "ue-ip-pool": "172.250.0.0/16",
            "dns": "8.8.8.8",
            "bitrate-uplink": "200 Mbps",
            "bitrate-downlink": "100 Mbps",
            "var5qi": 8,
            "priority-level": 1
        }
    ]
}
```

## Create a Profile

This path creates a new profile.

| Method | Path               |
| ------ | ------------------ |
| POST   | `/api/v1/profiles` |

### Parameters

- `name` (string): The Name of the profile.
- `ue-ip-pool` (string): The IP pool of the profile in CIDR notation. Example: `172.250.0.0/16`.
- `dns` (string): The IP address of the DNS server of the profile. Example: `8.8.8.8`.
- `mtu` (integer): The MTU of the profile. Must be an integer between 0 and 65535.
- `bitrate-uplink` (string): The uplink bitrate of the profile. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `bitrate-downlink` (string): The downlink bitrate of the profile. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `var5qi` (integer): The QoS class identifier of the profile. Must be an integer between 1 and 255.
- `priority-level` (integer): The priority level of the profile. Must be an integer between 1 and 255.

### Sample Response

```json
{
    "result": {
        "message": "Profile created successfully"
    }
}
```

## Update a Profile

This path updates an existing profile.

| Method | Path                      |
| ------ | ------------------------- |
| PUT    | `/api/v1/profiles/{name}` |

### Parameters

- `ue-ip-pool` (string): The IP pool of the profile in CIDR notation. Example: `172.250.0.0/16`.
- `dns` (string): The IP address of the DNS server of the profile. Example: `8.8.8.8`.
- `mtu` (integer): The MTU of the profile. Must be an integer between 0 and 65535.
- `bitrate-uplink` (string): The uplink bitrate of the profile. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `bitrate-downlink` (string): The downlink bitrate of the profile. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `var5qi` (integer): The QoS class identifier of the profile. Must be an integer between 1 and 255.
- `priority-level` (integer): The priority level of the profile. Must be an integer between 1 and 255.


### Sample Response

```json
{
    "result": {
        "message": "Profile updated successfully"
    }
}
```

## Get a Profile

This path returns the details of a specific profile.

| Method | Path                      |
| ------ | ------------------------- |
| GET    | `/api/v1/profiles/{name}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "name": "my-profile",
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

## Delete a Profile

This path deletes a profile from Ella Core.

| Method | Path                      |
| ------ | ------------------------- |
| DELETE | `/api/v1/profiles/{name}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "Profile deleted successfully"
    }
}
```
