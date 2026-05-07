# Data model and provisioning order

Resources have a strict dependency chain. Always create in this order; verify prerequisites with `GET` before creating dependents.

Resource paths are not all top-level: data networks, BGP, NAT, and routes live under `/api/v1/networking/*`; radios under `/api/v1/ran/*`.

## 1. Data Network

`/api/v1/networking/data-networks` — represents a DNN/APN (e.g. `internet`). Fields: `name`, `ip_pool` (CIDR for UE IP allocation, e.g. `10.45.0.0/22`), `dns`, `mtu`, `status`.

## 2. Slice

`/api/v1/slices` — represents an S-NSSAI. Fields: `name`, `sst` (0–255), optional `sd` (up to 6 hex chars).

## 3. Profile

`/api/v1/profiles` — defines subscriber-level aggregate bitrate caps. Fields: `name`, `ue_ambr_uplink`, `ue_ambr_downlink` (bitrate strings).

## 4. Policy

`/api/v1/policies` — defines per-session QoS and binds a **profile** to a **slice** and a **data network**. A profile can have multiple policies — one per `(slice, data network)` combination, up to **12 per profile**.

Policies may include ordered uplink/downlink network rules (allow/deny with optional prefix, protocol, and port-range filters).

## 5. Subscriber

`/api/v1/subscribers` — a SIM/device identified by IMSI, assigned to a profile. Inherits all policies attached to that profile. **Maximum 1000 subscribers** per instance. Auth credentials (K, OPc, sequence number) are not in the main resource — fetch them separately at `/api/v1/subscribers/{imsi}/credentials`.

## Uniqueness

Names of policies, profiles, slices, and data networks must be unique within their kind. Subscribers are unique by IMSI.

## Deletion order

Reverse of creation. A resource cannot be deleted while another depends on it — delete or reassign dependents first.
