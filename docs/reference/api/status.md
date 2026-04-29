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

### Fields

Top‑level (always present):

- `version` (string): Software version.
- `revision` (string): Git commit hash.
- `initialized` (boolean): True once the system has at least one user.
- `ready` (boolean): True once the node has completed full startup.
- `schemaVersion` (integer): Shared‑database schema version this binary expects. Reported in both standalone and HA modes.

The nested `cluster` object is present only when HA is enabled:

- `enabled` (boolean): Always `true` inside this object.
- `role` (string): Raft role of this node — `Leader`, `Follower`, or `Candidate`.
- `nodeId` (integer): Raft node ID of this instance.
- `isLeader` (boolean): Convenience — `role == "Leader"`.
- `leaderNodeId` (integer): Raft node ID of the current leader; zero when unknown.
- `leaderAPIAddress` (string): HTTP API URL of the current leader. Omitted when the leader is unknown.
- `appliedIndex` (integer): Last Raft log index applied by this node.
- `clusterId` (string): Cluster ID from the operator configuration. Omitted when not set.

### Sample Response

```json
{
    "result": {
        "version": "v1.10.0",
        "revision": "388ce92244a0b304e9f6c15e3f896acee6fe7b1a",
        "initialized": true,
        "ready": true,
        "schemaVersion": 9
    }
}
```

When clustering is enabled, the response includes a `cluster` object:

```json
{
    "result": {
        "version": "v1.10.0",
        "revision": "388ce92244a0b304e9f6c15e3f896acee6fe7b1a",
        "initialized": true,
        "ready": true,
        "schemaVersion": 9,
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
