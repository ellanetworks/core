# Manage Single Sign-On Users

This guide manages the users who log in through single sign-on. [Configure single sign-on](https://docs.ellanetworks.com/core/how_to/configure_single_sign_on/index.md) first.

## Give a user access

Create a user so that they can log in through the identity provider.

1. In the UI, go to **Users** and click **Create**. Set:
   - **Email**: the user's email address at the identity provider
   - **Password**: off
   - **Role**: the user's role
1. Click **Create**.

## Map groups to roles

Map groups at the identity provider to roles so that their members get a role, and a user, when they log in.

### 1. Send groups to Ella Core

1. Go to **Clients**, open the `ella-core` client, and open the **Client scopes** tab.
1. Click `ella-core-dedicated`, then **Configure a new mapper**, or **Add mapper** > **By configuration**, then **Group Membership**.
1. Set **Name** and **Token Claim Name** to `groups`. Turn off **Full group path**. Keep **Add to ID token** on.
1. Click **Save**.

Configure the provider to send the user's groups in a claim, such as `groups`, as a list of group names or IDs.

### 2. Map the groups in Ella Core

1. Go to **Single Sign-On** and, in **Roles from groups**, click **Edit**.
1. Set **Groups claim** to `groups`.
1. Click **Add group** for each group, and set the group and its role.
1. Click **Update**.

## Link a user who has a password

Link a user who has a password to their account at the identity provider.

1. Log in to Ella Core with the password.
1. Go to **Profile** and click **Link** *display name* **account**.
1. Log in at the identity provider.

## Unlink a user

Unlink a user from their account at the identity provider.

1. Go to **Users** and open the user.
1. In **Account**, click **Unlink**, then **Unlink**.

## Offboard a user

Offboard a user to end their access to Ella Core.

1. Remove or disable the user at the identity provider.
1. In Ella Core, go to **Users**, open the user, and click **Delete**.

To keep the user instead, open the user and:

1. In **Account**, click **Unlink**, then **Unlink**.
1. Click **Remove Password**, then **Remove**, if they have one.
1. In **API Tokens**, delete each token.
