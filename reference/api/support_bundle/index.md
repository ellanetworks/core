# Support Bundle

Generate a support bundle containing system diagnostics, configuration, and database-derived JSON to help with debugging. Sensitive fields (for example private keys) are redacted where possible; however you should inspect the bundle contents before sharing it with Ella Networks support.

## Generate Support Bundle

| Method | Path                     |
| ------ | ------------------------ |
| POST   | `/api/v1/support-bundle` |

### Parameters

None

### Response

On success the server returns `200` with the body containing a gzipped tar archive and a `Content-Disposition` header recommending a filename like `ella-support-<timestamp>.tar.gz`. The response Content-Type is `application/gzip`.

The archive contains a best-effort collection of relevant diagnostics (database-derived JSON exports, YAML configuration files, system/network diagnostics, and eBPF maps data). The bundle is intended to be inspected locally before sharing.

## Bundle Contents

### Database and Configuration

- `db.json`: Database export containing operator configuration, policies, data networks, and subscriber information (with sensitive fields redacted)
- `config.yaml`: Runtime configuration file
- `system/`: System information including version, OS release, kernel version, memory, CPU, disk space, and network diagnostics

### eBPF Maps

The bundle includes eBPF map data in a `bpf/` directory (best-effort):

- Each map is exported as compressed NDJSON (`mapname.ndjson.gz`) with decoded key/value entries using generated Go struct types for accurate field representation
- Each map includes a corresponding `mapname_metadata.json` file containing:
- Map name, type (Hash, Array, RingBuf, etc.), key/value sizes
- Number of entries reported and whether entries were truncated
- Snapshot timestamp
- Any error encountered during export

**Default configuration:**

- **Excluded maps**: `nat_ct`, `flow_stats` (excluded due to potentially large size)
- **Max entries per map**: `10000` (entries beyond this limit are truncated with `truncated: true`)
- **Ring buffer maps**: Automatically skipped (cannot be iterated); includes `no_neigh_map` and `nocp_map` ringbuf variant

If BPF map export fails, an error file `bpf/error.txt` is included in the bundle instead.
