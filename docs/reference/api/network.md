---
description: RESTful API reference for managing network configuration.
---

# Network

## Update the network configuration

This path updates the network configuration of Ella Core.

| Method | Path              |
| ------ | ----------------- |
| PUT    | `/api/v1/network` |

### Parameters

- `mcc` (string): The Mobile Country Code (MCC) of the network. Must be a 3-digit string.
- `mnc` (string): The Mobile Network Code (MNC) of the network. Must be a 2 or 3-digit string.

### Sample Response

```json
{
    "result": {
        "message": "Network updated successfully"
    }
}
```

## Get network configuration

This path returns the network configuration of Ella Core.

| Method | Path              |
| ------ | ----------------- |
| GET    | `/api/v1/network` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "mcc": "001",
        "mnc": "01"
    }
}
```
