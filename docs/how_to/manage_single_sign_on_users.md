---
description: Step-by-step instructions to give, map, pin, unlink and remove the access of users who log in to Ella Core through single sign-on.
---

# Manage Single Sign-On Users

These guides manage users who log in through the identity provider. [Configure single sign-on](configure_single_sign_on.md) first.

## Give a user access

1. In the UI, go to **Users** and click **Create**. Set:
    - **Email**: the user's email address at the identity provider
    - **Password**: off
    - **Role**: the user's role
    - **Pin role**: checked to keep this role, if groups are mapped to roles
2. Click **Create**.

The user logs in with **Log in with** the display name. Their first login links their user to their account at the identity provider.

## Map groups to roles

Users in a mapped group get their role from it at each login, and get a user at their first login. Users in no mapped group cannot log in through the identity provider, unless their role is [pinned](#pin-a-users-role).

### 1. Send groups to Ella Core

=== "Keycloak"

    1. Go to **Clients**, open the `ella-core` client, and open the **Client scopes** tab.
    2. Click `ella-core-dedicated`, then **Configure a new mapper**, or **Add mapper** > **By configuration**, then **Group Membership**.
    3. Set **Name** and **Token Claim Name** to `groups`. Turn off **Full group path**. Keep **Add to ID token** on.
    4. Click **Save**.

    The groups are group names, such as `core-admins`.

=== "Other providers"

    Configure the provider to send the user's groups in a claim, such as `groups`, as a list of group names or IDs.

### 2. Map the groups in Ella Core

1. Go to **Single Sign-On** and, in **Roles from groups**, click **Edit**.
2. Set **Groups claim** to `groups`.
3. Click **Add group** for each group, and set the group and its role. Put the most privileged first: a user in several groups gets the role of the first.
4. Click **Update**.

## Pin a user's role

A pinned role is kept whatever the user's groups.

1. Go to **Users** and open the user.
2. In **Account**, edit the role: choose the role, check **Pin role**, and click **Update**. Changing a role that comes from groups checks it for you.

To unpin, uncheck **Pin role**. The role then follows the user's groups from their next login.

## Unlink a user

Unlink a user whose account at the identity provider was replaced, so that their next login links the new account by email.

1. Go to **Users** and open the user.
2. In **Account**, click **Unlink**, then **Unlink**.

## Offboard a user

1. Remove or disable the user at the identity provider. They can no longer log in, but their current session lasts until the **Session lifetime** ends.
2. To end their access at once, go to **Users** in Ella Core, open the user, and click **Delete**. This also deletes their API tokens.

To keep the user instead, open the user and:

1. In **Account**, click **Unlink**, then **Unlink**. This ends their single sign-on sessions.
2. Click **Remove Password**, then **Remove**, if they have one.
3. In **API Tokens**, delete each token.
