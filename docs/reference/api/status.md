---
description: RESTful API reference for getting the system status.
---

# Status

## Get the status

This path returns the status of Ella core


| Method | Path             |
| ------ | ---------------- |
| GET    | `/api/v1/status` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "version": "v0.2.0",
        "initialized": true
    }
}
```
