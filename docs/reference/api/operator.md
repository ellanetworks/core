---
description: RESTful API reference for managing the Operator ID and Operator Code.
---

# Operator

## Update the Operator Information

This path updates the operator information. The Mobile Country Code (MCC) and Mobile Network Code (MNC) are used to identify the operator.

| Method | Path               |
| ------ | ------------------ |
| PUT    | `/api/v1/operator` |

### Parameters

- `mcc` (string): The Mobile Country Code (MCC) of the network. Must be a 3-digit string.
- `mnc` (string): The Mobile Network Code (MNC) of the network. Must be a 2 or 3-digit string.
- `supportedTACs` (array): An array of supported TACs (Tracking Area Codes). Each TAC must be a 3-digit string.

### Sample Response

```json
{
    "result": {
        "message": "Operator updated successfully"
    }
}
```

## Get Operator Information

This path returns the operator Information.

| Method | Path               |
| ------ | ------------------ |
| GET    | `/api/v1/operator` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "mcc": "001",
        "mnc": "01",
        "supportedTACs": [
            "001",
            "002",
            "003"
        ]
    }
}
```

## Update the Operator Code (OP)

This path updates the Operator Code (OP). The OP is a 32-character hexadecimal string that identifies the operator. This value is secret and should be kept confidential. The OP is used to create the derived Operator Code (OPc).

| Method | Path                    |
| ------ | ----------------------- |
| PUT    | `/api/v1/operator/code` |

### Parameters

- `operatorCode` (string): The Operator Code (OP). Must be a 32-character hexadecimal string.

### Sample Response

```json
{
    "result": {
        "message": "Operator Code updated successfully"
    }
}
```
