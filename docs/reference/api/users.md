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

## Create an API Token

This path creates a new API token for the authenticated user. The API token can be used to authenticate with Ella Core's RESTful API. The API token will have the same permissions as your user account. Actions performed with the token will be logged under your user account.

| Method | Path                          |
| ------ | ----------------------------- |
| POST   | `/api/v1/users/me/api-tokens` |

### Parameters

- `name` (string): The name of the API token.
- `expires_at` (string, optional): The expiration date of the API token in RFC 3339 format. If not provided, the token will never expire.

### Sample Response

```json
{
    "result": {
        "token": "ellacore_Xl2yU1rcy2BP_8q5iOpNBtoXLYdwddbBCHInx"
    }
}
```

!!! note
    The API token is only returned once when created. Make sure to copy it and store it securely.

## List API Tokens

This path returns the list of API tokens for the authenticated user.

| Method | Path                          |
| ------ | ----------------------------- |
| GET    | `/api/v1/users/me/api-tokens` |

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "id": "Xl2yU1rcy2BP",
            "name": "My Token",
            "expires_at": "2024-12-31T23:59:59Z"
        }
    ]
}
```

## Delete an API Token

This path deletes an API token for the authenticated user.

| Method | Path                                    |
| ------ | --------------------------------------- |
| DELETE | `/api/v1/users/me/api-tokens/{tokenID}` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "API token deleted successfully"
    }
}
```
