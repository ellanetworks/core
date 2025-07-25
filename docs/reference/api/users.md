---
description: RESTful API reference for managing system users.
---

# Users

This section describes the RESTful API for managing system users. System users are used to authenticate with Ella Core and manage the system.

## List Users

This path returns the list of system users.

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
            "email": "admin@ellanetworks.com",
            "role_id": 1
        }
    ]
}
```

## Create a User

This path creates a new system user. The first user can be created without authentication.

| Method | Path            |
| ------ | --------------- |
| POST   | `/api/v1/users` |

### Parameters

- `email` (string): The email of the user.
- `password` (string): The password of the user.
- `role_id` (int): The role ID of the user. Allowed values:
    - 1 (admin): Administrator user with full access to network and system resources.
    - 2 (network manager): Network manager user with full access to network resources.
    - 3 (read only): Read-only user with only read access to network resources.

### Sample Response

```json
{
    "result": {
        "message": "User created successfully"
    }
}
```

## Update a User

This path updates an existing system user.

| Method | Path                    |
| ------ | ----------------------- |
| PUT    | `/api/v1/users/{email}` |

### Parameters

- `role_id` (int): The role of the user. Allowed values:
    - 1 (admin): Administrator user with full access to network and system resources.
    - 2 (network manager): Network manager user with full access to network resources.
    - 3 (read only): Read-only user with only read access to network resources.

### Sample Response

```json
{
    "result": {
        "message": "User updated successfully"
    }
}
```

## Get a User

This path returns the details of a specific system user.

| Method | Path                    |
| ------ | ----------------------- |
| GET    | `/api/v1/users/{email}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "email": "admin@ellanetworks.com",
        "role_id": 1
    }
}
```

## Delete a User

This path deletes a user from Ella Core.

| Method | Path                    |
| ------ | ----------------------- |
| DELETE | `/api/v1/users/{email}` |

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

## Update a User Password

This path updates the password of a specific system user.

| Method | Path                             |
| ------ | -------------------------------- |
| PUT    | `/api/v1/users/{email}/password` |

### Parameters

- `password` (string): The password of the user.

### Sample Response

```json
{
    "result": {
        "message": "User password updated successfully"
    }
}
```
