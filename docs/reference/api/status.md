---
description: RESTful API reference for getting the system status.
---

# Status

## Get the status

This path returns the status of Ella core.

| Method | Path             |
| ------ | ---------------- |
| GET    | `/api/v1/status` |

### Parameters

None

### Response Headers

When clustering is enabled, the response includes an `X-Ella-Role` header with the Raft role of the responding node (`Leader`, `Follower`, `Candidate`, `Shutdown`, or `Unknown`). Load balancers can use this header to direct write traffic to the leader.

### Sample Response

```json
{
    "result": {
        "version": "v1.21.0",
        "revision": "388ce92244a0b304e9f6c15e3f896acee6fe7b1a",
        "initialized": true,
        "ready": true,
        "schemaVersion": 9,
        "datapathAttachMode": "xdp-native",
        "cluster": {
            "enabled": false,
            "nodeId": "0199c4f1-2a7e-7b31-9c5d-1f2e3a4b5c6d"
        }
    }
}
```

When clustering is enabled, the response includes a `cluster` object:

```json
{
    "result": {
        "version": "v1.21.0",
        "revision": "388ce92244a0b304e9f6c15e3f896acee6fe7b1a",
        "initialized": true,
        "ready": true,
        "schemaVersion": 9,
        "datapathAttachMode": "xdp-native",
        "cluster": {
            "enabled": true,
            "role": "Leader",
            "nodeId": 1,
            "isLeader": true,
            "leaderNodeId": 1,
            "appliedIndex": 42,
            "clusterId": "my-cluster",
            "leaderAPIAddress": "https://10.0.0.1:5002"
        }
    }
}
```

## Check Liveness

This path answers while the process serves HTTP. It takes no token.

| Method | Path            |
| ------ | --------------- |
| GET    | `/api/v1/livez` |

It answers `204 No Content`.

## Check Readiness

This path returns whether the node is ready to receive traffic, and the result of each check. It takes no token.

| Method | Path             |
| ------ | ---------------- |
| GET    | `/api/v1/readyz` |

### Query Parameters

| Name      | Type    | Description |
| --------- | ------- | ----------- |
| `cluster` | boolean | Add the `cluster` check. Default `false`. |

### Checks

| Name          | Passes when |
| ------------- | ----------- |
| `shutdown`    | The node is not stopping. |
| `member`      | The node finished starting and, in a cluster, joined it. |
| `initialized` | Clustering is disabled, or a user exists. |
| `drain`       | Clustering is disabled, or the node is not draining or drained. |
| `database`    | The database answers. |
| `leader`      | The node leads, or heard from the leader within twice the election timeout. |
| `cluster`     | Clustering is disabled, or the node leads and the cluster is healthy. Only with `cluster=true`. |

### Response Fields

| Field             | Type    | Description |
| ----------------- | ------- | ----------- |
| `ready`           | boolean | Whether every check passes. |
| `checks`          | array   | The checks, in order. |
| `checks[].name`   | string  | Name of the check. |
| `checks[].ok`     | boolean | Whether it passes. |
| `checks[].detail` | string  | Why it fails. Empty when it passes. |

### Response Codes

| Code  | Description |
| ----- | ----------- |
| `200` | Every check passes. |
| `400` | `cluster` is not `true` or `false`. |
| `503` | A check fails. |

### Sample Response

```json
{
    "result": {
        "ready": false,
        "checks": [
            {"name": "shutdown", "ok": true, "detail": ""},
            {"name": "member", "ok": true, "detail": ""},
            {"name": "initialized", "ok": true, "detail": ""},
            {"name": "drain", "ok": true, "detail": ""},
            {"name": "database", "ok": true, "detail": ""},
            {"name": "leader", "ok": false, "detail": "no contact with the cluster leader for 12s"}
        ]
    }
}
```
