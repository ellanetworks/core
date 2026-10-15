---
description: RESTful API reference for locating subscribers and provisioning cell positions.
---

# Location (beta)

Ella Core supports three positioning methods: Cell ID, Enhanced Cell ID (E-CID), and GNSS in standalone mode. The Location API requests a subscriber's location, manages periodic location requests, and provisions the cell positions those estimates are drawn from.

!!! warning "Beta"
    The Location API is experimental and served under `/api/beta`. Its paths and payloads may change without notice.

## Locate a Subscriber

This path returns a subscriber's current location.

| Method | Path                |
| ------ | ------------------- |
| POST   | `/api/beta/location` |

### Parameters

- `supi` (string): Subscriber identity.
- `method` (string, optional): `cell_id`, `ecid`, or `gnss`. Defaults to `cell_id`.
- `mode` (string, optional): `network_based` or `ue_assisted` for `ecid`; `standalone` for `gnss`. Defaults to every mode supported by `method`.
- `measurement_quantities` (object, optional): E-CID measurement quantities (TS 38.455, TS 36.455). Only for `ecid` with `mode` `network_based` or omitted.
    - `nr` (array of strings, optional): Used when the serving cell is NR. Each entry must be one of `cell-ID`, `sS-RSRP`, `sS-RSRQ`, `cSI-RSRP`, `cSI-RSRQ`, `angleOfArrivalNR`, `timingAdvanceNR`, or `uE-Rx-Tx-Time-Diff`. No duplicates allowed.
    - `eutra` (array of strings, optional): Used when the serving cell is E-UTRA. Each entry must be one of `cell-ID`, `angleOfArrival`, `timingAdvanceType1`, `timingAdvanceType2`, `rSRP`, or `rSRQ`. No duplicates allowed.

### Responses

| Status | Condition                                 |
| ------ | ----------------------------------------- |
| `200`  | Returns a location estimate.              |
| `404`  | Subscriber not registered, or no estimate. |
| `504`  | Location request timed out.               |

### Sample Response

```json
{
    "result": {
        "locationEstimate": {
            "shape": "POINT_UNCERTAINTY_CIRCLE",
            "point": {
                "lat": 37.7749,
                "lon": -122.4194
            },
            "uncertainty": 150
        },
        "positioningDataList": [
            {
                "method": "CELLID",
                "mode": "CONVENTIONAL",
                "usage": "SUCCESS_RESULTS_USED_TO_GENERATE_LOCATION"
            }
        ],
        "ncgi": {
            "plmnId": {
                "mcc": "001",
                "mnc": "01"
            },
            "nrCellId": "000000010"
        }
    }
}
```

### Sample Response (`ecid`)

```json
{
    "result": {
        "locationEstimate": {
            "shape": "POINT_UNCERTAINTY_CIRCLE",
            "point": {
                "lat": 37.7749,
                "lon": -122.4194
            },
            "uncertainty": 150
        },
        "positioningDataList": [
            {
                "method": "CELLID",
                "mode": "CONVENTIONAL",
                "usage": "SUCCESS_RESULTS_USED_TO_GENERATE_LOCATION"
            },
            {
                "method": "NR_ECID",
                "mode": "CONVENTIONAL",
                "usage": "UNSUCCESS"
            },
            {
                "method": "NR_ECID",
                "mode": "UE_ASSISTED",
                "usage": "SUCCESS_RESULTS_NOT_USED"
            }
        ],
        "ncgi": {
            "plmnId": {
                "mcc": "001",
                "mnc": "01"
            },
            "nrCellId": "000000010"
        },
        "radioMeasurements": [
            {
                "source": "UE",
                "rat": "NR",
                "serving": true,
                "pci": 180,
                "arfcn": 662592,
                "arfcnType": "SSB",
                "ssRsrp": -84,
                "ssRsrq": -11,
                "ssbBeams": [
                    {
                        "index": 0,
                        "rsrp": -84,
                        "rsrq": -11
                    }
                ]
            },
            {
                "source": "UE",
                "rat": "EUTRA",
                "serving": false,
                "pci": 148,
                "arfcn": 9310,
                "rsrp": -98,
                "rsrq": -7.5
            }
        ]
    }
}
```

## Create a Deferred Location Request

This path starts periodic location reporting for a subscriber (TS 23.273 deferred location request).

| Method | Path                                   |
| ------ | -------------------------------------- |
| POST   | `/api/beta/location/deferred-requests` |

### Parameters

- `supi` (string): Subscriber identity.
- `ldr_type` (string): One of `UE_AVAILABLE`, `PERIODIC`, `ENTERING_INTO_AREA`, `LEAVING_FROM_AREA`, `BEING_INSIDE_AREA`, or `MOTION`. Only `PERIODIC` is supported.
- `periodic_event_info` (object):
    - `reporting_amount` (integer): Number of reports, 1 to 8639999. Required unless `reporting_infinite_ind` is `true`.
    - `reporting_interval` (integer): Interval between reports in seconds, 1 to 8639999. `reporting_amount` x `reporting_interval` must not exceed 8639999.
    - `reporting_infinite_ind` (boolean, optional): Report until 8639999 seconds have elapsed.
    - `reporting_interval_ms` (integer, optional): 1 to 999. Not supported; `reporting_interval` is used.
- `scheduled_loc_time` (string, optional): RFC 3339 time of the first report. Must be in the future.
- `method` (string, optional): `cell_id`, `ecid`, or `gnss`. Defaults to `cell_id`.
- `mode` (string, optional): `network_based` or `ue_assisted` for `ecid`; `standalone` for `gnss`. Defaults to every mode supported by `method`.
- `measurement_quantities` (object, optional): As in [Locate a Subscriber](#locate-a-subscriber).

### Responses

| Status | Condition                                                  |
| ------ | ---------------------------------------------------------- |
| `201`  | Returns the deferred location request.                     |
| `400`  | Invalid parameters.                                        |
| `404`  | Subscriber not registered on this node.                    |
| `409`  | The subscriber already has an active deferred location request. |
| `501`  | `ldr_type` is not supported.                               |
| `503`  | Deferred location requests are unavailable.                |

### Sample Request

```json
{
    "supi": "imsi-001010000000001",
    "ldr_type": "PERIODIC",
    "periodic_event_info": {
        "reporting_amount": 120,
        "reporting_interval": 30
    },
    "method": "ecid",
    "mode": "network_based"
}
```

### Sample Response

```json
{
    "result": {
        "id": "019a2f3c-8d1e-7b2a-9c4f-1e2d3c4b5a69",
        "supi": "imsi-001010000000001",
        "ldr_type": "PERIODIC",
        "periodic_event_info": {
            "reporting_amount": 120,
            "reporting_interval": 30
        },
        "method": "ecid",
        "mode": "network_based",
        "status": "ACTIVE",
        "created_at": "2026-10-15T14:00:00Z"
    }
}
```

## List Deferred Location Requests

This path returns a paginated list of deferred location requests, newest first.

| Method | Path                                   |
| ------ | -------------------------------------- |
| GET    | `/api/beta/location/deferred-requests` |

### Query Parameters

| Name       | In    | Type | Default | Allowed                  | Description          |
| ---------- | ----- | ---- | ------- | ------------------------ | -------------------- |
| `supi`     | query | str  |         |                          | Subscriber identity. |
| `status`   | query | str  |         | `ACTIVE`, `TERMINATED`   | Request status.      |
| `page`     | query | int  | `1`     | `>= 1`                   | Page number.         |
| `per_page` | query | int  | `25`    | `1..100`                 | Items per page.      |

### Response Fields

| Field                    | Type    | Description                                                                      |
| ------------------------ | ------- | -------------------------------------------------------------------------------- |
| `id`                     | string  | Request identifier.                                                              |
| `supi`                   | string  | Subscriber identity.                                                             |
| `ldr_type`               | string  | `PERIODIC`.                                                                      |
| `periodic_event_info`    | object  | Accepted reporting parameters.                                                   |
| `scheduled_loc_time`     | string  | Time of the first report.                                                        |
| `method`                 | string  | Positioning method.                                                              |
| `mode`                   | string  | Positioning mode.                                                                |
| `measurement_quantities` | object  | E-CID measurement quantities.                                                    |
| `status`                 | string  | `ACTIVE` or `TERMINATED`.                                                        |
| `termination_cause`      | string  | `NORMAL_TERMINATION`, `TERMINATION_BY_UE`, or `TERMINATION_BY_NETWORK` (TS 29.572). |
| `termination_reason`     | string  | `ue_deregistered`, `ue_mobility`, `positioning_failed`, `node_restart`, or `node_shutdown`. |
| `event_report_counter`   | integer | Number of reports produced (TS 29.572). Absent before the first report.          |
| `created_at`             | string  | Creation time.                                                                   |
| `terminated_at`          | string  | Termination time.                                                                |

Terminated requests and their reports are deleted 7 days after termination. Each request keeps its 10000 most recent reports.

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "id": "019a2f3c-8d1e-7b2a-9c4f-1e2d3c4b5a69",
                "supi": "imsi-001010000000001",
                "ldr_type": "PERIODIC",
                "periodic_event_info": {
                    "reporting_amount": 120,
                    "reporting_interval": 30
                },
                "method": "ecid",
                "mode": "network_based",
                "status": "TERMINATED",
                "termination_cause": "TERMINATION_BY_NETWORK",
                "termination_reason": "ue_deregistered",
                "event_report_counter": 42,
                "created_at": "2026-10-15T14:00:00Z",
                "terminated_at": "2026-10-15T14:21:03.512Z"
            }
        ],
        "page": 1,
        "per_page": 25,
        "total_count": 1
    }
}
```

## Get a Deferred Location Request

This path returns a deferred location request.

| Method | Path                                        |
| ------ | ------------------------------------------- |
| GET    | `/api/beta/location/deferred-requests/{id}` |

### Path Parameters

| Name | Type   | Description         |
| ---- | ------ | ------------------- |
| `id` | string | Request identifier. |

The response fields are those of [List Deferred Location Requests](#list-deferred-location-requests).

## Delete a Deferred Location Request

This path stops a deferred location request if it is active, then deletes it and its reports.

| Method | Path                                        |
| ------ | ------------------------------------------- |
| DELETE | `/api/beta/location/deferred-requests/{id}` |

### Path Parameters

| Name | Type   | Description         |
| ---- | ------ | ------------------- |
| `id` | string | Request identifier. |

Returns `200 OK` on success.

## List Location Reports

This path returns the location reports of a deferred location request, oldest first.

| Method | Path                                                |
| ------ | --------------------------------------------------- |
| GET    | `/api/beta/location/deferred-requests/{id}/reports` |

### Query Parameters

| Name    | In    | Type | Default | Allowed   | Description                                  |
| ------- | ----- | ---- | ------- | --------- | -------------------------------------------- |
| `after` | query | int  | `0`     | `>= 0`    | Return reports with a greater `seq`.         |
| `limit` | query | int  | `100`   | `1..1000` | Maximum number of reports.                   |

### Response Fields

| Field                   | Type    | Description                                              |
| ----------------------- | ------- | -------------------------------------------------------- |
| `items[].seq`           | integer | Report number, starting at 1.                            |
| `items[].reported_at`   | string  | Report time.                                             |
| `items[].location_data` | object  | Location estimate, as returned by [Locate a Subscriber](#locate-a-subscriber). |
| `next`                  | integer | Value to pass as `after` for the following reports.      |

### Sample Response

```json
{
    "result": {
        "items": [
            {
                "seq": 1,
                "reported_at": "2026-10-15T14:00:00.812Z",
                "location_data": {
                    "locationEstimate": {
                        "shape": "POINT_UNCERTAINTY_CIRCLE",
                        "point": {
                            "lat": 37.7749,
                            "lon": -122.4194
                        },
                        "uncertainty": 50
                    },
                    "positioningDataList": [
                        {
                            "method": "NR_ECID",
                            "mode": "CONVENTIONAL",
                            "usage": "SUCCESS_RESULTS_USED_TO_GENERATE_LOCATION"
                        }
                    ]
                }
            }
        ],
        "next": 1
    }
}
```

## List Cell Positions

This path returns all provisioned cell positions.

| Method | Path                      |
| ------ | ------------------------- |
| GET    | `/api/beta/cell-positions` |

### Response Fields

| Field                    | Type    | Description                                          |
| ------------------------ | ------- | ---------------------------------------------------- |
| `id`                     | string  | Cell position identifier.                            |
| `rat`                    | string  | `nr` or `eutra`.                                     |
| `mcc`                    | string  | Mobile Country Code.                                 |
| `mnc`                    | string  | Mobile Network Code.                                 |
| `cell_identity`          | string  | Hex cell identity (NCI for NR, ECI for E-UTRA).      |
| `gnb_id`                 | string  | gNB identifier.                                      |
| `latitude`               | number  | WGS-84 latitude in decimal degrees.                  |
| `longitude`              | number  | WGS-84 longitude in decimal degrees.                 |
| `altitude`               | number  | Altitude, in metres.                                 |
| `uncertainty_semi_major` | number  | Semi-major axis of the uncertainty ellipse, in metres. |
| `uncertainty_semi_minor` | number  | Semi-minor axis of the uncertainty ellipse, in metres. |
| `orientation_major`      | integer | Orientation of the semi-major axis, in degrees.      |
| `confidence`             | integer | Confidence, in percent.                              |
| `source`                 | string  | Origin of the record (e.g. `provisioned`).           |

### Sample Response

```json
{
    "result": [
        {
            "id": "018f9f3a-4c2e-7bd1-9f21-0a1b2c3d4e5f",
            "rat": "nr",
            "mcc": "001",
            "mnc": "01",
            "cell_identity": "000000010",
            "latitude": 37.7749,
            "longitude": -122.4194,
            "source": "provisioned"
        }
    ]
}
```

## Create a Cell Position

This path provisions the geographic position of a cell antenna. Cell ID lookups resolve a subscriber's serving cell to this position.

| Method | Path                      |
| ------ | ------------------------- |
| POST   | `/api/beta/cell-positions` |

### Parameters

- `rat` (string): `nr` or `eutra`.
- `mcc` (string): Mobile Country Code.
- `mnc` (string): Mobile Network Code.
- `cell_identity` (string): Hex cell identity (NCI for NR, ECI for E-UTRA).
- `gnb_id` (string, optional): gNB identifier.
- `latitude` (number): WGS-84 latitude in decimal degrees. Range `-90`…`90`.
- `longitude` (number): WGS-84 longitude in decimal degrees. Range `-180`…`180`.
- `altitude` (number, optional): Altitude, in metres.
- `uncertainty_semi_major` (number, optional): Semi-major axis of the uncertainty ellipse, in metres.
- `uncertainty_semi_minor` (number, optional): Semi-minor axis of the uncertainty ellipse, in metres.
- `orientation_major` (integer, optional): Orientation of the semi-major axis, in degrees.
- `confidence` (integer, optional): Confidence, in percent.

### Sample Request (E-UTRA)

```json
{
    "rat": "eutra",
    "mcc": "001",
    "mnc": "01",
    "cell_identity": "0000001",
    "latitude": 37.7749,
    "longitude": -122.4194
}
```

### Sample Response

```json
{
    "result": {
        "message": "Cell position created",
        "id": "018f9f3a-4c2e-7bd1-9f21-0a1b2c3d4e5f"
    }
}
```

## Get a Cell Position

This path returns a provisioned cell position.

| Method | Path                           |
| ------ | ------------------------------ |
| GET    | `/api/beta/cell-positions/{id}` |

### Path Parameters

| Name | Type   | Description               |
| ---- | ------ | ------------------------- |
| `id` | string | Cell position identifier. |

### Sample Response

```json
{
    "result": {
        "id": "018f9f3a-4c2e-7bd1-9f21-0a1b2c3d4e5f",
        "rat": "nr",
        "mcc": "001",
        "mnc": "01",
        "cell_identity": "000000010",
        "latitude": 37.7749,
        "longitude": -122.4194,
        "source": "provisioned"
    }
}
```

## Update a Cell Position

This path updates a provisioned cell position.

| Method | Path                           |
| ------ | ------------------------------ |
| PUT    | `/api/beta/cell-positions/{id}` |

### Path Parameters

| Name | Type   | Description               |
| ---- | ------ | ------------------------- |
| `id` | string | Cell position identifier. |

### Parameters

- `rat` (string): `nr` or `eutra`.
- `mcc` (string): Mobile Country Code.
- `mnc` (string): Mobile Network Code.
- `cell_identity` (string): Hex cell identity (NCI for NR, ECI for E-UTRA).
- `gnb_id` (string, optional): gNB identifier.
- `latitude` (number): WGS-84 latitude in decimal degrees. Range `-90`…`90`.
- `longitude` (number): WGS-84 longitude in decimal degrees. Range `-180`…`180`.
- `altitude` (number, optional): Altitude, in metres.
- `uncertainty_semi_major` (number, optional): Semi-major axis of the uncertainty ellipse, in metres.
- `uncertainty_semi_minor` (number, optional): Semi-minor axis of the uncertainty ellipse, in metres.
- `orientation_major` (integer, optional): Orientation of the semi-major axis, in degrees.
- `confidence` (integer, optional): Confidence, in percent.

### Sample Response

```json
{
    "result": {
        "message": "Cell position updated"
    }
}
```

## Delete a Cell Position

This path deletes a provisioned cell position.

| Method | Path                           |
| ------ | ------------------------------ |
| DELETE | `/api/beta/cell-positions/{id}` |

### Path Parameters

| Name | Type   | Description               |
| ---- | ------ | ------------------------- |
| `id` | string | Cell position identifier. |

Returns `200 OK` on success.
