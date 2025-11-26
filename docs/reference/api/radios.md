---
description: RESTful API reference for viewing connected radio information.
---

# Radios

Radios are automatically added to Ella Core as they connect to the network as long as they are configured to use the same Tracking Area Code (TAC), Mobile Country Code (MCC), and Mobile Network Code (MNC) as Ella Core.

The Radio API provides endpoints to view information about connected radios.

## List Radios

This path returns the list of radios in the inventory.


| Method | Path             |
| ------ | ---------------- |
| GET    | `/api/v1/ran/radios` |

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
                "name": "gnb1",
                "id": "001:01:000102",
                "address": "10.1.107.203/192.168.251.5:9487",
                "supported_tais": [
                    {
                        "tai": {
                            "plmnID": {
                                "mcc": "001",
                                "mnc": "01"
                            },
                            "tac": "000001"
                        },
                        "snssais": [
                            {
                                "sst": 1,
                                "sd": "102030"
                            }
                        ]
                    },
                    {
                        "tai": {
                            "plmnID": {
                                "mcc": "123",
                                "mnc": "12"
                            },
                            "tac": "000002"
                        },
                        "snssais": [
                            {
                                "sst": 1,
                                "sd": "102031"
                            }
                        ]
                    }
                ]
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
}
```

## Get a Radio

This path returns the details of a specific radio in the inventory.

| Method | Path                    |
| ------ | ----------------------- |
| GET    | `/api/v1/ran/radios/{name}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "name": "gnb1",
        "id": "001:01:000102",
        "address": "10.1.107.203/192.168.251.5:9487",
        "supported_tais": [
            {
                "tai": {
                    "plmnID": {
                        "mcc": "001",
                        "mnc": "01"
                    },
                    "tac": "000001"
                },
                "snssais": [
                    {
                        "sst": 1,
                        "sd": "102030"
                    }
                ]
            },
            {
                "tai": {
                    "plmnID": {
                        "mcc": "123",
                        "mnc": "12"
                    },
                    "tac": "000002"
                },
                "snssais": [
                    {
                        "sst": 1,
                        "sd": "102031"
                    }
                ]
            }
        ]
    }
}
```

## List Radio Events

This path returns the list of radio events.

| Method | Path                         |
| ------ | ---------------------------- |
| GET    | `/api/v1/ran/events`   |

### Query Parameters

| Name             | In    | Type | Default | Allowed           | Description                                                                                     |
| ----------       | ----- | ---- | ------- | ----------------- | ----------------------------------------------------------------------------------------------- |
| `page`           | query | int  | `1`     | `>= 1`            | 1-based page index.                                                                             |
| `per_page`       | query | int  | `25`    | `1…100`           | Number of items per page.                                                                       |
| `protocol`       | query | str  |         |                   | Filter by protocol.                                                                              |
| `direction`      | query | str  |         | inbound, outbound | Filter by log direction.                                                                        |
| `message_type`   | query | str  |         |                   | Filter by message type.                                                                          |
| `timestamp_from` | query | str  |         |                   | Filter logs from this timestamp (inclusive). RFC3339 format (e.g., 2006-01-02T15:04:05Z07:00).  |
| `timestamp_to`   | query | str  |         |                   | Filter logs up to this timestamp (inclusive). RFC3339 format (e.g., 2006-01-02T15:04:05Z07:00). |

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "id": 1,
                "timestamp": "2025-08-12T16:58:00.810-0400",
                "protocol": "NGAP",
                "message_type": "PDU Session Establishment Accept",
                "direction": "inbound",
                "raw": "ABUAOQAABAAbAAkAAPEQMAASNFAAUkAMBIBnbmIwMDEyMzQ1AGYAEAAAAAABAADxEAAAEAgQIDAAFUABQA",
                "details": "{\"pduSessionID\":1}"
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
}
```

## Get Radio Event

This path returns a specific radio event by its ID.

| Method | Path                         |
| ------ | ---------------------------- |
| GET    | `/api/v1/ran/events/{id}` |

### Sample Response

```json
{
    "result": {
        "raw": "IBUALAAABAABAAUBAGFtZgBgAAgAAADxEMr+AABWQAH/AFAACwAA8RAAABAIECAw",
        "decoded": {
            "successful_outcome": {
                "procedure_code": "NGSetup",
                "criticality": "Reject (0)",
                "value": {
                    "ng_setup_response": {
                        "ies": [
                            {
                                "id": "AMFName (1)",
                                "criticality": "Reject (0)",
                                "amf_name": "amf"
                            },
                            {
                                "id": "ServedGUAMIList (96)",
                                "criticality": "Reject (0)",
                                "served_guami_list": [
                                    {
                                        "plmn_id": {
                                            "mcc": "001",
                                            "mnc": "01"
                                        },
                                        "amf_id": "cafe00"
                                    }
                                ]
                            },
                            {
                                "id": "RelativeAMFCapacity (86)",
                                "criticality": "Ignore (1)",
                                "relative_amf_capacity": 255
                            },
                            {
                                "id": "PLMNSupportList (80)",
                                "criticality": "Reject (0)",
                                "plmn_support_list": [
                                    {
                                        "plmn_id": {
                                            "mcc": "001",
                                            "mnc": "01"
                                        },
                                        "slice_support_list": [
                                            {
                                                "sst": 1,
                                                "sd": "102030"
                                            }
                                        ]
                                    }
                                ]
                            }
                        ]
                    }
                }
            }
        }
    }
}
```


## Update Radio Event Retention Policy

This path updates the radio event retention policy.

| Method | Path                           |
| ------ | ------------------------------ |
| PUT    | `/api/v1/ran/events/retention` |

### Parameters

- `days` (integer): The number of days to retain radio events. Must be a positive integer.

### Sample Response

```json
{
    "result": {
        "message": "Radio event retention policy updated successfully"
    }
}
```

## Clear Radio Events

This path deletes all radio events.

| Method | Path                         |
| ------ | ---------------------------- |
| DELETE | `/api/v1/ran/events`   |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "All radio events have been deleted successfully"
    }
}
```

## Get Radio Event Retention Policy

This path returns the current radio event retention policy.

| Method | Path                           |
| ------ | ------------------------------ |
| GET    | `/api/v1/ran/events/retention` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "days": 30
    }
}
```
