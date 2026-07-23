# Subscribers

This section describes the RESTful API for managing network subscribers. Network subscribers are the devices that connect to the private mobile network.

## List Subscribers

This path returns the list of network subscribers.

| Method | Path                  |
| ------ | --------------------- |
| GET    | `/api/v1/subscribers` |

### Query Parameters

| Name           | In    | Type | Default | Allowed | Description                                                                      |
| -------------- | ----- | ---- | ------- | ------- | -------------------------------------------------------------------------------- |
| `page`         | query | int  | `1`     | `>= 1`  | 1-based page index.                                                              |
| `per_page`     | query | int  | `25`    | `1…100` | Number of items per page.                                                        |
| `radio`        | query | str  |         |         | Filter by radio name. Returns only subscribers connected to the specified radio. |
| `data_network` | query | str  |         |         | Filter by data network name. Returns only subscribers whose profile reaches it.  |

### Sample Response

```
{
    "result": {
        "items": [
            {
                "imsi": "001010100007487",
                "profile_name": "default",
                "status": {
                    "registered": true,
                    "radio_access_type": "5G",
                    "num_sessions": 1
                }
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
}
```

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

### Sample Response

```
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

### Sample Response

```
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

```
{
  "result": {
    "imsi": "001010100007487",
    "profile_name": "default",
    "status": {
      "registered": true,
      "radio_access_type": "5G",
      "imei": "359881234567890",
      "ciphering_algorithm": "SNOW3G",
      "integrity_algorithm": "SNOW3G",
      "last_seen_at": "2026-03-16T12:34:56Z",
      "last_seen_radio": "gNB-1"
    },
  "sessions": [
      {
        "radio_access_type": "5G",
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

## Get Subscriber Credentials

This path returns the authentication credentials for a specific subscriber. The response includes the subscriber's permanent key, OPc, and sequence number. This is the preferred way to retrieve credentials and replaces the deprecated fields on the List and Get responses.

An audit log entry is created each time credentials are viewed.

| Method | Path                                     |
| ------ | ---------------------------------------- |
| GET    | `/api/v1/subscribers/{imsi}/credentials` |

### Parameters

None

### Sample Response

```
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

```
{
    "result": {
        "message": "Subscriber deleted successfully"
    }
}
```
