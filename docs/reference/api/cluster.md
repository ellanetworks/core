---
description: RESTful API reference for managing cluster membership.
---

# Cluster

This section describes the RESTful API for managing cluster membership. These endpoints are only available when clustering is enabled in the configuration file.

## List Cluster Members

This path returns the list of cluster members.

| Method | Path                       |
| ------ | -------------------------- |
| GET    | `/api/v1/cluster/members`  |

### Parameters

None

### Sample Response

```json
{
    "result": [
        {
            "nodeId": 1,
            "raftAddress": "10.0.0.1:7000",
            "apiAddress": "https://10.0.0.1:5000",
            "binaryVersion": "v1.10.0",
            "suffrage": "voter",
            "isLeader": true
        },
        {
            "nodeId": 2,
            "raftAddress": "10.0.0.2:7000",
            "apiAddress": "https://10.0.0.2:5000",
            "binaryVersion": "v1.10.0",
            "suffrage": "voter",
            "isLeader": false
        }
    ]
}
```

## Remove a Cluster Member

This path removes a node from the Raft cluster. The node must be drained first (`drainState == "drained"`) unless `force=true` is set. Requires admin privileges.

| Method | Path                            |
| ------ | ------------------------------- |
| DELETE | `/api/v1/cluster/members/{id}`  |

### Query Parameters

| Name    | In    | Type | Default | Description                                     |
| ------- | ----- | ---- | ------- | ----------------------------------------------- |
| `force` | query | bool | `false` | Bypass the drain precondition.                  |

### Sample Response

```json
{
    "result": {
        "message": "Cluster member removed"
    }
}
```

## Promote a Cluster Member

This path promotes a nonvoter node to a voter in the Raft cluster. Autopilot promotes healthy nonvoters automatically; use this endpoint to promote immediately. Requires admin privileges.

| Method | Path                                    |
| ------ | --------------------------------------- |
| POST   | `/api/v1/cluster/members/{id}/promote`  |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "Cluster member promoted to voter"
    }
}
```

## Get Autopilot State

This path returns the live autopilot view of the cluster: per-peer health, voter roster, and failure tolerance. Only the leader can produce this state; followers proxy the request to the leader automatically. Requires admin privileges.

| Method | Path                         |
| ------ | ---------------------------- |
| GET    | `/api/v1/cluster/autopilot`  |

### Parameters

None

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
            }
        ]
    }
}
```

## Drain Cluster Member

This path marks a node as draining and runs the local drain side-effects: transfers Raft leadership if the target is the leader, signals connected RANs that this AMF's GUAMI is unavailable, and stops the local BGP speaker. A node must be drained before it can be removed. Requires admin privileges.

When `deadlineSeconds` is 0 (default), drain is synchronous and returns once `state` is `drained`. When `deadlineSeconds > 0`, the call returns `state: draining` immediately; a background watcher flips the state to `drained` once the target's active-lease count reaches zero or the deadline elapses.

| Method | Path                                            |
| ------ | ----------------------------------------------- |
| POST   | `/api/v1/cluster/members/{id}/drain`            |

### Parameters

- `deadlineSeconds` (integer, optional): Seconds to wait for the node's active-lease count to reach zero. 0 = synchronous (default).

### Sample Response

```json
{
    "result": {
        "drainState": "drained"
    }
}
```

## Resume Cluster Member

This path reverses drain on a node: restarts the local BGP speaker (if BGP is enabled) and clears `drainState` back to `active`. RAN unavailability and transferred leadership are not reversed. Idempotent. Requires admin privileges.

| Method | Path                                            |
| ------ | ----------------------------------------------- |
| POST   | `/api/v1/cluster/members/{id}/resume`           |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "message": "Cluster member resumed"
    }
}
```

## Mint Join Token

This path mints a single-use HMAC token authorising `nodeID` to request its first cluster leaf certificate. The cluster root fingerprint is embedded in the token. Requires admin privileges.

| Method | Path                               |
| ------ | ---------------------------------- |
| POST   | `/api/v1/cluster/pki/join-tokens`  |

### Parameters

- `nodeID` (integer): Node ID of the joining host.
- `ttlSeconds` (integer, optional): Token lifetime in seconds. Defaults to `1800`.

### Sample Response

```json
{
    "result": {
        "token": "AQAAAPx...",
        "expiresAt": 1714233600
    }
}
```
