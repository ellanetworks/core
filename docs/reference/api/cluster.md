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
            "binaryVersion": "v1.17.0",
            "suffrage": "voter",
            "isLeader": true,
            "drainState": "active"
        },
        {
            "nodeId": 2,
            "raftAddress": "10.0.0.2:7000",
            "apiAddress": "https://10.0.0.2:5000",
            "binaryVersion": "v1.17.0",
            "suffrage": "voter",
            "isLeader": false,
            "drainState": "active"
        }
    ]
}
```

## Remove a Cluster Member

This path removes a node from the Raft cluster. The node must be drained first (`drainState == "drained"`) unless `force=true` is set. The current leader cannot be removed regardless of `force`. Must be sent to the leader. Requires admin privileges.

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

This path promotes a nonvoter node to a voter in the Raft cluster. Autopilot promotes healthy nonvoters automatically; use this endpoint to promote immediately. Must be sent to the leader. Requires admin privileges.

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

This path returns the live autopilot view of the cluster: per-peer health, voter roster, and failure tolerance. Requires admin privileges.

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

This path drains a node, moving its subscribers to the rest of the cluster so it can be restarted, upgraded, or removed. Must be sent to the leader. Requires admin privileges.

| Method | Path                                            |
| ------ | ----------------------------------------------- |
| POST   | `/api/v1/cluster/members/{id}/drain`            |

### Parameters

None.

### Sample Response

```json
{
    "result": {
        "drainState": "draining"
    }
}
```

## Resume Cluster Member

This path returns a drained node to service. Idempotent. Must be sent to the leader. Requires admin privileges.

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

This path mints a single-use token authorising `nodeID` to join the cluster. Must be sent to the leader. Requires admin privileges.

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
