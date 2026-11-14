# Roamers

This section describes the RESTful API for managing [roamers](https://docs.ellanetworks.com/core/explanation/roaming/#roamers): subscribers of a roaming partner registered on the private mobile network.

## List Roamers

This path returns the list of roamers, ordered by IMSI.

| Method | Path              |
| ------ | ----------------- |
| GET    | `/api/v1/roamers` |

### Query Parameters

| Name           | In    | Type | Default | Allowed     | Description                                                         |
| -------------- | ----- | ---- | ------- | ----------- | ------------------------------------------------------------------- |
| `page`         | query | int  | `1`     | `>= 1`      | 1-based page index.                                                 |
| `per_page`     | query | int  | `25`    | `1…100`     | Number of items per page.                                           |
| `partner`      | query | str  |         |             | Filter by roaming partner name.                                     |
| `radio`        | query | str  |         |             | Filter by radio name.                                               |
| `data_network` | query | str  |         |             | Filter by data network name, through the roaming partner's profile. |
| `search`       | query | str  |         | ≤ 254 chars | Filter by IMSI or MSISDN substring.                                 |

### Responses

| Status | Condition                                                  |
| ------ | ---------------------------------------------------------- |
| `200`  | Returns the roamers.                                       |
| `400`  | Invalid parameters.                                        |
| `404`  | The roaming partner, radio or data network does not exist. |

### Sample Response

```
{
    "result": {
        "items": [
            {
                "imsi": "999700000000001",
                "partner": "Partner Telecom",
                "home_plmn": {"mcc": "999", "mnc": "70"},
                "msisdn": "+33612345678",
                "since": "2026-03-16T12:30:02Z",
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
        "per_page": 25,
        "total_count": 1
    }
}
```

## Get a Roamer

This path returns the details of a specific roamer, with its registrations and sessions on the node serving the request.

| Method | Path                     |
| ------ | ------------------------ |
| GET    | `/api/v1/roamers/{imsi}` |

### Parameters

None

### Responses

| Status | Condition                  |
| ------ | -------------------------- |
| `200`  | Returns the roamer.        |
| `400`  | Invalid IMSI.              |
| `404`  | The roamer does not exist. |

### Sample Response

```
{
    "result": {
        "imsi": "999700000000001",
        "partner": "Partner Telecom",
        "home_plmn": {"mcc": "999", "mnc": "70"},
        "msisdn": "+33612345678",
        "since": "2026-03-16T12:30:02Z",
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
            }
        ],
        "sessions": [
            {
                "system": "5G",
                "id": 1,
                "status": "active",
                "ip_type": "IPv4",
                "ipv4_address": "10.45.0.3",
                "data_network": "internet",
                "slice": {
                    "sst": 1
                },
                "ambr_uplink": "100 Mbps",
                "ambr_downlink": "200 Mbps"
            }
        ]
    }
}
```

## Deregister a Roamer

This path deregisters a roamer on the node that serves it and purges it at its home network.

| Method | Path                     |
| ------ | ------------------------ |
| DELETE | `/api/v1/roamers/{imsi}` |

### Parameters

None

### Responses

| Status | Condition                                          |
| ------ | -------------------------------------------------- |
| `200`  | The roamer is deregistered.                        |
| `400`  | Invalid IMSI.                                      |
| `404`  | The roamer does not exist.                         |
| `409`  | The roamer was not deregistered within one minute. |

### Sample Response

```
{
    "result": {
        "message": "Roamer deregistered successfully"
    }
}
```
