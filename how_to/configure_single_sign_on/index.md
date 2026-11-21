# Configure Single Sign-On

This guide lets users log in to the Ella Core UI through an OpenID Connect identity provider.

## Prerequisites

- **Ella Core**: reached by users over HTTPS, at a URL with no path. In a cluster, every node runs the same release.
- **Identity provider**: an OpenID Connect provider, reachable from every Ella Core node over HTTPS.
- **Ella Core account**: an Admin that logs in with a password.

The examples use these values:

| Value                                                | Role                                                                    |
| ---------------------------------------------------- | ----------------------------------------------------------------------- |
| `https://core.example.com`                           | Ella Core's external URL                                                |
| `https://core.example.com/api/v1/auth/oidc/callback` | Redirect URL: the external URL followed by `/api/v1/auth/oidc/callback` |
| `https://core.example.com/login`                     | Post-logout redirect URL: the external URL followed by `/login`         |

## 1. Register Ella Core at the identity provider

In the realm users log in to:

1. Go to **Clients** and click **Create client**.
1. Set **Client type** to **OpenID Connect** and **Client ID** to `ella-core`. Click **Next**.
1. Turn on **Client authentication**. Keep **Standard flow** checked. Click **Next**.
1. Set **Valid redirect URIs** to the redirect URL and **Valid post logout redirect URIs** to the post-logout redirect URL. Click **Save**.
1. In the client's **Credentials** tab, copy the **Client Secret**.

In **Users**, turn on **Email verified** for each user.

1. Register a web application, or confidential client, with the redirect URL and the post-logout redirect URL.
1. Create a client secret for it.
1. Allow the `openid`, `email` and `profile` scopes.

If the provider does not send `email_verified`, turn on **Trust email addresses from this provider** in step 2.

## 2. Configure Ella Core

In the UI, go to **Single Sign-On**.

1. In **URLs**, click **Edit**, set **External URL** to `https://core.example.com`, and click **Update**. Check that the **Redirect URL** and **Post-logout redirect URL** are the ones registered in step 1.

1. In **Provider**, click **Edit**, set these fields, and click **Update**:

   | Field                                        | Value                                                                                       |
   | -------------------------------------------- | ------------------------------------------------------------------------------------------- |
   | **Display name**                             | The name on the login button, such as `Keycloak`                                            |
   | **Issuer URL**                               | Keycloak: `https://<keycloak-host>/realms/<realm>`. Otherwise, from the provider.           |
   | **CA certificate**                           | The PEM CA certificate of the identity provider, only if a private CA signs its certificate |
   | **Client ID**                                | Keycloak: `ella-core`. Otherwise, from the provider.                                        |
   | **Client secret**                            | The secret from step 1                                                                      |
   | **Scopes**                                   | `openid email profile`                                                                      |
   | **Session lifetime**                         | `24` hours                                                                                  |
   | **Trust email addresses from this provider** | Off, unless the provider does not send `email_verified`                                     |

## 3. Enable single sign-on

On the **Single Sign-On** page, turn on **Enabled**.

## 4. Log in

1. Give a user access: [create their user](https://docs.ellanetworks.com/core/how_to/manage_single_sign_on_users/#give-a-user-access), or [map their groups to a role](https://docs.ellanetworks.com/core/how_to/manage_single_sign_on_users/#map-groups-to-roles).
1. In a private browser window, open `https://core.example.com`, and click **Log in with** your display name.
1. Log in at the identity provider. Ella Core opens.
