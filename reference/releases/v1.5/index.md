# v1.5

- **Release date**: March 9, 2026
- **GitHub release**: [v1.5.0](https://github.com/ellanetworks/core/releases/tag/v1.5.0)

## New features

- **AI skill support**: Use an AI agent to monitor and manage Ella Core using its OpenAPI specification and [SKILL.md](https://github.com/ellanetworks/core/blob/v1.5.0/.github/skills/ella-core-api/SKILL.md). This feature is useful for network monitoring, troubleshooting, security investigations, and more. To enable it:
  - Create a dedicated user for the agent with the appropriate role (e.g. "network manager" for full network access, "read only" for monitoring).
  - Place `SKILL.md` in a directory that your AI agent expects.
  - Ask the agent questions about your 5G network. The agent will prompt you for Ella Core's URL and an API token when needed.
- **Support bundle**: Click on "Support Bundle" and generate a support bundle that contains a summary of Ella Core's configuration and data, with sensitive fields removed. Share this package with Ella Networks when you need support. You can view and edit the content of the bundle before sharing it. This feature is only available to admins.
- **User detail page**: Click on a user ID and manage API tokens, edit roles and view recent audit logs. This page is only available for admins and is especially useful to provision API tokens for other users.
- **Subscriber details page**: Click on a subscriber ID and view provisioning information, connection data and traffic summary. Useful for provisioning, troubleshooting, and audit.
- **Filter audit logs**: Filter audit logs by date or user in the UI or API

## Documentation

- **[Platform security explanation](https://docs.ellanetworks.com/core/explanation/platform_security/index.md)**: Explanation of Authentication & Authorization, Secret storage, Transport security, minimal attack surface and audit logging in Ella Core.
- **[Manage Your Network with AI agents how-to guide](https://docs.ellanetworks.com/core/how_to/ai_agents/index.md)**

## Maintenance

- Bug fixes
- Dependency updates

## Deprecations

- **New endpoint for subscriber credentials**: If you depend on Ella Core's API for retrieving subscriber credentials, switch to `GET /api/v1/subscribers/{imsi}/credentials`. The credential fields in the subscriber struct returned by `GET /api/v1/subscribers/{imsi}` and `GET /api/v1/subscribers` are deprecated and will be removed in a future release.

## Patch releases

### v1.5.2

- **Release date**: March 13, 2026
- **GitHub release**: [v1.5.2](https://github.com/ellanetworks/core/releases/tag/v1.5.2)

#### Improvements

- Quality and consistency improvements ([#1096](https://github.com/ellanetworks/core/pull/1096))
- Require current password on self-change ([#1102](https://github.com/ellanetworks/core/pull/1102))
- Increase eBPF ring buffer map size to 16K ([#1103](https://github.com/ellanetworks/core/pull/1103))

#### Bug fixes

- Backup on systems without /tmp dir ([#1101](https://github.com/ellanetworks/core/pull/1101))

### v1.5.1

- **Release date**: March 12, 2026
- **GitHub release**: [v1.5.1](https://github.com/ellanetworks/core/releases/tag/v1.5.1)

#### Bug fixes

- Use HASH_MAPS instead of ARRAYs for FARs, QERs, and URRs ([#1095](https://github.com/ellanetworks/core/pull/1095))
- Short integrity protected NAS payload ([#1098](https://github.com/ellanetworks/core/pull/1098))
- Length check in path switch request IE ([#1099](https://github.com/ellanetworks/core/pull/1099))
