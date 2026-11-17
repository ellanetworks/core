---
description: Step-by-step instructions to let users log in to Ella Core through an OpenID Connect identity provider, such as Keycloak.
---

# Configure Single Sign-On

This guide lets users log in to the Ella Core UI through an OpenID Connect identity provider.

## Prerequisites

- **Ella Core**: reached by users over HTTPS, at a URL with no path. In a cluster, every node runs the same release.
- **Identity provider**: an OpenID Connect provider, reachable from every Ella Core node over HTTPS.
- **Ella Core account**: an Admin that logs in with a password.

The examples use these values:

| Value | Role |
|---|---|
| `https://core.example.com` | Ella Core's external URL |
| `https://core.example.com/api/v1/auth/oidc/callback` | Redirect URL: the external URL followed by `/api/v1/auth/oidc/callback` |
| `https://core.example.com/login` | Post-logout redirect URL: the external URL followed by `/login` |

## 1. Register Ella Core at the identity provider

=== "Keycloak"

    In the realm users log in to:

    1. Go to **Clients** and click **Create client**.
    2. Set **Client type** to **OpenID Connect** and **Client ID** to `ella-core`. Click **Next**.
    3. Turn on **Client authentication**. Keep **Standard flow** checked. Click **Next**.
    4. Set **Valid redirect URIs** to the redirect URL and **Valid post logout redirect URIs** to the post-logout redirect URL. Click **Save**.
    5. In the client's **Credentials** tab, copy the **Client Secret**.

    In **Users**, turn on **Email verified** for each user.

=== "Other providers"

    1. Register a web application, or confidential client, with the redirect URL and the post-logout redirect URL.
    2. Create a client secret for it.
    3. Allow the `openid`, `email` and `profile` scopes.

    If the provider does not send `email_verified`, turn on **Trust email addresses from this provider** in step 2.

## 2. Configure Ella Core

In the UI, go to **Single Sign-On** and, in **Provider**, click **Edit**. Set:

| Field | Value |
|---|---|
| **External URL** | `https://core.example.com` |
| **Issuer URL** | Keycloak: `https://<keycloak-host>/realms/<realm>`. Otherwise, from the provider. |
| **CA certificate** | The PEM CA certificate of the identity provider, only if a private CA signs its certificate |
| **Client ID** | Keycloak: `ella-core`. Otherwise, from the provider. |
| **Client secret** | The secret from step 1 |
| **Display name** | The name on the login button, such as `Keycloak` |
| **Scopes** | `openid email profile` |
| **Session lifetime** | `24` hours |
| **Trust email addresses from this provider** | Off, unless the provider does not send `email_verified` |

Check that the **Redirect URL** and **Post-logout redirect URL** are the ones registered in step 1.

## 3. Test the identity provider

In the dialog, click **Test**. Check that it shows **Works**, then click **Update**.

## 4. Enable single sign-on

On the **Single Sign-On** page, turn on **Enabled**.

## 5. Log in

1. Give a user access: [create their user](manage_single_sign_on_users.md#give-a-user-access), or [map their groups to a role](manage_single_sign_on_users.md#map-groups-to-roles).
2. In a private browser window, open `https://core.example.com`, and click **Log in with** your display name.
3. Log in at the identity provider. Ella Core opens.
