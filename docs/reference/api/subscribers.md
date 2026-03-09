---
description: RESTful API reference for managing network subscribers.
---

# Subscribers

This section describes the RESTful API for managing network subscribers. Network subscribers are the devices that connect to the private mobile network.

## List Subscribers

This path returns the list of network subscribers.

| Method | Path                  |
| ------ | --------------------- |
| GET    | `/api/v1/subscribers` |

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
                "imsi": "001010100007487",
                "opc": "981d464c7c52eb6e5036234984ad0bcf",
                "sequenceNumber": "16f3b3f70fc7",
                "key": "5122250214c33e723a5dd523fc145fc0",
                "policyName": "default",
                "status": {
                    "registered": true,
                    "sessions": [
                        {
                            "ipAddress": "1.2.3.4"
                        }
                    ]
                }
            }
        ],
        "page": 1,
        "per_page": 10,
        "total_count": 1
    }
}
```

!!! warning "Deprecated fields"
    The `key`, `opc`, and `sequenceNumber` fields in each subscriber item are deprecated and will be removed in a future release.
    Use **GET /api/v1/subscribers/{imsi}/credentials** to retrieve authentication credentials instead.

## Create a Subscriber

This path creates a new network subscriber.

| Method | Path                  |
| ------ | --------------------- |
| POST   | `/api/v1/subscribers` |

### Parameters

- `imsi` (string): The IMSI of the subscriber. Must be a 15-digit string starting with `<mcc><mnc>`.
- `key` (string): The key of the subscriber. Must be a 32-character hexadecimal string.
- `sequenceNumber` (string): The sequence number of the subscriber. Must be a 6-byte hexadecimal string.
- `PolicyName` (string): The policy name of the subscriber. Must be the name of an existing policy.
- `opc` (optional string): The operator code of the subscriber. If not provided, it will be generated automatically using the Operator Code (OP) and the `key` parameter.

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

- `PolicyName` (string): The policy name of the subscriber.

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
        "opc": "981d464c7c52eb6e5036234984ad0bcf",
        "sequenceNumber": "16f3b3f70fc7",
        "key": "5122250214c33e723a5dd523fc145fc0",
        "policyName": "default",
        "status": {
            "registered": true,
            "sessions": [
                {
                    "ipAddress": "1.2.3.4"
                }
            ]
        }
    }
}
```

!!! warning "Deprecated fields"
    The `key`, `opc`, and `sequenceNumber` fields are deprecated and will be removed in a future release.
    Use **GET /api/v1/subscribers/{imsi}/credentials** to retrieve authentication credentials instead.

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
