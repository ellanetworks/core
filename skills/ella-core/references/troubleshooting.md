# Troubleshooting

## HTTP status patterns

| Status | Meaning | What to do |
|--------|---------|------------|
| 400 | Validation error | Read the `error` field — typical messages: `"invalid Var5qi format - must be an integer associated with a non-GBR 5QI"`, `"Invalid group_by parameter"`. Common causes: invalid 5QI, ARP out of range, malformed bitrate string, IMSI not 15 digits, key/opc not 32-char hex, missing required query params (e.g. `group_by` on subscriber-usage). See `references/conventions.md`. |
| 401 | Missing or invalid token | Verify `ELLA_CORE_TOKEN` is set and starts with `ellacore_`. The token may have been revoked — mint a new one. |
| 403 | Authenticated but unauthorized | Token's role is too restricted (see `references/auth.md`). Don't retry — switch tokens. Notably `subscriber:read_credentials` is Admin/NetworkManager only. |
| 404 | Resource not found | Typical message: `"Slice not found"`, `"Subscriber not found"`. List the parent collection to confirm the name/IMSI. Check spelling and case. |
| 409 | Conflict | Typical message: `"Slice already exists"`. The name/IMSI already exists. Pick another, or `GET` the existing resource if that's what the user wanted. |
| 5xx | Server error | Check `GET /api/v1/status`. If healthy, capture the request and recent actions; this likely warrants a bug report. |

Ella Core does not use 422; validation errors come back as 400.

## Cannot reach the instance

1. `curl -sSk "$ELLA_CORE_URL/api/v1/status"` confirms reachability without auth.
2. If it hangs, the URL is wrong or the host is unreachable — confirm with the user.
3. TLS errors: Ella Core typically uses a self-signed certificate. Add `-k` (or `--cacert <file>`) to curl. If you see `http: server gave HTTP response to HTTPS client`, the user is hitting `https://` against a cleartext (`h2c`) instance — switch to `http://`.

## Mutation "succeeds" but state didn't change

Re-read the resource with `GET`. If state didn't change, the API likely silently ignored an unknown field — check the response's `message` and the OpenAPI schema for the correct field names.

## Provisioning fails with "not found" on a parent

Verify the dependency chain (`references/data-model.md`). For example, a policy cannot reference a slice that doesn't exist.
