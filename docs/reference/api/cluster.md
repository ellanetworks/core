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
            "nodeId": "0199c4f1-2a7e-7b31-9c5d-1f2e3a4b5c6d",
            "displayName": "core-mtl-a",
            "amfPointer": 1,
            "raftAddress": "10.0.0.1:7000",
            "apiAddress": "https://10.0.0.1:5000",
            "binaryVersion": "v1.18.0",
            "suffrage": "voter",
            "isLeader": true,
            "drainState": "active"
        },
        {
            "nodeId": "0199c4f1-8b02-7d44-a1e7-5c6d7e8f9a0b",
            "displayName": "",
            "amfPointer": 2,
            "raftAddress": "10.0.0.2:7000",
            "apiAddress": "https://10.0.0.2:5000",
            "binaryVersion": "v1.18.0",
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

This path promotes a nonvoter node to a voter in the Raft cluster. Must be sent to the leader. Requires admin privileges.

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

This path returns the live autopilot view of the cluster. Requires admin privileges.

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
        "leaderNodeId": "0199c4f1-2a7e-7b31-9c5d-1f2e3a4b5c6d",
        "voters": [
            "0199c4f1-2a7e-7b31-9c5d-1f2e3a4b5c6d",
            "0199c4f1-8b02-7d44-a1e7-5c6d7e8f9a0b"
        ],
        "servers": [
            {
                "nodeId": "0199c4f1-2a7e-7b31-9c5d-1f2e3a4b5c6d",
                "raftAddress": "10.0.0.1:7000",
                "nodeStatus": "alive",
                "healthy": true,
                "isLeader": true,
                "hasVotingRights": true,
                "stableSince": "2026-04-20T08:15:02Z"
            },
            {
                "nodeId": "0199c4f1-8b02-7d44-a1e7-5c6d7e8f9a0b",
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

## Get Cluster Health

This path returns a summary of cluster health. Unlike `/api/v1/cluster/autopilot`, it exposes no per-node identities or addresses and is readable by every role.

This path always returns `200`. When no leader is known, or the leader cannot be reached, the response reports `hasLeader: false` instead of failing, so callers can render a degraded state.

| Method | Path                      |
| ------ | ------------------------- |
| GET    | `/api/v1/cluster/health`  |

### Parameters

None

### Sample Response

```json
{
    "result": {
        "enabled": true,
        "hasLeader": true,
        "healthy": true,
        "healthyVoters": 3,
        "totalVoters": 3,
        "failureTolerance": 1
    }
}
```

## Drain Cluster Member

This path drains a node, moving its subscribers to the rest of the cluster so it can be restarted, upgraded, or removed. Requires admin privileges.

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

This path returns a drained node to service. Idempotent. Requires admin privileges.

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

This path mints a single-use token authorising a host to join the cluster. Must be sent to the leader. Requires admin privileges.

| Method | Path                               |
| ------ | ---------------------------------- |
| POST   | `/api/v1/cluster/pki/join-tokens`  |

### Parameters

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
