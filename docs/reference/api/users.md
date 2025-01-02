---
description: RESTful API reference for managing system users.
---


# Users

## List Users

This path returns the list of system users configured in Ella Core.


| Method | Path            |
| ------ | --------------- |
| GET    | `/api/v1/users` |

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "username": "admin"
        }
    ]
}
```

## Create a User

This path creates a new user in Ella Core. The first user can be created without authentication.

| Method | Path            |
| ------ | --------------- |
| POST   | `/api/v1/users` |

### Parameters

- `username` (string): The username of the user. 
- `password` (string): The password of the user.

### Sample Response

```json
{
    "result": {
        "message": "User created successfully"
    }
}
```

## Update a User

This path updates an existing user in Ella Core.

| Method | Path                       |
| ------ | -------------------------- |
| PUT    | `/api/v1/users/{username}` |

### Parameters

- `password` (string): The password of the user.

### Sample Response

```json
{
    "result": {
        "message": "User updated successfully"
    }
}
```

## Get a User

This path returns the details of a specific user in Ella Core.

| Method | Path                       |
| ------ | -------------------------- |
| GET    | `/api/v1/users/{username}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "username": "guillaume"
    }
}
```

## Delete a User

This path deletes a user from Ella Core.

| Method | Path                       |
| ------ | -------------------------- |
| DELETE | `/api/v1/users/{username}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "User deleted successfully"
    }
}
```
