# Single Sign-On

These paths require Admin.

## Get the OIDC Settings

This path returns the OIDC settings.

| Method | Path           |
| ------ | -------------- |
| GET    | `/api/v1/oidc` |

### Response Fields

| Field                      | Type    | Description                                                                      |
| -------------------------- | ------- | -------------------------------------------------------------------------------- |
| `enabled`                  | boolean | Single sign-on is enabled.                                                       |
| `external_url`             | string  | URL of Ella Core.                                                                |
| `redirect_url`             | string  | Redirect URL to register at the identity provider.                               |
| `post_logout_redirect_url` | string  | Post-logout redirect URL to register at the identity provider.                   |
| `issuer`                   | string  | Issuer URL.                                                                      |
| `client_id`                | string  | Client ID.                                                                       |
| `client_secret_set`        | boolean | A client secret is stored.                                                       |
| `display_name`             | string  | Name on the login button.                                                        |
| `scopes`                   | array   | Scopes. Default `openid`, `email`, `profile`.                                    |
| `session_lifetime_seconds` | int     | Session lifetime. Default `86400`.                                               |
| `trust_unverified_email`   | boolean | Accept emails without `email_verified`.                                          |
| `ca_certificate`           | string  | PEM CA certificates of the identity provider.                                    |
| `ca_certificates`          | array   | `subject` and `not_after` (RFC 3339) of each certificate in `ca_certificate`.    |
| `groups_claim`             | string  | Claim holding the user's groups.                                                 |
| `group_roles`              | array   | `group` to `role_id` mappings. The highest role among the user's groups applies. |

### Sample Response

```
{
    "result": {
        "enabled": true,
        "external_url": "https://core.example.com",
        "redirect_url": "https://core.example.com/api/v1/auth/oidc/callback",
        "post_logout_redirect_url": "https://core.example.com/login",
        "issuer": "https://idp.example.com/realms/ella",
        "client_id": "ella-core",
        "client_secret_set": true,
        "display_name": "Keycloak",
        "scopes": ["openid", "email", "profile"],
        "session_lifetime_seconds": 86400,
        "trust_unverified_email": false,
        "ca_certificate": "",
        "ca_certificates": [],
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

| Field                      | Type    | Required           | Description                                                                                    |
| -------------------------- | ------- | ------------------ | ---------------------------------------------------------------------------------------------- |
| `enabled`                  | boolean | Yes                | `true` is refused unless the discovery document of `issuer` can be fetched.                    |
| `external_url`             | string  | When enabled       | https URL with no path.                                                                        |
| `issuer`                   | string  | When enabled       | https URL with no query or fragment.                                                           |
| `client_id`                | string  | When enabled       | At most 255 characters.                                                                        |
| `client_secret`            | string  | When enabled       | At most 255 characters. Empty keeps the stored secret, unless `issuer` or `client_id` changes. |
| `display_name`             | string  | When enabled       | At most 64 characters.                                                                         |
| `scopes`                   | array   | Yes                | At most 20. Includes `openid` and `email`.                                                     |
| `session_lifetime_seconds` | int     | Yes                | `300` to `2592000`.                                                                            |
| `trust_unverified_email`   | boolean | No                 | Default `false`.                                                                               |
| `ca_certificate`           | string  | No                 | PEM certificates.                                                                              |
| `groups_claim`             | string  | With `group_roles` | Claim name.                                                                                    |
| `group_roles`              | array   | No                 | At most 100 `{group, role_id}` objects. `role_id` is `1` to `3`.                               |
