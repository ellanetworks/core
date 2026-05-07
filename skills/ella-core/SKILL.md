---
name: ella-core
description: >
  Manage and inspect a running Ella Core 5G private mobile network via its REST API.
  Use when the user asks to provision, update, list, inspect, or delete subscribers,
  profiles, slices, policies, data networks, radios, routes, BGP peers, NAT, operators,
  or users; or asks about live status, data usage, flow reports, audit logs, AMF/SMF/UPF
  state, PDU sessions, gNB connections, IMSIs, S-NSSAIs, DNNs, 5QI, AMBR, ARP, or any
  runtime telemetry of an Ella Core instance.
metadata:
  version: 0.2.0
  author: ellanetworks
---

# Ella Core Skill

Ella Core is a 5G private mobile network packaged as a single Go binary (AMF + SMF + UPF + AUSF). This skill operates a running instance via its REST API.

## Connection

Two values are required:

- **Base URL** — e.g. `http://192.168.1.10:5000`. Read from `ELLA_CORE_URL` if set.
- **API token** — string prefixed with `ellacore_`. Read from `ELLA_CORE_TOKEN` if set.

Ask the user for any value that is not in the environment or earlier in the conversation. Token minting and roles: `references/auth.md`.

Authenticate every request with `Authorization: Bearer $ELLA_CORE_TOKEN`. Exceptions (no auth required): `GET /api/v1/status`, `GET /api/v1/metrics`, `GET /api/v1/openapi.yaml`.

Ella Core typically serves HTTPS with a self-signed certificate. Use `curl -k` (or `--cacert <file>` if the user provides one) for `https://` URLs. Plain HTTP works against `h2c` instances.

## Calling the API

Use `curl -sk` (silent + skip cert verification) and pipe JSON through `jq` (or `python3 -m json.tool`):

```bash
curl -sk -H "Authorization: Bearer $ELLA_CORE_TOKEN" \
  "$ELLA_CORE_URL/api/v1/subscribers?per_page=100" | jq
```

**Retrieval-first.** Before calling any endpoint not already used in this session, check `references/examples.md` for a recipe. If the operation isn't covered, fetch the OpenAPI spec — the source of truth, never guess paths or shapes:

```bash
curl -s "$ELLA_CORE_URL/api/v1/openapi.yaml"
```

Fetch once per session and reuse. Don't pre-fetch resource lists unless you need their IDs — aggregate endpoints often return everything in one call.

## Endpoint namespacing

Many resources live under sub-paths — never assume top-level. Common groups:

- `/api/v1/auth/*` — login, logout, refresh, lookup-token
- `/api/v1/networking/*` — data-networks, routes, NAT, BGP, interfaces
- `/api/v1/ran/*` — radios, events
- `/api/v1/logs/audit` — audit logs
- `/api/v1/operator/*` — operator code, ID, home-network-keys, NAS security
- `/api/v1/cluster/*` — HA cluster management
- `/api/v1/subscribers`, `/api/v1/profiles`, `/api/v1/policies`, `/api/v1/slices` — top-level
- `/api/v1/subscriber-usage`, `/api/v1/flow-reports` — top-level

When in doubt, fetch the OpenAPI spec.

## Response envelope

- Success: `{"result": <payload>}`
- Error: `{"error": "message"}`
- Mutation success (201/200): `{"result": {"message": "..."}}`

## Pagination

List endpoints accept `page` (default 1) and `per_page` (default 25, max 100). Responses include `items`, `page`, `per_page`, `total_count`. Use `per_page=100` when iterating to minimize round-trips.

## Mutations and destructive operations

For any `POST`, `PUT`, `PATCH`, or `DELETE`:

1. **Confirm with the user first.** State what will change and ask for explicit approval. Do not chain destructive calls without re-confirming.
2. **List or GET before delete.** Confirm the target exists and show its current state to the user.
3. **Verify prerequisites before create.** Resources follow a strict dependency chain (data network → slice → profile → policy → subscriber). See `references/data-model.md`.
4. **One change at a time.** Don't bundle unrelated mutations. Re-read state after each one to confirm it landed.

## When to load references

| File | Read when |
|------|-----------|
| `references/auth.md` | Setting up tokens, hitting 401/403, reasoning about RBAC |
| `references/data-model.md` | Provisioning new resources or explaining the data model |
| `references/conventions.md` | Questions about IMSI, K/OPc, 5QI, ARP, AMBR, bitrate strings, byte conversion, BGP/NAT specifics |
| `references/examples.md` | Looking for a recipe before fetching the full OpenAPI |
| `references/troubleshooting.md` | A request fails or returns unexpected data |

## External documentation

For non-API guidance, fetch from `https://docs.ellanetworks.com/` — for example `how_to/ai_agents/`, `how_to/backup_and_restore/`, or `reference/`.
