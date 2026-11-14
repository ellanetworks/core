# v1.6

Breaking changes

This release contains breaking changes. Review the Breaking changes section below before upgrading. For clusters, follow the [upgrade guide](https://docs.ellanetworks.com/core/how_to/upgrade/index.md).

- **Release date**: March 18, 2026
- **GitHub release**: [v1.6.0](https://github.com/ellanetworks/core/releases/tag/v1.6.0)

## New features

- **Multi Home Network Keys**: Configure Ella Core with multiple Profile A (X25519) and B (B-256) Home network keys. This feature is useful for Key rotation and increases Ella Core compatibility in more environments.
- **NAS Encryption configuration**: Configure which algorithms are supported by Ella Core for NAS subscriber messages.
- **Service Provider Name (SPN) Configuration**: Configure the Service Provider Name sent to subscribers. This network name is displayed in commercial phones.

## Maintenance

- **Security fixes**: This release includes many security improvements at the API, Radio, and subscriber layers.
- Bug fixes
- Dependency updates

## Breaking changes

This change can be breaking for some users. By adding support for multiple home network keys, Ella Core now respects the home network key ID set in the SIM card and rejects registration requests for IDs not associated with a key. If you see registration rejects with the log below, go to the operator page and make sure the ID matches with the one configured in the SIM card.

```
Mar 17 12:06:55 raspberrypi main[4120]: {"level":"warn","ts":"2026-03-17T12:06:55.533-0400","caller":"gmm/handle_registration_request.go:217","msg":"Authentication procedure failed, rejecting registration","component":"AMF","AMF_UE_NGAP_ID":"AMF_UE_NGAP_ID:10","error":"failed to send ue authentication request: ausf UE Authentication Authenticate Request failed: failed to create auth data: couldn't convert suci to supi: home network key not found (scheme=A, keyId=1): not found"}
```

API endpoints used to manage home network keys changed. If you explicitly depend on the API, adapt your script to use the new endpoints. View the Operator API documentation [here](https://docs.ellanetworks.com/core/reference/api/operator/index.md).
