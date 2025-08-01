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

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "imsi": "001010100007487",
            "ipAddress": "",
            "opc": "981d464c7c52eb6e5036234984ad0bcf",
            "sequenceNumber": "16f3b3f70fc7",
            "key": "5122250214c33e723a5dd523fc145fc0",
            "policyName": "default"
        }
    ]
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
        "ipAddress": "",
        "opc": "981d464c7c52eb6e5036234984ad0bcf",
        "sequenceNumber": "16f3b3f70fc7",
        "key": "5122250214c33e723a5dd523fc145fc0",
        "policyName": "default"
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
