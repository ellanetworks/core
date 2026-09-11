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
| `radio`    | query | str  |         |         | Filter by radio name. |
| `data_network` | query | str |     |         | Filter by data network name.|
| `search`   | query | str  |         | ≤ 254 chars | Filter by IMSI or description substring. |

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
                    "systems": ["5G"],
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
- `description` (optional string): A free-text note about the subscriber. At most 64 characters.

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
- `description` (optional string): A free-text note about the subscriber. At most 64 characters. This path replaces the subscriber in full, so omitting the field clears the stored note.

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
        "system": "5G",
        "registered": true,
        "connection_state": "connected",
        "radio": "gNB-1",
        "last_seen_at": "2026-03-16T12:34:56Z",
        "imei": "359881234567890",
        "ciphering_algorithm": "128-NEA2",
        "integrity_algorithm": "128-NIA2",
        "connection": {
          "amf_ue_ngap_id": 12,
          "ran_ue_ngap_id": 39
        }
      },
      {
        "system": "4G",
        "registered": false,
        "connection_state": null,
        "radio": "eNB-7",
        "last_seen_at": "2026-03-16T12:30:11Z",
        "connection": null
      }
    ],
    "sessions": [
      {
        "system": "5G",
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

`registrations` holds one entry per mobility-management context. It is empty for a subscriber the core has never served.

| Field | Description |
| ----- | ----------- |
| `system` | `5G` or `4G`. The core that registered the device. |
| `registered` | RM state in 5G, EMM state in 4G. `false` on an entry the core remembers but holds no context for. |
| `connection_state` | `connected`, `idle`, or `null`. CM state in 5G, ECM state in 4G. |
| `radio` | Radio serving this registration, or the last one that did when the device is idle or deregistered. |
| `last_seen_at` | Timestamp of last activity in this system (RFC 3339). |
| `imei` | 15-digit IMEI of the device. Absent once the core has released the context. |
| `ciphering_algorithm` | `NEA0` / `128-NEA1..3` in 5G, `EEA0` / `128-EEA1..3` in 4G. Absent when none is established. |
| `integrity_algorithm` | `NIA0` / `128-NIA1..3` in 5G, `EIA0` / `128-EIA1..3` in 4G. Absent when none is established. |
| `connection` | UE-associated logical connection, or `null` when the device holds none. |

### Connection identifiers

`connection` carries only the identifier pair belonging to the registration's `system`.

| Field | System | Description |
| ----- | ------ | ----------- |
| `amf_ue_ngap_id` | 5G | AMF UE NGAP ID, `INTEGER (0..2^40-1)`. Allocated by the core. |
| `ran_ue_ngap_id` | 5G | RAN UE NGAP ID, `INTEGER (0..2^32-1)`. Allocated by the serving radio. Absent until the radio allocates one, for example on a handover target before it accepts the handover. |
| `mme_ue_s1ap_id` | 4G | MME UE S1AP ID, `INTEGER (0..2^32-1)`. Allocated by the core. |
| `enb_ue_s1ap_id` | 4G | eNB UE S1AP ID, `INTEGER (0..2^24-1)`. Allocated by the serving radio. Absent until the radio allocates one. |

### Sessions

| Field | Description |
| ----- | ----------- |
| `system` | `5G` or `4G`, matching a registration's `system`. |
| `id` | PDU Session ID (5G) or linked EPS Bearer ID (4G). |
| `status` | Session status (for example `active`, `inactive`). |
| `ip_type` | `IPv4`, `IPv6` or `IPv4v6`. |
| `data_network` | DNN (5G) or APN (4G). |
| `slice` | S-NSSAI. 5G only. |

## Get Subscriber Credentials

This path returns the authentication credentials for a specific subscriber. 

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
