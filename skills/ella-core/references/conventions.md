# Field formats and valid values

## Identifiers

- **IMSI** — 15-digit string, e.g. `"999016992280505"`. In `CreateSubscriberParams`, the field is `imsi`. Must start with the operator's MCC+MNC.
- **Permanent key (K)** — 32-character hex string. **Field name in the API is `key`** (not `K`).
- **OPc** — 32-character hex string. **Field name in the API is `opc`** (lowercase). Optional on create — derived from operator code if omitted.
- **Sequence number** — 6-byte hex string. **Field name in the API is `sequenceNumber`** (camelCase, unlike most other fields which are snake_case).

## QoS

- **5QI** — valid values: `5, 6, 7, 8, 9, 69, 70, 79, 80`
- **ARP** — integer, range `1-15`
- **AMBR / session bitrate** — string with unit, e.g. `"50 Mbps"`, `"1 Gbps"`, `"500 Kbps"`

## Dates

- Usage queries — `YYYY-MM-DD`

## Subscriber usage

`GET /api/v1/subscriber-usage` **requires** the `group_by` query parameter (`day` or `subscriber`); omitting it returns 400. Optional: `start`, `end` (defaults: 7 days ago → today), `subscriber` (IMSI filter).

Response is `{"result": [<single-key objects>]}` — each object's key is a date (when `group_by=day`) or an IMSI (when `group_by=subscriber`); the value is a `{uplink_bytes, downlink_bytes, total_bytes}` summary.

## Byte counts and presentation

Usage byte fields are `uplink_bytes`, `downlink_bytes`, `total_bytes` (int64). Convert to binary units before presenting:

- MiB = bytes / 1,048,576
- GiB = bytes / 1,073,741,824

Round to one decimal place with the unit suffix (e.g. `46.2 MiB`, `1.3 GiB`). Verify `uplink_bytes + downlink_bytes == total_bytes` before presenting.

## BGP and NAT

- BGP supports up to **5 peers** (`/api/v1/networking/bgp/peers`).
- BGP global state at `/api/v1/networking/bgp` includes `enabled`, `localAS`, `routerID`, `listenAddress`, and `rejectedPrefixes`.
- NAT status at `/api/v1/networking/nat` returns `{"enabled": bool}`. When NAT is enabled, the BGP speaker runs but does **not** advertise subscriber routes.

## Flow reports

`/api/v1/flow-reports` — filterable by subscriber, protocol, source, and destination. Includes both allowed and dropped flows. `/api/v1/flow-reports/stats` returns top protocols and top destination IPs (no required parameters).
