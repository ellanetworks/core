---
description: RESTful API reference for managing profiles.
---

# Profiles

Profiles define the subscriber's Aggregate Maximum Bit Rate (UE-AMBR). Ella Core signals this value to the radio, which enforces it. A profile can be shared across multiple policies.

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
                "ue_ambr_downlink": "1 Gbps",
                "allow_4g": true,
                "allow_5g": true
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
- `ue_ambr_uplink` (string): Aggregate uplink bitrate cap across all of the subscriber's sessions (UE-AMBR). Enforced by the radio. Format: `<number> <unit>` (e.g. `"500 Mbps"`). Allowed units: Kbps, Mbps, Gbps.
- `ue_ambr_downlink` (string): Aggregate downlink bitrate cap across all of the subscriber's sessions (UE-AMBR). Enforced by the radio. Format: `<number> <unit>` (e.g. `"1 Gbps"`). Allowed units: Kbps, Mbps, Gbps.
- `allow_4g` (boolean, optional): Whether subscribers using this profile may attach over 4G (EPC). Defaults to `true`.
- `allow_5g` (boolean, optional): Whether subscribers using this profile may register over 5G (5GC). Defaults to `true`.

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
        "ue_ambr_downlink": "1 Gbps",
        "allow_4g": true,
        "allow_5g": true
    }
}
```

## Update a Profile

This path updates an existing profile.

| Method | Path                      |
| ------ | ------------------------- |
| PUT    | `/api/v1/profiles/{name}` |

### Parameters

- `ue_ambr_uplink` (string): Aggregate uplink bitrate cap across all of the subscriber's sessions (UE-AMBR). Enforced by the radio. Format: `<number> <unit>` (e.g. `"500 Mbps"`). Allowed units: Kbps, Mbps, Gbps. The ceiling depends on the access the profile permits: 10 Gbps when 4G is allowed (S1AP `BitRate`, TS 36.413), otherwise 4 Tbps (NGAP `BitRate`, TS 38.413).
- `ue_ambr_downlink` (string): Aggregate downlink bitrate cap across all of the subscriber's sessions (UE-AMBR). Enforced by the radio. Format: `<number> <unit>` (e.g. `"1 Gbps"`). Allowed units: Kbps, Mbps, Gbps. The ceiling depends on the access the profile permits: 10 Gbps when 4G is allowed (S1AP `BitRate`, TS 36.413), otherwise 4 Tbps (NGAP `BitRate`, TS 38.413).
- `allow_4g` (boolean, optional): Whether subscribers using this profile may attach over 4G (EPC). Defaults to `true`.
- `allow_5g` (boolean, optional): Whether subscribers using this profile may register over 5G (5GC). Defaults to `true`.

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

## Get a Profile's IMS Initial Filter Criteria

This path returns a profile's IMS initial filter criteria, in priority order.

| Method | Path                                          |
| ------ | --------------------------------------------- |
| GET    | `/api/v1/profiles/{name}/ims-filter-criteria` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "initial_filter_criteria": [
            {
                "priority": 0,
                "application_server_name": "conference",
                "trigger_point": {
                    "condition_type_cnf": false,
                    "spt": [{"group": [0], "request_uri": "^sip:conference@.*$"}]
                },
                "default_handling": "SESSION_TERMINATED",
                "include_register_request": false,
                "include_register_response": false
            }
        ]
    }
}
```

## Update a Profile's IMS Initial Filter Criteria

This path replaces a profile's IMS initial filter criteria. Registered subscribers of the profile get the new user profile.

| Method | Path                                          |
| ------ | --------------------------------------------- |
| PUT    | `/api/v1/profiles/{name}/ims-filter-criteria` |

### Parameters

- `initial_filter_criteria` (array): In priority order ([TS 29.228 Annex B.2.2](https://www.3gpp.org/DynaReport/29228.htm)). An empty list removes all of them. Each criterion has:
    - `application_server_name` (string): The name of the [IMS application server](operator.md#create-an-ims-application-server) matching requests are sent to.
    - `trigger_point` (optional object): Without it, the criterion matches every request.
        - `condition_type_cnf` (boolean): `true` for conjunctive normal form, `false` for disjunctive normal form.
        - `spt` (array): Service point triggers. Each has `group` (array of integers), an optional `condition_negated` (boolean), and exactly one of `request_uri` (string), `method` (string), `sip_header` (`{header, content}`), `session_case` (`ORIGINATING_REGISTERED`, `TERMINATING_REGISTERED`, `TERMINATING_UNREGISTERED`, `ORIGINATING_UNREGISTERED` or `ORIGINATING_CDIV`) and `session_description` (`{line, content}`). With the `REGISTER` method, `registration_type` lists up to two of `INITIAL_REGISTRATION`, `RE-REGISTRATION` and `DE-REGISTRATION`. Request URIs, headers, lines and contents are extended regular expressions.
    - `profile_part` (optional string): `REGISTERED` or `UNREGISTERED`, the part of the user profile the criterion belongs to. Omitted for both.
    - `default_handling` (optional string): `SESSION_CONTINUED` or `SESSION_TERMINATED`, what the S-CSCF does when the application server can't be reached.
    - `service_info` (optional string): Passed transparently to the application server.
    - `include_register_request` (optional boolean)
    - `include_register_response` (optional boolean)

The `priority` of a criterion is its position in the list. It is returned on reads and ignored on writes.

### Sample Request

```json
{
    "initial_filter_criteria": [
        {
            "application_server_name": "conference",
            "trigger_point": {
                "condition_type_cnf": false,
                "spt": [{"group": [0], "request_uri": "^sip:conference@.*$"}]
            },
            "default_handling": "SESSION_TERMINATED"
        }
    ]
}
```

### Sample Response

```json
{
    "result": {
        "message": "Profile initial filter criteria updated successfully"
    }
}
```
