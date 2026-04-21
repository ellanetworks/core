---
description: RESTful API reference for managing cluster membership.
---

# Cluster

These endpoints are only available when clustering is enabled in the configuration file.

## List Cluster Members

Returns all registered cluster members. This is the static configuration view — who the cluster is configured to be. For the live operational view (who is healthy, how much replication lag, how many failures the cluster can absorb) use [Get Autopilot State](#get-autopilot-state) instead.

| Method | Path                       |
| ------ | -------------------------- |
| GET    | `/api/v1/cluster/members`  |

### Sample Response

```json
{
    "result": [
        {
            "nodeId": 1,
            "raftAddress": "10.0.0.1:7000",
            "apiAddress": "https://10.0.0.1:5000",
            "binaryVersion": "v1.9.1",
            "suffrage": "voter",
            "isLeader": true
        },
        {
            "nodeId": 2,
            "raftAddress": "10.0.0.2:7000",
            "apiAddress": "https://10.0.0.2:5000",
            "binaryVersion": "v1.9.1",
            "suffrage": "voter",
            "isLeader": false
        }
    ]
}
```

## Remove a Cluster Member

Removes a node from the Raft cluster and deletes its record. The node must be drained first (`drainState == "drained"`) or the caller must pass `?force=true`. Requires admin privileges.

| Method | Path                            |
| ------ | ------------------------------- |
| DELETE | `/api/v1/cluster/members/{id}`  |

### Parameters

- `id` (integer, path): Node ID of the cluster member to remove.
- `force` (boolean, query, optional): Bypass the drain precondition. Use only when the drain endpoint cannot reach the target (for example, the node has already been terminated).

### Sample Response

```json
{
    "result": {
        "message": "Cluster member removed"
    }
}
```

## Promote a Cluster Member

Promotes a nonvoter node to a full voter in the Raft cluster. Requires admin privileges.

Autopilot also promotes non-voters automatically once they have been healthy and up-to-date for the server stabilization window (10 seconds). This endpoint is intended for operators who need to promote a node immediately without waiting, for example during a rolling upgrade.

| Method | Path                                    |
| ------ | --------------------------------------- |
| POST   | `/api/v1/cluster/members/{id}/promote`  |

### Parameters

- `id` (integer, path): Node ID of the cluster member to promote.

### Sample Response

```json
{
    "result": {
        "message": "Cluster member promoted to voter"
    }
}
```

## Get Autopilot State

Returns the live autopilot view of the cluster: per‑peer health and replication lag, voter/nonvoter roster, and cluster‑wide `failureTolerance`. Requires admin privileges.

`cluster/members` answers *what the cluster is configured to be*. `cluster/autopilot` answers *how the cluster is actually doing right now*. The two complement each other — the UI polls both.

| Method | Path                         |
| ------ | ---------------------------- |
| GET    | `/api/v1/cluster/autopilot`  |

### Leader‑only; automatic proxy

Autopilot runs only on the leader, so only the leader can produce state. Followers proxy this GET to the leader over the cluster port; the response is returned to the original caller verbatim. Immediately after a leadership change the new leader may return an empty state for a short window (typically under one second) until its first autopilot tick publishes a state.

### Fields

- `healthy` (boolean): True when every voter is healthy according to autopilot's current config.
- `failureTolerance` (integer): How many voter failures the cluster can currently absorb without losing quorum. Zero means any additional failure will stall writes.
- `leaderNodeId` (integer): Raft node ID of the current leader; zero when unknown. Matches the field of the same name on `GET /api/v1/status`.
- `voters` (array of integers): Node IDs of voting members, ascending.
- `servers` (array): Per‑peer state, sorted by `nodeId`.
    - `nodeId` (integer): Raft node ID.
    - `raftAddress` (string): Raft transport address (`host:port`). Matches the field of the same name on `/api/v1/cluster/members`.
    - `nodeStatus` (string): Autopilot lifecycle state — `alive`, `left`, `failed`.
    - `healthy` (boolean): Autopilot's verdict for this peer. Followers flip to unhealthy when their heartbeats stop.
    - `isLeader` (boolean): True when this peer is the current Raft leader.
    - `hasVotingRights` (boolean): True for voters and the leader; false for nonvoters.
    - `stableSince` (string, RFC 3339, optional): Last time this peer's `healthy` value changed. For an unhealthy peer this is when contact was lost. Omitted when unknown.

### Sample Response

```json
{
    "result": {
        "healthy": true,
        "failureTolerance": 1,
        "leaderNodeId": 1,
        "voters": [1, 2, 3],
        "servers": [
            {
                "nodeId": 1,
                "raftAddress": "10.0.0.1:7000",
                "nodeStatus": "alive",
                "healthy": true,
                "isLeader": true,
                "hasVotingRights": true,
                "stableSince": "2026-04-20T08:15:02Z"
            },
            {
                "nodeId": 2,
                "raftAddress": "10.0.0.2:7000",
                "nodeStatus": "alive",
                "healthy": true,
                "isLeader": false,
                "hasVotingRights": true,
                "stableSince": "2026-04-20T08:15:02Z"
            },
            {
                "nodeId": 3,
                "raftAddress": "10.0.0.3:7000",
                "nodeStatus": "left",
                "healthy": false,
                "isLeader": false,
                "hasVotingRights": true,
                "stableSince": "2026-04-20T08:15:02Z"
            }
        ]
    }
}
```

## Drain Cluster Member

Marks the target node as draining and runs the local drain side-effects on it: transfers Raft leadership if the target is the leader, signals connected RANs that this AMF's GUAMI is unavailable (AMF Status Indication, TS 38.413), and stops the local BGP speaker. The target continues serving existing flows. Requires admin privileges.

Drain persists a three-state machine on the cluster_members row (`active` → `draining` → `drained`) that every node can read. A node must be `drained` before it can be removed (see Remove Cluster Member), or the caller must pass `?force=true`.

When `deadlineSeconds` is 0 (default), drain is synchronous: the call returns once side-effects complete and `state` is `drained`. When `deadlineSeconds > 0`, the call returns `state: draining` immediately; a background watcher on the leader flips the state to `drained` once the target's active-lease count reaches zero or the deadline elapses.

In HA mode, drain runs on the leader (followers forward the request automatically). The leader persists drain state through Raft and dispatches the node-local side-effects to the target node over the cluster mTLS port; when the target is the leader itself, the side-effects run inline.

| Method | Path                                            |
| ------ | ----------------------------------------------- |
| POST   | `/api/v1/cluster/members/{id}/drain`            |

### Parameters

- `id` (path, integer, required): Node ID of the cluster member to drain.
- `deadlineSeconds` (body, integer, optional): Seconds to wait for the node's active-lease count to reach zero. 0 = synchronous (default).

### Sample Response

```json
{
    "result": {
        "message": "draining",
        "state": "drained",
        "transferredLeadership": true,
        "ransNotified": 2,
        "bgpStopped": true
    }
}
```

## Resume Cluster Member

Reverses drain on the target node: restarts the local BGP speaker (if BGP is globally enabled) and clears `drainState` back to `active`. Requires admin privileges.

Not reversed by resume:

- **RAN unavailability.** No NGAP message revokes AMF Status Indication; the RAN's next NG Setup re-establishes the GUAMI.
- **Raft leadership.** Leadership transferred during drain stays with the new leader.

Idempotent: resuming an already-active node is a no-op.

| Method | Path                                            |
| ------ | ----------------------------------------------- |
| POST   | `/api/v1/cluster/members/{id}/resume`           |

### Parameters

- `id` (path, integer, required): Node ID of the cluster member to resume.

### Sample Response

```json
{
    "result": {
        "bgpStarted": true
    }
}
```

## Mint Join Token

Mints a single-use HMAC token authorising `nodeID` to request its
first cluster leaf. Requires admin privileges.

| Method | Path                               |
| ------ | ---------------------------------- |
| POST   | `/api/v1/cluster/pki/join-tokens`  |

Request:

```json
{ "nodeID": 2, "ttlSeconds": 1800 }
```

- `nodeID` (int, required): Node-id of the joining host.
- `ttlSeconds` (int, optional): Token lifetime; defaults to 1800.

Response:

```json
{
    "result": {
        "token": "AQAAAPx...",
        "expiresAt": 1714233600
    }
}
```

Put `token` in the joining host's `cluster.join-token` config field
before starting the daemon. The cluster root fingerprint is embedded
in the token.
