---
description: RESTful API reference for authentication.
---

# Authentication

This section describes the RESTful API for system user authentication.

## Login

This path logs the user in. It sets an httpOnly session cookie valid for 30 days and returns a short-lived JWT access token (valid for 15 minutes) that can be used immediately to authenticate API requests via the `Authorization: Bearer <token>` header.

| Method | Path                 |
| ------ | -------------------- |
| POST   | `/api/v1/auth/login` |

### Parameters

- `email` (string): The email to authenticate with.
- `password` (string): The password to authenticate with.

### Sample Response

```json
{
    "result": {
        "message": "Login successful",
        "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwidXNlcm5hbWUiOiJhZG1pbiIsImV4cCI6MTczNTU4NTk0MX0.0BsZVMLCzJ6mzCXlf3qfAR2k6Fk7aUsGfHV7Tj1Dqy4"
    }
}
```

## Refresh

This path validates the current session cookie and returns a new JWT token. This token can then be used to authenticate future requests by sending it in the `Authorization` header using the `Bearer <token>` scheme. This token is valid for 15 minutes.

| Method | Path                 |
| ------ | -------------------- |
| POST   | `/api/v1/auth/refresh` |

!!! warning
    Avoid relying on refresh tokens for API access since they require regular renewals. Instead, use [API tokens](users.md#create-an-api-token) which offer explicit expiry settings and can be manually revoked.

### Parameters

None

### Sample Response

```json
{
    "result": {
        "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwidXNlcm5hbWUiOiJhZG1pbiIsImV4cCI6MTczNTU4NTk0MX0.0BsZVMLCzJ6mzCXlf3qfAR2k6Fk7aUsGfHV7Tj1Dqy4"
    }
}
```

## Lookup a JWT Token

This path returns whether a JWT token is valid. The token must be sent in the `Authorization` header, like other authenticated requests.

| Method | Path                        |
| ------ | --------------------------- |
| POST   | `/api/v1/auth/lookup-token` |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "valid": true,
    }
}
```
