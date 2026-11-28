# v1.23

Breaking changes

This release contains breaking changes. Review the Breaking changes section below before upgrading. For clusters, follow the [upgrade guide](https://docs.ellanetworks.com/core/how_to/upgrade/index.md).

- **Release date**: November 25, 2026
- **GitHub release**: [v1.23.0](https://github.com/ellanetworks/core/releases/tag/v1.23.0)

## New features

- **Single Sign-On (SSO)**: Users log in to Ella Core with your organization's identity provider (ex. Google, Microsoft, Okta) over OpenID Connect. See the [SSO guide](https://docs.ellanetworks.com/core/how_to/configure_single_sign_on/index.md).
- **100k subscribers**: Ella Core supports up to 100,000 subscribers. A new `datapath.max-sessions` config file option sets the maximum number of concurrent PDU sessions. See the [config file reference](https://docs.ellanetworks.com/core/reference/config_file/index.md).
- **GTP-U fragmentation**: Ella Core reassembles GTP-U packets fragmented by the radio and fragments downlink packets too large for N3, as required by TS 29.281.
- **Radio events setting**: Admins can turn off radio event storage from the Radio Events page or the API.

## Maintenance

- **Helm charts**: Ella Core is published as a Helm chart, including a high-availability deployment, and validated on OpenShift, RKE2, GKE, EKS and Kind. See the [installation guide](https://docs.ellanetworks.com/core/how_to/install/index.md).
- **Signed images**: Container images are signed and attested.
- Performance improvements for large deployments.
- Bug fixes
- Dependency updates

## Breaking changes

- Radios are identified by their `ref` instead of their name in API filters, ex. `GET /api/v1/subscribers?radio={ref}`.

If you depend directly on the API, you may need to adapt your integration.
