# Single Sign-On

Users can log in to the UI through the organization's identity provider instead of, or as well as, a password. Ella Core works with any OpenID Connect identity provider, such as Microsoft Entra ID, Okta, Ping Identity, Google or Keycloak. To set it up, see [Configure Single Sign-On](https://docs.ellanetworks.com/core/how_to/configure_single_sign_on/index.md).

Login page with single sign-on

## Password and single sign-on

|                     | Password                                | Single sign-on                                                                    |
| ------------------- | --------------------------------------- | --------------------------------------------------------------------------------- |
| Credentials kept by | Ella Core                               | The identity provider, with its own policies, such as multi-factor authentication |
| Role set by         | An Admin                                | An Admin, or the highest role from the user's groups at the identity provider     |
| Session             | 30 days                                 | The session lifetime, 24 hours by default                                         |
| Access removed by   | Deleting the user in Ella Core, at once | Removing the user at the identity provider, at the end of their session           |

Password login stays when single sign-on is on, and at least one Admin always keeps a password. An identity provider that is down or misconfigured cannot lock everyone out.

## Matching users

A user is matched to their account at the identity provider, not to their email address, so a changed or reused email address does not give access to another user. The email address is used once, to link the account at its first login, only if the identity provider has verified it and the user has no password. A user with a password links from their profile. A provider that does not say whether email addresses are verified, such as Microsoft Entra ID, must be trusted explicitly: anyone who can set their email address there could take over the matching user.

Email addresses match in any case. If several users have the address in different cases, the one in lower case is linked; with none in lower case, the login is refused. An account with no user in Ella Core gets one only from a [group mapping](https://docs.ellanetworks.com/core/how_to/manage_single_sign_on_users/#map-groups-to-roles), within the limit of 50 users.

## Sessions

An access token ends with its session.

## Offboarding

Ella Core does not learn when someone is removed at the identity provider. Their session lasts until its lifetime ends, and their API tokens until deleted. A shorter session lifetime narrows that window, at the cost of logging in more often. To end access at once, [offboard the user](https://docs.ellanetworks.com/core/how_to/manage_single_sign_on_users/#offboard-a-user) in Ella Core.
