---
name: ella-core-api
description: >
  Query and manage a live Ella Core 5G private network instance via its REST API.
  Use when the user asks about subscribers, data usage, QoS policies, radios,
  data networks, routes, NAT, flow reports, audit logs, operator configuration,
  or any runtime state of the Ella Core network. Also use when the user asks to
  provision, update, or delete network resources.
---

# Ella Core API Skill

## Connection details

Before making any API call, you need two pieces of information:

1. **Base URL** — the address of the Ella Core instance (e.g. `http://192.168.1.10:5000`)
2. **API Token** — a token prefixed with `ellacore_`

**You MUST ask the user to provide both values before proceeding.**

Authenticate every request with the header:

```
Authorization: Bearer <API_TOKEN>
```

Exception: `GET /api/v1/status`, `GET /api/v1/metrics`, and `GET /api/v1/openapi.yaml` require no authentication.

## Discovering the API

The complete OpenAPI 3.1 specification is served unauthenticated at:

```bash
curl -s <BASE_URL>/api/v1/openapi.yaml
```

**Always fetch this spec first** when you need to understand an endpoint's parameters, request body, or response schema. The spec is the source of truth — do not guess endpoint shapes.

## How to call the API

Use `curl` in the terminal. Always use `-s` for silent mode and pipe JSON responses through `python3 -m json.tool` or `jq` for readability.

### Read example

```bash
curl -s -H "Authorization: Bearer <API_TOKEN>" \
  "<BASE_URL>/api/v1/subscribers?page=1&per_page=25" | python3 -m json.tool
```

### Write example

```bash
curl -s -X POST -H "Authorization: Bearer <API_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"name":"my-policy","bitrate_uplink":"50 Mbps","bitrate_downlink":"100 Mbps","var5qi":9,"arp":1,"data_network_name":"internet"}' \
  "<BASE_URL>/api/v1/policies" | python3 -m json.tool
```

## Response envelope

All JSON responses are wrapped:

- **Success**: `{"result": <payload>}`
- **Error**: `{"error": "message"}`
- **Mutation success**: `{"result": {"message": "..."}}`

## Pagination

List endpoints accept `page` (default 1) and `per_page` (default 25, max 100) query parameters.
Responses include `items`, `page`, `per_page`, and `total_count`.

To iterate all items, increment `page` until `page * per_page >= total_count`.

## Important notes

- IMSI values are 15-digit strings (e.g. `"999016992280505"`)
- Permanent keys (K) and OPc are 32-character hex strings
- Dates use `YYYY-MM-DD` format for usage queries
- Bitrate strings use format like `"100 Mbps"` or `"1 Gbps"`
- 5QI valid values: 5, 6, 7, 8, 9, 69, 70, 79, 80
- ARP range: 1-15
- Policy, data network, and subscriber names/IMSIs must be unique
- Usage endpoints return byte counts as raw integers. When presenting data volumes to users, convert to binary units: divide by 1,048,576 for MiB or 1,073,741,824 for GiB. Always display values rounded to one decimal place with the unit suffix (e.g. "46.2 MiB", "1.3 GiB"). Verify that uplink + downlink = total before presenting.
