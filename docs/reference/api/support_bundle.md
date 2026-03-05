---
description: RESTful API reference for generating a support bundle.
---

# Support Bundle

Generate a support bundle containing system diagnostics, configuration, and database-derived JSON to help with debugging. Sensitive fields (for example private keys) are redacted where possible; however you should inspect the bundle contents before sharing it with Ella Networks support.

## Generate Support Bundle

| Method | Path |
| ------ | ---- |
| POST | `/api/v1/support-bundle` |

### Parameters

None

### Response

On success the server returns `200` with the body containing a gzipped tar archive and a `Content-Disposition` header recommending a filename like `ella-support-<timestamp>.tar.gz`. The response Content-Type is `application/gzip`.

The archive contains a best-effort collection of relevant diagnostics (database-derived JSON exports, YAML configuration files, and system/network diagnostics). The bundle is intended to be inspected locally before sharing.
