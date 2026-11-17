---
description: RESTful API reference for authentication.
---

# Authentication

This section describes the RESTful API for system user authentication.

## Login

This path logs the user in and sets an `httpOnly` session cookie valid for 30 days.

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
        "message": "Login successful"
        "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MSwiZW1haWwiOiJhZG1pbkBlbGxhbmV0d29ya3MuY29tIiwicm9sZV9pZCI6MSwiZXhwIjoxNzcwNjgwNTA2fQ.1w9fxtLIfwY4sBOCwdXIZtSmDk8YaVEseAQHJ-5rUXI"
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

## Log Out

This path ends the session of the cookie, and its access tokens, and clears the cookie.

| Method | Path                  |
| ------ | --------------------- |
| POST   | `/api/v1/auth/logout` |

## Lookup a JWT Token

This path returns whether a JWT token is valid. The token must be sent in the `Authorization` header.

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

## Rotate Secret

Generates a new JWT signing secret. All existing user sessions are immediately invalidated. API tokens (prefixed `ellacore_`) are not affected. Requires admin role.

| Method | Path                           |
| ------ | ------------------------------ |
| POST   | `/api/v1/auth/rotate-secret`   |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "Secret rotated successfully. All user sessions have been invalidated."
    }
}
```

## Get the Single Sign-On Option

This path returns whether single sign-on is enabled.

| Method | Path                |
| ------ | ------------------- |
| GET    | `/api/v1/auth/oidc` |

### Sample Response

```json
{
    "result": {
        "enabled": true,
        "display_name": "Keycloak"
    }
}
```

## Log In Through the Identity Provider

This path redirects the browser to the identity provider.

| Method | Path                      |
| ------ | ------------------------- |
| GET    | `/api/v1/auth/oidc/login` |

## Return From the Identity Provider

This path is the redirect URL. On success, it sets the session cookie and redirects to `/`. Otherwise, it redirects to `/login?error=<error>`.

| Method | Path                         |
| ------ | ---------------------------- |
| GET    | `/api/v1/auth/oidc/callback` |

### Errors

| Error                | Description |
| -------------------- | ----------- |
| `rate_limited`       | Too many login attempts. |
| `unavailable`        | Single sign-on is disabled, or the identity provider is unreachable. |
| `failed`             | The login is invalid or expired. |
| `denied`             | The identity provider refused access. |
| `not_set_up`         | No user matches the email. |
| `email_not_verified` | The email is not verified. |
| `linked_elsewhere`   | The user is linked to another account. |
| `no_group`           | No group of the user is mapped to a role. |
| `groups_overage`     | The identity provider did not list the groups. |
| `too_many_users`     | The user limit is reached. |
| `error`              | Internal error. |
