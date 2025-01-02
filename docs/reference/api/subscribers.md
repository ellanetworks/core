---
description: RESTful API reference for managing network subscribers.
---

# Subscribers

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
            "profileName": "default-default"
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
- `opc` (string): The OPC of the subscriber.  Must be a 32-character hexadecimal string.
- `key` (string): The key of the subscriber. Must be a 32-character hexadecimal string.
- `sequenceNumber` (string): The sequence number of the subscriber. Must be a 6-byte hexadecimal string.
- `ProfileName` (string): The profile name of the subscriber. Must be the name of an existing profile.

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

- `opc` (string): The OPC of the subscriber.
- `key` (string): The key of the subscriber.
- `sequenceNumber` (string): The sequence number of the subscriber.
- `ProfileName` (string): The profile name of the subscriber.

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
        "profileName": "default-default"
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
