---
description: RESTful API reference for configuring single sign-on through OpenID Connect in Ella Core.
---

# Single Sign-On

These paths require an Admin access token from [login](auth.md#log-in). API tokens are refused.

## Get the OIDC Settings

This path returns the OIDC settings.

| Method | Path           |
| ------ | -------------- |
| GET    | `/api/v1/oidc` |

### Response Fields

| Field                      | Type    | Description |
| -------------------------- | ------- | ----------- |
| `enabled`                  | boolean | Single sign-on is enabled. |
| `external_url`             | string  | URL of Ella Core. |
| `redirect_url`             | string  | Redirect URL to register at the identity provider. |
| `issuer`                   | string  | Issuer URL. |
| `client_id`                | string  | Client ID. |
| `client_secret_set`        | boolean | A client secret is stored. |
| `display_name`             | string  | Name on the login button. |
| `scopes`                   | array   | Scopes. Default `openid`, `email`, `profile`. |
| `session_lifetime_seconds` | int     | Session lifetime. Default `86400`. |
| `trust_unverified_email`   | boolean | Accept emails without `email_verified`. |
| `ca_certificate`           | string  | PEM CA certificates of the identity provider. |
| `groups_claim`             | string  | Claim holding the user's groups. |
| `group_roles`              | array   | Ordered `group` to `role_id` mappings. |

### Sample Response

```json
{
    "result": {
        "enabled": true,
        "external_url": "https://core.example.com",
        "redirect_url": "https://core.example.com/api/v1/auth/oidc/callback",
        "issuer": "https://idp.example.com/realms/ella",
        "client_id": "ella-core",
        "client_secret_set": true,
        "display_name": "Keycloak",
        "scopes": ["openid", "email", "profile"],
        "session_lifetime_seconds": 86400,
        "trust_unverified_email": false,
        "ca_certificate": "",
        "groups_claim": "groups",
        "group_roles": [
            {"group": "core-admins", "role_id": 1},
            {"group": "noc", "role_id": 3}
        ]
    }
}
```

## Update the OIDC Settings

This path replaces the OIDC settings.

| Method | Path           |
| ------ | -------------- |
| PUT    | `/api/v1/oidc` |

### Request Body

| Field                      | Type    | Required           | Description |
| -------------------------- | ------- | ------------------ | ----------- |
| `enabled`                  | boolean | Yes                | |
| `external_url`             | string  | When enabled       | https URL with no path. |
| `issuer`                   | string  | When enabled       | https URL with no query or fragment. |
| `client_id`                | string  | When enabled       | At most 255 characters. |
| `client_secret`            | string  | When enabled       | At most 255 characters. Empty keeps the stored secret, unless `issuer` or `client_id` changes. |
| `display_name`             | string  | When enabled       | At most 64 characters. |
| `scopes`                   | array   | Yes                | At most 20. Includes `openid` and `email`. |
| `session_lifetime_seconds` | int     | Yes                | `300` to `2592000`. |
| `trust_unverified_email`   | boolean | No                 | Default `false`. |
| `ca_certificate`           | string  | No                 | PEM certificates. |
| `groups_claim`             | string  | With `group_roles` | Claim name. |
| `group_roles`              | array   | No                 | At most 100 `{group, role_id}` objects. `role_id` is `1` to `3`. |

## Test an Identity Provider

This path checks an identity provider without changing the settings.

| Method | Path                     |
| ------ | ------------------------ |
| POST   | `/api/v1/oidc/discovery` |

### Request Body

| Field            | Type   | Required | Description |
| ---------------- | ------ | -------- | ----------- |
| `issuer`         | string | Yes      | Issuer URL. |
| `ca_certificate` | string | No       | PEM certificates. |

### Response Fields

| Field                        | Type    | Description |
| ---------------------------- | ------- | ----------- |
| `issuer`                     | string  | Issuer URL. |
| `authorization_endpoint`     | string  | Authorization endpoint. |
| `token_endpoint`             | string  | Token endpoint. |
| `token_endpoint_auth_method` | string  | `client_secret_basic` or `client_secret_post`. |
| `userinfo_endpoint`          | string  | UserInfo endpoint. Empty if none. |
| `jwks_uri`                   | string  | JWKS URI. |
| `signing_algorithms`         | array   | ID token signing algorithms. |
| `keys`                       | int     | Number of signing keys. |
| `pkce`                       | boolean | PKCE is supported. |
| `issuer_parameter`           | boolean | RFC 9207 is supported. |
