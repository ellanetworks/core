---
description: RESTful API reference for initializing Ella Core.
---

# Initialize

This section describes the RESTful API for initializing Ella Core. Initialization consists of creating the first admin user. This user can then create other users and manage the system.

## Initialize the System

This path initializes the system by creating the first admin user. This endpoint can only be called if no users exist in the system.

| Method | Path            |
| ------ | --------------- |
| POST   | `/api/v1/init` |

### Parameters

- `email` (string): The email of the user.
- `password` (string): The password of the user.

### Sample Response

```json
{
    "result": {
        "message": "System initialized successfully"
    }
}
```
