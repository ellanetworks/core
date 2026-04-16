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

When clustering is enabled, the response includes an `X-Ella-Role` header with the Raft role of the responding node (`Leader`, `Follower`, or `Candidate`). Load balancers can use this header to direct write traffic to the leader.

### Sample Response

```json
{
    "result": {
        "version": "v1.9.1",
        "revision": "388ce92244a0b304e9f6c15e3f896acee6fe7b1a",
        "initialized": true,
        "ready": true
    }
}
```

When clustering is enabled, the response includes a `cluster` object:

```json
{
    "result": {
        "version": "v1.9.1",
        "revision": "388ce92244a0b304e9f6c15e3f896acee6fe7b1a",
        "initialized": true,
        "ready": true,
        "cluster": {
            "enabled": true,
            "role": "Leader",
            "nodeId": 1,
            "appliedIndex": 42,
            "clusterId": "my-cluster",
            "schemaVersion": 9,
            "leaderAddress": "https://10.0.0.1:5002"
        }
    }
}
```
