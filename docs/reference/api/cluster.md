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

## Add a Cluster Member

Adds a new node to the Raft cluster and registers it in the cluster members table. In HA mode, follower nodes automatically forward this request to the current leader.

| Method | Path                       |
| ------ | -------------------------- |
| POST   | `/api/v1/cluster/members`  |

### Parameters

- `nodeId` (integer, required): Raft node ID for the new member. Must be a positive integer.
- `raftAddress` (string, required): The `host:port` used for Raft consensus communication.
- `apiAddress` (string, required): The URL used for the REST API.
- `suffrage` (string, optional): Either `"voter"` or `"nonvoter"`. Defaults to `"voter"`.
- `clusterId` (string, optional): Cluster ID to validate against the operator configuration.
- `schemaVersion` (integer, optional): Schema version of the joining node. Rejected if lower than the leader's schema.

### Sample Response

```json
{
    "result": {
        "message": "Cluster member added"
    }
}
```

## Remove a Cluster Member

Removes a node from the Raft cluster and deletes its record. Requires admin privileges.

| Method | Path                            |
| ------ | ------------------------------- |
| DELETE | `/api/v1/cluster/members/{id}`  |

### Parameters

- `id` (integer, path): Node ID of the cluster member to remove.

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
    - `healthy` (boolean): Autopilot's verdict combining last‑contact, last‑term, and log‑lag checks.
    - `isLeader` (boolean): True when this peer is the current Raft leader.
    - `hasVotingRights` (boolean): True for voters and the leader; false for nonvoters.
    - `lastContactMs` (integer): Milliseconds since this peer last heartbeated with the leader. Zero for the leader's own row.
    - `lastTerm` (integer): Highest Raft term this peer has a record of.
    - `lastIndex` (integer): Last Raft log index this peer has acknowledged. Compare against the leader's row to read replication lag.
    - `stableSince` (string, RFC 3339, optional): Last time this peer's `healthy` value changed. Omitted when unknown.

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
                "lastContactMs": 0,
                "lastTerm": 7,
                "lastIndex": 18234,
                "stableSince": "2026-04-20T08:15:02Z"
            },
            {
                "nodeId": 2,
                "raftAddress": "10.0.0.2:7000",
                "nodeStatus": "alive",
                "healthy": true,
                "isLeader": false,
                "hasVotingRights": true,
                "lastContactMs": 12,
                "lastTerm": 7,
                "lastIndex": 18233,
                "stableSince": "2026-04-20T08:15:02Z"
            },
            {
                "nodeId": 3,
                "raftAddress": "10.0.0.3:7000",
                "nodeStatus": "left",
                "healthy": false,
                "isLeader": false,
                "hasVotingRights": true,
                "lastContactMs": 30000,
                "lastTerm": 7,
                "lastIndex": 17500,
                "stableSince": "2026-04-20T08:15:02Z"
            }
        ]
    }
}
```

## Drain Node

Gracefully prepares this node for removal. Signals RANs to redirect new UE registrations elsewhere, withdraws BGP advertisements so upstream routers reroute user‑plane traffic, and — in HA mode only — transfers Raft leadership when this node is the leader. The node continues serving existing flows until it is shut down. Requires admin privileges.

In **single‑node mode** the Raft step is a no‑op (no peer to receive leadership); the RAN and BGP steps still run, which makes drain useful as a pre‑shutdown hook.

Unlike other mutating cluster endpoints, this request is **not** proxied to the leader — it acts on per‑node runtime state (local BGP, local RANs, local Raft participation). You must send it to the specific node you want to drain.

| Method | Path                     |
| ------ | ------------------------ |
| POST   | `/api/v1/cluster/drain`  |

### Parameters

- `timeoutSeconds` (integer, optional): Maximum time in seconds for each drain step. Defaults to 5.

### Sample Response

```json
{
    "result": {
        "message": "draining",
        "transferredLeadership": true,
        "ransNotified": 2,
        "bgpStopped": true
    }
}
```
