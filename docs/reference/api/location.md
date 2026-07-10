---
description: RESTful API reference for locating subscribers and provisioning cell positions.
---

# Location (beta)

Ella Core supports two positioning methods: Cell ID and Enhanced Cell ID (E-CID). The Location API requests a subscriber's location, tracks deferred positioning sessions, and provisions the cell positions those estimates are drawn from.

!!! warning "Beta"
    The Location API is experimental and served under `/api/beta`. Its paths and payloads may change without notice.

## Locate a Subscriber

This path requests a subscriber's current location. `immediate` returns an estimate directly; `periodic` and `triggered` open a positioning session; `cancel` terminates one.

| Method | Path                |
| ------ | ------------------- |
| POST   | `/api/beta/location` |

### Query Parameters

| Name      | In    | Type | Default | Allowed        | Description                                                        |
| --------- | ----- | ---- | ------- | -------------- | ------------------------------------------------------------------ |
| `verbose` | query | bool | `false` | `true`,`false` | Attach the `supplementaryMeasurements` block (raw NRPPa measurements). |

### Parameters

- `request_type` (string): `immediate`, `periodic`, `triggered`, or `cancel`.
- `supi` (string): Subscriber identity. Required unless `request_type` is `cancel`.
- `method` (string, optional): `cell_id` or `ecid`. Defaults to `cell_id`.
- `session_id` (string): Session to terminate. Required when `request_type` is `cancel`.
- `qos_response_time_ms` (integer, optional): Requested response-time budget, in milliseconds.
- `qos_horizontal_accuracy_m` (integer, optional): Requested horizontal accuracy, in metres.

### Responses

| Status | Condition                                              |
| ------ | ------------------------------------------------------ |
| `200`  | `immediate` request; returns a location estimate.      |
| `201`  | `periodic`/`triggered` request; returns a session `id`. |
| `204`  | `cancel` request; no body.                             |

### Sample Response (`immediate`)

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
        "accuracyFulfilmentIndicator": "REQUESTED_ACCURACY_FULFILLED",
        "positioningDataList": [
            {
                "method": "CELLID",
                "mode": "CONVENTIONAL",
                "usage": "SUCCESS_RESULTS_USED"
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

For a 4G (E-UTRA) subscriber the serving cell is reported as `ecgi` with a 7-hex-digit `eutraCellId`, and E-CID uses the `ECID` positioning method:

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
        "accuracyFulfilmentIndicator": "REQUESTED_ACCURACY_FULFILLED",
        "positioningDataList": [
            {
                "method": "ECID",
                "mode": "CONVENTIONAL",
                "usage": "SUCCESS_RESULTS_USED"
            }
        ],
        "ecgi": {
            "plmnId": {
                "mcc": "001",
                "mnc": "01"
            },
            "eutraCellId": "0000001"
        }
    }
}
```

### Sample Response (`periodic`/`triggered`)

```json
{
    "result": {
        "id": "018f9f3a-4c2e-7bd1-9f21-0a1b2c3d4e5f"
    }
}
```

## List Positioning Sessions

This path returns the positioning sessions for a subscriber.

| Method | Path                             |
| ------ | -------------------------------- |
| GET    | `/api/beta/positioning/sessions` |

### Query Parameters

| Name   | In    | Type | Default | Allowed | Description                      |
| ------ | ----- | ---- | ------- | ------- | -------------------------------- |
| `supi` | query | str  |         |         | Subscriber identity. Required.   |

### Response Fields

| Field          | Type    | Description                                             |
| -------------- | ------- | ------------------------------------------------------- |
| `id`           | string  | Session identifier.                                     |
| `supi`         | string  | Subscriber identity.                                    |
| `session_type` | integer | `0` immediate, `1` periodic, `2` triggered.             |
| `method`       | string  | Positioning method.                                     |
| `status`       | integer | `0` active, `1` completed, `2` failed, `3` cancelled.   |
| `created_at`   | integer | Creation time, Unix seconds.                            |
| `updated_at`   | integer | Last update time, Unix seconds.                         |

### Sample Response

```json
{
    "result": [
        {
            "id": "018f9f3a-4c2e-7bd1-9f21-0a1b2c3d4e5f",
            "supi": "imsi-001010000000001",
            "session_type": 1,
            "method": "ecid",
            "status": 0,
            "created_at": 1720000000,
            "updated_at": 1720000000
        }
    ]
}
```

## Get a Positioning Session

This path returns a positioning session, including its most recent location estimate.

| Method | Path                                  |
| ------ | ------------------------------------- |
| GET    | `/api/beta/positioning/sessions/{id}` |

### Path Parameters

| Name | Type   | Description         |
| ---- | ------ | ------------------- |
| `id` | string | Session identifier. |

### Response Fields

| Field                       | Type    | Description                                            |
| --------------------------- | ------- | ----------------------------------------------------- |
| `id`                        | string  | Session identifier.                                   |
| `supi`                      | string  | Subscriber identity.                                  |
| `session_type`              | integer | `0` immediate, `1` periodic, `2` triggered.           |
| `method`                    | string  | Positioning method.                                   |
| `status`                    | integer | `0` active, `1` completed, `2` failed, `3` cancelled. |
| `qos_response_time_ms`      | integer | Requested response-time budget, in milliseconds.      |
| `qos_horizontal_accuracy_m` | integer | Requested horizontal accuracy, in metres.             |
| `last_result`               | object  | Most recent location estimate.                        |
| `created_at`                | integer | Creation time, Unix seconds.                          |
| `updated_at`                | integer | Last update time, Unix seconds.                       |

### Sample Response

```json
{
    "result": {
        "id": "018f9f3a-4c2e-7bd1-9f21-0a1b2c3d4e5f",
        "supi": "imsi-001010000000001",
        "session_type": 1,
        "method": "ecid",
        "status": 1,
        "qos_response_time_ms": 5000,
        "qos_horizontal_accuracy_m": 50,
        "last_result": {
            "locationEstimate": {
                "shape": "POINT_UNCERTAINTY_CIRCLE",
                "point": {
                    "lat": 37.7749,
                    "lon": -122.4194
                },
                "uncertainty": 50
            },
            "accuracyFulfilmentIndicator": "REQUESTED_ACCURACY_FULFILLED"
        },
        "created_at": 1720000000,
        "updated_at": 1720000100
    }
}
```

## Cancel a Positioning Session

This path cancels an active positioning session.

| Method | Path                                  |
| ------ | ------------------------------------- |
| DELETE | `/api/beta/positioning/sessions/{id}` |

### Path Parameters

| Name | Type   | Description         |
| ---- | ------ | ------------------- |
| `id` | string | Session identifier. |

Returns `204 No Content` on success.

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

Returns `204 No Content` on success.
