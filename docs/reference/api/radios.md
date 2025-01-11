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
| GET    | `/api/v1/radios` |

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "name": "gnb1",
            "id": "001:01:000102",
            "address": "10.1.107.203/192.168.251.5:9487",
            "supported_tais": [
                {
                    "tai": {
                        "plmnId": {
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
                        "plmnId": {
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
    ]
}
```

## Get a Radio

This path returns the details of a specific radio in the inventory.

| Method | Path                    |
| ------ | ----------------------- |
| GET    | `/api/v1/radios/{name}` |

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
                    "plmnId": {
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
                    "plmnId": {
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
