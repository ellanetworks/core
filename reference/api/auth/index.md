# Authentication

This section describes the RESTful API for system user authentication.

## Log In

This path logs the user in and sets an `httpOnly` session cookie valid for 30 days.

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

Avoid relying on refresh tokens for API access since they require regular renewals. Instead, use [API tokens](https://docs.ellanetworks.com/core/reference/api/users/#create-an-api-token) which offer explicit expiry settings and can be manually revoked.

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

## Log Out

This path ends the session of the cookie, and its access tokens, and clears the cookie. For a single sign-on session, it returns the identity provider's logout URL.

| Method | Path                  |
| ------ | --------------------- |
| POST   | `/api/v1/auth/logout` |

### Sample Response

```
{
    "result": {
        "redirect_url": "https://idp.example.com/realms/ella/protocol/openid-connect/logout?client_id=ella-core&post_logout_redirect_uri=https%3A%2F%2Fcore.example.com%2Flogin"
    }
}
```

## Lookup a JWT Token

This path returns whether a JWT token is valid. The token must be sent in the `Authorization` header.

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

Generates a new JWT signing secret. All existing user sessions are immediately invalidated. API tokens (prefixed `ellacore_`) are not affected. Requires admin role.

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

## Get the Single Sign-On Option

This path returns whether single sign-on is enabled.

| Method | Path                |
| ------ | ------------------- |
| GET    | `/api/v1/auth/oidc` |

### Sample Response

```
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

### Query Parameters

- `link` (string, optional): `1` links the identity provider account to the user of the session cookie, and redirects to `/profile`.

## Return From the Identity Provider

This path is the redirect URL. On success, it sets the session cookie and redirects to `/`. Otherwise, it redirects to `/login?error=<error>`.

| Method | Path                         |
| ------ | ---------------------------- |
| GET    | `/api/v1/auth/oidc/callback` |

### Errors

| Error                | Description                                                           |
| -------------------- | --------------------------------------------------------------------- |
| `rate_limited`       | Too many login attempts.                                              |
| `unavailable`        | Single sign-on is disabled, or the identity provider is unreachable.  |
| `failed`             | The login is invalid or expired.                                      |
| `denied`             | The identity provider refused access.                                 |
| `not_set_up`         | No user can be matched.                                               |
| `email_not_verified` | The email is not verified.                                            |
| `linked_elsewhere`   | The user is linked to another account.                                |
| `link_required`      | The user has a password and must link the account from their profile. |
| `no_group`           | No group of the user is mapped to a role.                             |
| `groups_overage`     | The identity provider did not list the groups.                        |
| `too_many_users`     | The user limit is reached.                                            |
| `error`              | Internal error.                                                       |
