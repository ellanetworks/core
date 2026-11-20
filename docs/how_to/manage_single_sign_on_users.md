---
description: Step-by-step instructions to give, map, link, unlink and remove the access of users who log in to Ella Core through single sign-on.
---

# Manage Single Sign-On Users

This guide manages the users who log in through single sign-on. [Configure single sign-on](configure_single_sign_on.md) first.

## Give a user access

Create a user so that they can log in through the identity provider.

1. In the UI, go to **Users** and click **Create**. Set:
    - **Email**: the user's email address at the identity provider
    - **Password**: off
    - **Role**: the user's role
2. Click **Create**.

## Map groups to roles

Map groups at the identity provider to roles so that their members get a role, and a user, when they log in.

### 1. Send groups to Ella Core

=== "Keycloak"

    1. Go to **Clients**, open the `ella-core` client, and open the **Client scopes** tab.
    2. Click `ella-core-dedicated`, then **Configure a new mapper**, or **Add mapper** > **By configuration**, then **Group Membership**.
    3. Set **Name** and **Token Claim Name** to `groups`. Turn off **Full group path**. Keep **Add to ID token** on.
    4. Click **Save**.

=== "Other providers"

    Configure the provider to send the user's groups in a claim, such as `groups`, as a list of group names or IDs.

### 2. Map the groups in Ella Core

1. Go to **Single Sign-On** and, in **Roles from groups**, click **Edit**.
2. Set **Groups claim** to `groups`.
3. Click **Add group** for each group, and set the group and its role.
4. Click **Update**.

## Link a user who has a password

Link a user who has a password to their account at the identity provider.

1. Log in to Ella Core with the password.
2. Go to **Profile** and click **Link** *display name* **account**.
3. Log in at the identity provider.

## Unlink a user

Unlink a user from their account at the identity provider.

1. Go to **Users** and open the user.
2. In **Account**, click **Unlink**, then **Unlink**.

## Offboard a user

Offboard a user to end their access to Ella Core.

1. Remove or disable the user at the identity provider.
2. In Ella Core, go to **Users**, open the user, and click **Delete**.

To keep the user instead, open the user and:

1. In **Account**, click **Unlink**, then **Unlink**.
2. Click **Remove Password**, then **Remove**, if they have one.
3. In **API Tokens**, delete each token.
