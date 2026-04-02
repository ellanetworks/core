---
description: RESTful API reference for managing profiles.
---

# Profiles

Profiles define UE-level aggregate maximum bit rate (UE-AMBR) settings that can be shared across multiple policies.

## List Profiles

This path returns the list of profiles.

| Method | Path               |
| ------ | ------------------ |
| GET    | `/api/v1/profiles` |

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
                "name": "enterprise",
                "ue_ambr_uplink": "1 Gbps",
                "ue_ambr_downlink": "1 Gbps"
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
}
```

## Create a Profile

This path creates a new profile.

| Method | Path               |
| ------ | ------------------ |
| POST   | `/api/v1/profiles` |

### Parameters

- `name` (string): The name of the profile.
- `ue_ambr_uplink` (string): The UE aggregate maximum bit rate for the uplink. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `ue_ambr_downlink` (string): The UE aggregate maximum bit rate for the downlink. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.

### Sample Response

```json
{
    "result": {
        "message": "Profile created successfully"
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
        "name": "enterprise",
        "ue_ambr_uplink": "1 Gbps",
        "ue_ambr_downlink": "1 Gbps"
    }
}
```

## Update a Profile

This path updates an existing profile.

| Method | Path                      |
| ------ | ------------------------- |
| PUT    | `/api/v1/profiles/{name}` |

### Parameters

- `ue_ambr_uplink` (string): The UE aggregate maximum bit rate for the uplink. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.
- `ue_ambr_downlink` (string): The UE aggregate maximum bit rate for the downlink. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps.

### Sample Response

```json
{
    "result": {
        "message": "Profile updated successfully"
    }
}
```

## Delete a Profile

This path deletes a profile from Ella Core. A profile cannot be deleted if it is referenced by any policy.

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
