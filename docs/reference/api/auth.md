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

### Response Codes

| Code  | Description |
| ----- | ----------- |
| `401` | Wrong email or password, or the user has no password. |
| `429` | More than 20 attempts in a minute from the client address. |

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

This path validates the current session cookie and returns a new JWT token. This token can then be used to authenticate future requests by sending it in the `Authorization` header using the `Bearer <token>` scheme. This token is valid for 15 minutes, and ends with its session.

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

### Response Codes

| Code  | Description |
| ----- | ----------- |
| `204` | Logged out, or no session. |

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

This path tells the login page whether to show the button to log in through the identity provider.

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

The login page sends the browser to this path, which redirects it to the identity provider. It counts toward the login limit of 20 per minute per address. The login must complete within 10 minutes.

| Method | Path                      |
| ------ | ------------------------- |
| GET    | `/api/v1/auth/oidc/login` |

## Return From the Identity Provider

The identity provider sends the browser back to this path, the redirect URL. Each login completes once, on any node of a cluster. On success, it sets the session cookie, for the [session lifetime](oidc.md#get-the-oidc-settings), and redirects to `/`. Otherwise, it redirects to `/login?error=<error>`, and the `auth_oidc_login` [audit log](audit_logs.md) of the node gives the details.

| Method | Path                         |
| ------ | ---------------------------- |
| GET    | `/api/v1/auth/oidc/callback` |

### Errors

| Error                | Description |
| -------------------- | ----------- |
| `rate_limited`       | Too many logins from the address. |
| `unavailable`        | Single sign-on is off, or the identity provider cannot be reached. |
| `failed`             | The login expired, was already used or is invalid, the single sign-on settings changed during it, or the identity provider returned an error. |
| `denied`             | The user or the identity provider refused access. |
| `not_set_up`         | The identity provider gives no email, no user has it and no group mapping is set, or several users have it in different cases and none in lower case. |
| `email_not_verified` | The identity provider marks the email as not verified, or does not say and [`trust_unverified_email`](oidc.md#update-the-oidc-settings) is off. |
| `linked_elsewhere`   | The user with the email is linked to another account at the identity provider. |
| `no_group`           | None of the user's groups is mapped to a role. |
| `groups_overage`     | The user is in too many groups for the identity provider to list them. |
| `too_many_users`     | 50 users exist. |
| `error`              | Internal error. |
