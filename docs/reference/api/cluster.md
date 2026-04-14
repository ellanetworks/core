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
            "apiAddress": "10.0.0.1:5000"
        },
        {
            "nodeId": 2,
            "raftAddress": "10.0.0.2:7000",
            "apiAddress": "10.0.0.2:5000"
        }
    ]
}
```

## Add a Cluster Member

Adds a new node to the Raft cluster and registers it in the cluster members table. Requires admin privileges.

| Method | Path                       |
| ------ | -------------------------- |
| POST   | `/api/v1/cluster/members`  |

### Parameters

- `nodeId` (integer): Raft node ID for the new member. Must be a positive integer.
- `raftAddress` (string): The `host:port` used for Raft consensus communication.
- `apiAddress` (string): The `host:port` used for the REST API.

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
