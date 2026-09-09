---
description: RESTful API reference for managing network subscribers.
---

# Subscribers

This section describes the RESTful API for managing network subscribers. Network subscribers are the devices that connect to the private mobile network.

## List Subscribers

This path returns the list of network subscribers, ordered by IMSI.

| Method | Path                  |
| ------ | --------------------- |
| GET    | `/api/v1/subscribers` |

### Query Parameters

| Name       | In    | Type | Default | Allowed | Description                   |
| ---------- | ----- | ---- | ------- | ------- | ----------------------------- |
| `page`     | query | int  | `1`     | `>= 1`  | 1-based page index.           |
| `per_page` | query | int  | `25`    | `1…100` | Number of items per page.     |
| `radio`    | query | str  |         |         | Filter by radio name. Returns registered subscribers whose last-seen radio is this one, so an idle subscriber still matches the radio that last served it. |
| `data_network` | query | str |     |         | Filter by data network name. Returns only subscribers whose profile reaches it. |
| `search`   | query | str  |         | ≤ 254 chars | Filter by IMSI or description substring. The value is matched literally, so `%` and `_` are not wildcards. Case is ignored for ASCII letters only. |

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "imsi": "001010100007487",
                "profile_name": "default",
                "description": "Warehouse gate reader",
                "status": {
                    "registered": true,
                    "connection_state": "connected",
                    "radio_access_types": ["5G"],
                    "num_sessions": 1,
                    "last_seen_at": "2026-03-16T12:34:56Z",
                    "last_seen_radio": "gNB-1"
                }
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
}
```

`description` is omitted from an item when that subscriber has no note.

## Create a Subscriber

This path creates a new network subscriber.

| Method | Path                  |
| ------ | --------------------- |
| POST   | `/api/v1/subscribers` |

### Parameters

- `imsi` (string): The IMSI of the subscriber. Must be a 15-digit string starting with `<mcc><mnc>`.
- `key` (string): The key of the subscriber. Must be a 32-character hexadecimal string.
- `sequenceNumber` (string): The sequence number of the subscriber. Must be a 6-byte hexadecimal string.
- `profile_name` (string): The profile name of the subscriber. Must be the name of an existing profile.
- `opc` (optional string): The operator code of the subscriber. If not provided, it will be generated automatically using the Operator Code (OP) and the `key` parameter.
- `description` (optional string): A free-text note about the subscriber. At most 64 characters; surrounding whitespace is trimmed.

### Sample Response

```json
{
    "result": {
        "message": "Subscriber created successfully"
    }
}
```

## Update a Subscriber

This path updates an existing network subscriber.

| Method | Path                         |
| ------ | ---------------------------- |
| PUT    | `/api/v1/subscribers/{imsi}` |

### Parameters

- `profile_name` (string): The profile name of the subscriber.
- `description` (optional string): A free-text note about the subscriber. At most 64 characters; surrounding whitespace is trimmed. This path replaces the subscriber in full, so omitting the field clears the stored note.

### Sample Response

```json
{
    "result": {
        "message": "Subscriber updated successfully"
    }
}
```

## Get a Subscriber

This path returns the details of a specific network subscriber.

| Method | Path                         |
| ------ | ---------------------------- |
| GET    | `/api/v1/subscribers/{imsi}` |

### Parameters

None

### Sample Response

```json
{
  "result": {
    "imsi": "001010100007487",
    "profile_name": "default",
    "description": "Warehouse gate reader",
    "registrations": [
      {
        "system": "5GS",
        "access_type": "3GPP",
        "registered": true,
        "connection_state": "connected",
        "radio": "gNB-1",
        "last_seen_at": "2026-03-16T12:34:56Z",
        "pei": "imeisv-3535938300494715",
        "imei": "359881234567890",
        "ciphering_algorithm": "128-NEA2",
        "integrity_algorithm": "128-NIA2",
        "connection": {
          "amf_ue_ngap_id": 12,
          "ran_ue_ngap_id": 39
        }
      },
      {
        "system": "EPS",
        "access_type": "3GPP",
        "registered": false,
        "connection_state": null,
        "radio": "eNB-7",
        "last_seen_at": "2026-03-16T12:30:11Z",
        "connection": null
      }
    ],
    "sessions": [
      {
        "system": "5GS",
        "access_types": ["3GPP"],
        "id": 1,
        "status": "active",
        "ip_type": "IPv4v6",
        "ipv4_address": "10.45.0.2",
        "ipv6_prefix": "2001:db8::/64",
        "data_network": "internet",
        "slice": {
          "sst": 1,
          "sd": "000001"
        },
        "ambr_uplink": "100 Mbps",
        "ambr_downlink": "200 Mbps"
      }
    ]
  }
}
```

`description` is omitted when the subscriber has no note.

### Registrations

`registrations` holds one entry per mobility-management context, ordered 5GS first. It is empty for a subscriber the core has never served.

| Field | Description |
| ----- | ----------- |
| `system` | `5GS` or `EPS`. The core that registered the device, independent of the radio: an NSA device on NR radio is registered in `EPS`. |
| `access_type` | `3GPP` or `non-3GPP`. Always `3GPP` in this release. |
| `registered` | RM state in 5GS (TS 23.501 §5.3.2.2), EMM state in EPS (TS 23.401 §4.6.2). `false` on an entry the core remembers but holds no context for. |
| `connection_state` | `connected` or `idle`. CM state in 5GS (TS 23.501 §5.3.3.2), ECM state in EPS (TS 23.401 §4.6.3). Independent of `registered`: a device still registering is `connected` with `registered` false. `null` when the core holds no context. |
| `radio` | Radio serving this registration, or the last one that did when the device is idle or deregistered, in which case it may be stale. Held in memory by the serving node: not shared across cluster nodes, and reset on restart. |
| `last_seen_at` | Timestamp of last activity in this system (RFC 3339). |
| `pei` | Permanent Equipment Identifier in NAS-prefixed form (`imei-…` / `imeisv-…`). 5GS only (TS 23.003 §6.4). Absent once the core has released the context. |
| `imei` | 15-digit IMEI. On a 5GS registration this is the IMEI carried by `pei`, present only when the PEI is an IMEI or IMEISV. Absent once the core has released the context. |
| `ciphering_algorithm` | `NEA0` / `128-NEA1..3` in 5GS (TS 33.501 Annex D), `EEA0` / `128-EEA1..3` in EPS (TS 33.401 Annex B). Absent once the core has released the context. |
| `integrity_algorithm` | `NIA0` / `128-NIA1..3` in 5GS, `EIA0` / `128-EIA1..3` in EPS. Absent once the core has released the context. |
| `connection` | UE-associated logical connection, or `null` when the device holds none. |

### Connection identifiers

`connection` carries only the identifier pair belonging to the registration's `system`.

| Field | System | Description |
| ----- | ------ | ----------- |
| `amf_ue_ngap_id` | 5GS | AMF UE NGAP ID, `INTEGER (0..2^40-1)` (TS 38.413 §9.3.3.1). Allocated by the core. |
| `ran_ue_ngap_id` | 5GS | RAN UE NGAP ID, `INTEGER (0..2^32-1)` (TS 38.413 §9.3.3.2). Allocated by the serving NG-RAN node. Absent until it is allocated, for example on a handover target before HANDOVER REQUEST ACKNOWLEDGE (TS 38.413 §9.2.3.5). |
| `mme_ue_s1ap_id` | EPS | MME UE S1AP ID, `INTEGER (0..2^32-1)` (TS 36.413 §9.2.3.3). Allocated by the core. |
| `enb_ue_s1ap_id` | EPS | eNB UE S1AP ID, `INTEGER (0..2^24-1)` (TS 36.413 §9.2.3.4). Allocated by the serving eNB. Absent until it is allocated. |

These values appear under the same names on Ella Core's log lines for the device.

A RAN-allocated identifier is unique only within one NG or S1 interface instance and is reused after the connection ends, so it is not a stable device identifier.

### Sessions

| Field | Description |
| ----- | ----------- |
| `system` | `5GS` or `EPS`, matching a registration's `system`. |
| `access_types` | Every access the session is carried over. A Multi-Access PDU Session names both `3GPP` and `non-3GPP` (TS 23.501 §5.6.1); any other session names exactly one. |
| `id` | PDU Session ID (5GS) or linked EPS Bearer ID (EPS). |
| `status` | Session status (for example `active`, `inactive`). |
| `ip_type` | `IPv4`, `IPv6` or `IPv4v6`. |
| `data_network` | DNN (5GS) or APN (EPS). |
| `slice` | S-NSSAI. 5GS only. |

## Get Subscriber Credentials

This path returns the authentication credentials for a specific subscriber. The response includes the subscriber's permanent key, OPc, and sequence number. This is the preferred way to retrieve credentials and replaces the deprecated fields on the List and Get responses.

An audit log entry is created each time credentials are viewed.

| Method | Path                                      |
| ------ | ----------------------------------------- |
| GET    | `/api/v1/subscribers/{imsi}/credentials`  |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "key": "5122250214c33e723a5dd523fc145fc0",
        "opc": "981d464c7c52eb6e5036234984ad0bcf",
        "sequenceNumber": "16f3b3f70fc7"
    }
}
```

## Delete a Subscriber

This path deletes a subscriber from Ella Core.

| Method | Path                         |
| ------ | ---------------------------- |
| DELETE | `/api/v1/subscribers/{imsi}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "Subscriber deleted successfully"
    }
}
```
