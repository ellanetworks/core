---
description: RESTful API reference for configuring single sign-on through OpenID Connect in Ella Core.
---

# Single Sign-On

These paths configure login to the UI through an OpenID Connect (OIDC) identity provider. They require Admin, and an access token from [login](auth.md#login): an API token gets `403`. Until every node of a cluster runs a release with single sign-on, `PUT` returns `503`, and `GET` returns the initial settings.

## Get the OIDC Settings

This path never returns the client secret.

| Method | Path            |
| ------ | --------------- |
| GET    | `/api/v1/oidc`  |

### Response Fields

| Field                      | Type    | Description |
| -------------------------- | ------- | ----------- |
| `enabled`                  | boolean | Users can log in through the identity provider. |
| `external_url`             | string  | The URL users reach Ella Core at. |
| `redirect_url`             | string  | The redirect URL to register at the identity provider. Empty without an external URL. |
| `issuer`                   | string  | The identity provider's issuer URL. |
| `client_id`                | string  | The client ID registered at the identity provider. |
| `client_secret_set`        | boolean | A client secret is stored. |
| `display_name`             | string  | The name on the login button. |
| `scopes`                   | array   | The scopes requested. Initially `openid`, `email` and `profile`. |
| `session_lifetime_seconds` | int     | How long a session started through the identity provider lasts. Initially `86400`. |
| `trust_unverified_email`   | boolean | Email addresses are accepted when the identity provider does not send `email_verified`. An `email_verified` of `false` is always refused. |
| `ca_certificate`           | string  | PEM certificates the identity provider's TLS certificate must chain to. Empty: the system's. |
| `groups_claim`             | string  | The claim, in the ID token or UserInfo, that holds the user's groups. |
| `group_roles`              | array   | Group-to-role mappings, in order. |

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

This path replaces the settings, which take effect at once. Disabling OIDC or changing the issuer ends all sessions started through the identity provider.

| Method | Path            |
| ------ | --------------- |
| PUT    | `/api/v1/oidc`  |

### Request Body

| Field                      | Type    | Required        | Description |
| -------------------------- | ------- | --------------- | ----------- |
| `enabled`                  | boolean | Yes             | |
| `external_url`             | string  | When enabled    | An https URL with no path. In a cluster, the load balancer's URL. Stored as `https://host[:port]`. |
| `issuer`                   | string  | When enabled    | An https URL with no query or fragment, as the identity provider gives it. |
| `client_id`                | string  | When enabled    | At most 255 characters. |
| `client_secret`            | string  | When enabled and none is stored, or when `issuer` or `client_id` changes | At most 255 characters. Empty or absent: the stored secret is kept. |
| `display_name`             | string  | When enabled    | At most 64 characters. |
| `scopes`                   | array   | Yes             | At most 20, including `openid` and `email`. |
| `session_lifetime_seconds` | int     | Yes             | `300` to `2592000` (30 days). |
| `trust_unverified_email`   | boolean | No              | Default `false`. |
| `ca_certificate`           | string  | No              | PEM certificates. |
| `groups_claim`             | string  | With `group_roles` | The claim name, such as `groups`. |
| `group_roles`              | array   | No              | At most 100 objects with a `group`, matched exactly, and a `role_id`, `1` to `3`. Each group at most once. |

### Group Roles

| `group_roles` | At each login through the identity provider |
| ------------- | ------------------------------------------- |
| Empty         | An Admin sets the role. Someone with no user in Ella Core is refused. |
| Not empty     | The first mapping whose group the user is in sets the role. A user in none is refused. Someone with no user in Ella Core gets one, with that role, if fewer than 50 users exist. A [pinned](users.md#update-a-user) role is kept, whatever the groups. The last Admin with a password stays Admin. |

### Response Codes

| Code  | Description |
| ----- | ----------- |
| `200` | Updated. The response holds the new settings. |
| `400` | Invalid field, or `issuer` or `client_id` changes without a `client_secret`. |

## Test an Identity Provider

This path checks that Ella Core can use an identity provider. It changes no setting.

| Method | Path                      |
| ------ | ------------------------- |
| POST   | `/api/v1/oidc/discovery`  |

### Request Body

| Field            | Type   | Required | Description |
| ---------------- | ------ | -------- | ----------- |
| `issuer`         | string | Yes      | The issuer URL to test. |
| `ca_certificate` | string | No       | PEM certificates to trust for the provider. |

### Response Fields

| Field                    | Type    | Description |
| ------------------------ | ------- | ----------- |
| `issuer`                 | string  | The provider's issuer. |
| `authorization_endpoint` | string  | |
| `token_endpoint`         | string  | |
| `token_endpoint_auth_method` | string | `client_secret_basic` or `client_secret_post`: how Ella Core sends the client secret. |
| `userinfo_endpoint`      | string  | Empty if the provider has none. |
| `jwks_uri`               | string  | |
| `signing_algorithms`     | array   | The algorithms the provider signs ID tokens with. |
| `keys`                   | int     | The number of signing keys. |
| `pkce`                   | boolean | The provider supports PKCE. |
| `issuer_parameter`       | boolean | The provider supports RFC 9207. |

### Response Codes

| Code  | Description |
| ----- | ----------- |
| `200` | The provider can be used. |
| `400` | Invalid issuer or CA certificate. |
| `502` | The provider cannot be reached or cannot be used, for example because its token endpoint supports neither `client_secret_basic` nor `client_secret_post`. The error says why. |
