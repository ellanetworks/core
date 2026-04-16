---
description: RESTful API reference for managing cluster membership.
---

# Cluster

These endpoints are only available when clustering is enabled in the configuration file.

## List Cluster Members

Returns all registered cluster members.

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
            "suffrage": "voter"
        },
        {
            "nodeId": 2,
            "raftAddress": "10.0.0.2:7000",
            "apiAddress": "https://10.0.0.2:5000",
            "binaryVersion": "v1.9.1",
            "suffrage": "voter"
        }
    ]
}
```

## Add a Cluster Member

Adds a new node to the Raft cluster and registers it in the cluster members table. This endpoint accepts either admin credentials or the cluster join token for authentication.

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

## Drain Node

Gracefully prepares this node for removal. Signals RANs to redirect new UE registrations elsewhere, withdraws BGP advertisements so upstream routers reroute user‑plane traffic, and transfers Raft leadership if this node is the leader. The node continues serving existing flows until it is shut down. Requires admin privileges.

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
