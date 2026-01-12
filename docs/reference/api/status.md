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
        "version": "v1.0.0",
        "revision": "388ce92244a0b304e9f6c15e3f896acee6fe7b1a",
        "initialized": true
    }
}
```
