---
description: RESTful API reference for generating a support bundle.
---

# Support Bundle

## Generate Support Bundle

| Method | Path |
| ------ | ---- |
| POST | `/api/v1/support-bundle` |

### Parameters

None

### Response

| Property | Value |
| -------- | ----- |
| Status | `200` |
| `Content-Type` | `application/gzip` |
| `Content-Disposition` | `attachment; filename="ella-support-<YYYYMMDD_HHMMSS>.tar.gz"` |
| Body | gzipped tar archive |

!!! warning
    Support bundles contain operational data and personally identifiable information (PII). They should only be shared with trusted partners.

### Archive contents

Each member is best-effort: a collector that fails writes an error file in its place, and the request still returns `200`.

| Path | Contents |
| ---- | -------- |
| `db.json` | Database export: `bundle_metadata`, `operator`, `home_network_keys`, `policies`, `networking`, `subscribers`, `ip_leases`, `ims_application_servers`, `ims_service_profiles`, `ims_service_profile_assignments`, `radio_logs` and other operational sections. The `audit_logs` section requires `audit_logs:list`; `usage_per_day` and `usage_per_subscriber` require `subscriber_usage:get`. Restricted sections are omitted and marked in `bundle_metadata.sections`. |
| `config.yaml` | Runtime configuration file |
| `logs/system.log` | Recent in-memory system logs, preserving original messages and identities |
| `collection_status.json` | Status of optional configuration, live UE and BPF collectors; database section status is recorded in `db.json` under `bundle_metadata.sections` |
| `amf_ues.json` | Live AMF and SMF UE state |
| `mme_ues.json` | Live MME UE state |
| `system/` | Version, OS release, kernel, memory, CPU, disk and network diagnostics |
| `bpf/` | eBPF map entries as gzipped NDJSON, with a `_metadata.json` index |

### Redacted fields

| Field | Value in the bundle |
| ----- | ------------------- |
| `operator.OperatorCode` | `*` |
| `home_network_keys[].PrivateKey` | `*` |
| `bgp_peers[].password` | `*` |
| `subscribers[]` | `imsi`, `msisdn` and `profile_id` only; keys, OPc and sequence numbers are not exported |
| `cluster.join-token` (`config.yaml`) | `*` |
| Live UE security state | Curated exports omit authentication vectors and key material |
| Application authentication data | Password hashes, API/session token hashes, signing secrets and cluster authentication keys are not exported |
| `radio_logs[]` | Raw signaling payloads are excluded because they can contain security keys; event metadata and stored details are retained |

System log retention is limited to 100,000 entries and 16 MiB of payload. An entry larger than 64 KiB is replaced with a valid JSON omission marker containing its original byte count. Its original message and fields are not retained in the bundle log buffer.
