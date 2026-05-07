# Authentication

## Token types

- **JWT session tokens** — issued by `POST /api/v1/auth/login` for browser/UI sessions. Short-lived; refreshed at `POST /api/v1/auth/refresh`. Not appropriate for agents.
- **API tokens** — long-lived, prefixed `ellacore_`. Use these for agents and automation. They inherit the role of the owning user.

Mint API tokens in the UI under **Users → API Tokens**, or via the API:

- `POST /api/v1/users/me/api-tokens` — for the calling user
- `POST /api/v1/users/{email}/api-tokens` — admin-only, for another user (path parameter is the user's **email**, not an ID)

Identify the calling user with `GET /api/v1/users/me`.

## Roles and permissions

Three roles:

| Role | ID | Can |
|------|----|-----|
| Admin | 1 | All endpoints, including user/operator management |
| Read-Only | 2 | Read-only access to all resources |
| Network Manager | 3 | Manage subscribers, profiles, policies, slices, data networks; read all telemetry |

Permissions are strings of the form `<resource>:<action>`, checked per endpoint. A 403 means the token's role lacks the required permission — switch to a higher-privilege token rather than retrying.

Notable privileged permission: `subscriber:read_credentials` (used by `GET /api/v1/subscribers/{imsi}/credentials`) is granted only to Admin and Network Manager.

## Environment variables

The skill expects:

- `ELLA_CORE_URL` — base URL with no trailing slash (e.g. `http://192.168.1.10:5000`)
- `ELLA_CORE_TOKEN` — full API token including the `ellacore_` prefix

If either is unset, ask the user before making any call.

## Unauthenticated endpoints

These do not require a token:

- `GET /api/v1/status` — health and version
- `GET /api/v1/metrics` — Prometheus metrics
- `GET /api/v1/openapi.yaml` — full OpenAPI 3.1 specification
