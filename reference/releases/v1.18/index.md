# v1.18

Breaking changes

This release contains breaking changes. Review the Breaking changes section below before upgrading. For clusters, follow the [upgrade guide](https://docs.ellanetworks.com/core/how_to/upgrade/index.md).

- **Release date**: September 16, 2026
- **GitHub release**: [v1.18.0](https://github.com/ellanetworks/core/releases/tag/v1.18.0)

## New features

- **Downlink buffering for idle subscribers**: Downlink packets addressed to an idle subscriber are now buffered while Ella Core pages it, and sent to the subscriber once the session is re-established.
- **Handover data forwarding**: Subscribers keep their traffic through 4G and 5G handovers. Ella Core now supports both direct and indirect forwarding, and sends GTP-U End Markers on the old tunnel.
- **Subscriber registrations**: The subscriber API exposes the RAN and core UE identities (RAN/AMF UE NGAP ID, eNB/MME UE S1AP ID). Those can be used for correlating UE events with the radios.
- **Dark mode**: The UI now supports dark mode.
- **Time range picker**: Network events, traffic and audit logs share a common time range picker.
- **Observability overhaul**: Metrics follow Prometheus naming conventions, logs use consistent field names with less noise at info-level, and the eBPF datapath exposes more metrics, including its memory usage in the UI.

## Maintenance

- Standardize radio identity model across 4G and 5G
- Improve co-hosting with OCUDU docs for permanent setups
- Performance improvements
- Security fixes
- Bug fixes
- Dependency updates

## Breaking changes

- Radios are now addressed by their `ref` field: `GET/DELETE /api/v1/ran/radios/{ranNodeType}/{id}` -> `GET/DELETE /api/v1/ran/radios/{ref}` (ex. `gNB:001-01:00002a@24`)
- The deprecated supported TAI field was removed from the radio list endpoint.
- On the subscriber detail endpoint, the merged status object is replaced by a registrations array, `sessions[].radio_access_type` becomes `sessions[].system`, and `ciphering_algorithm` and `integrity_algorithm` report the spec's 128- prefixed names. On the subscriber list, `status.radio_access_types` becomes `status.systems`.
- Time filters changed format. `start` and `end` are now RFC3339 instants. `/subscriber-usage` records are typed objects (`{"date": …}` / `{"imsi": …}`) instead of single-key maps.
- Metrics were renamed to follow Prometheus conventions. `app_sessions_total` -> `app_sessions`, `app_ip_addresses_total` -> `app_ip_addresses`, `app_ip_addresses_allocated_total` -> `app_ip_addresses_allocated`, and `app_upf_pipeline_latency_nanoseconds_total` -> `app_upf_pipeline_latency_seconds_total`.

If you depend directly on the API, you may need to adapt your integration.
