# Support Bundle

## Generate Support Bundle

| Method | Path                     |
| ------ | ------------------------ |
| POST   | `/api/v1/support-bundle` |

### Parameters

None

### Response

| Property              | Value                                                          |
| --------------------- | -------------------------------------------------------------- |
| Status                | `200`                                                          |
| `Content-Type`        | `application/gzip`                                             |
| `Content-Disposition` | `attachment; filename="ella-support-<YYYYMMDD_HHMMSS>.tar.gz"` |
| Body                  | gzipped tar archive                                            |

Warning

Redaction is limited to the fields listed under [Redacted fields](#redacted-fields). Inspect the archive before sharing it.

### Archive contents

Each member is best-effort: a collector that fails writes an error file in its place, and the request still returns `200`.

| Path           | Contents                                                                                                                                |
| -------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `db.json`      | Database export: `bundle_metadata`, `operator`, `home_network_keys`, `policies`, `networking`, `subscribers`, `ip_leases`, `radio_logs` |
| `config.yaml`  | Runtime configuration file                                                                                                              |
| `amf_ues.json` | Live AMF and SMF UE state                                                                                                               |
| `mme_ues.json` | Live MME UE state                                                                                                                       |
| `system/`      | Version, OS release, kernel, memory, CPU, disk and network diagnostics                                                                  |
| `bpf/`         | eBPF map entries as gzipped NDJSON, with a `_metadata.json` index                                                                       |

### Redacted fields

| Field                            | Value in the bundle                                          |
| -------------------------------- | ------------------------------------------------------------ |
| `operator.OperatorCode`          | `*`                                                          |
| `home_network_keys[].PrivateKey` | `*`                                                          |
| `subscribers[]`                  | `imsi` only; keys, OPc and sequence numbers are not exported |
