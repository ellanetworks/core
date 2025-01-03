---
description: RESTful API reference for authentication.
---

# Authentication

## Login

This path returns a token that can be used to authenticate with Ella Core.

| Method | Path                 |
| ------ | -------------------- |
| POST   | `/api/v1/auth/login` |

### Parameters

- `username` (string): The username to authenticate with.
- `password` (string): The password to authenticate with.

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
