# Authentication

This section describes the RESTful API for system user authentication.

## Login

This path logs the user in and sets an httpOnly session cookie valid for 30 days.

| Method | Path                 |
| ------ | -------------------- |
| POST   | `/api/v1/auth/login` |

### Parameters

- `email` (string): The email to authenticate with.
- `password` (string): The password to authenticate with.

### Sample Response

```
{
    "result": {
        "message": "Login successful"
        "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwiZW1haWwiOiJhZG1pbkBlbGxhbmV0d29ya3MuY29tIiwicm9sZV9pZCI6MSwiZXhwIjoxNzcwNjgwNTA2fQ.1w9fxtLIfwY4sBOCwdXIZtSmDk8YaVEseAQHJ-5rUXI"
    }
}
```

## Refresh

This path validates the current session cookie and returns a new JWT token. This token can then be used to authenticate future requests by sending it in the `Authorization` header using the `Bearer <token>` scheme. This token is valid for 15 minutes.

| Method | Path                   |
| ------ | ---------------------- |
| POST   | `/api/v1/auth/refresh` |

Warning

Avoid relying on refresh tokens for API access since they require regular renewals. Instead, use [API tokens](https://docs.ellanetworks.com/reference/api/users/#create-an-api-token) which offer explicit expiry settings and can be manually revoked.

### Parameters

None

### Sample Response

```
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

```
{
    "result": {
        "valid": true,
    }
}
```

## Rotate Secret

Generates a new JWT signing secret. All existing user sessions are immediately invalidated — users must re-authenticate. API tokens (prefixed `ellacore_`) are not affected. Requires admin role.

| Method | Path                         |
| ------ | ---------------------------- |
| POST   | `/api/v1/auth/rotate-secret` |

### Parameters

None

### Sample Response

```
{
    "result": {
        "message": "Secret rotated successfully. All user sessions have been invalidated."
    }
}
```
