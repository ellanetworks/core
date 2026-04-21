---
description: Step-by-step instructions to deploy Ella Core as a high-availability cluster.
---

# Deploy a High Availability Cluster (beta)

This guide walks through bringing up a three-node Ella Core cluster from scratch. For background on how clustering works, see the [High Availability](../explanation/high_availability.md) explanation.

!!! info "Beta feature"
    High availability is currently in beta. It is available for testing and feedback in the `main` branch but not recommended for production use yet. Expect breaking changes as we iterate on the design and implementation.

## Pre-requisites

- Three (or five) hosts that each meet the standard [system requirements](../reference/system_reqs.md).
- Ella Core installed on each host following the [Install](install.md) guide. Do **not** start the service yet.
- A reachable TCP port on each host for inter-node traffic (this guide uses `7000`).

## 1. Generate the cluster PKI

Every node authenticates to its peers with a leaf certificate signed by a shared cluster CA. The leaf's Common Name must be `ella-node-<node-id>`, where `node-id` matches `cluster.node-id` in the node's config. `node-id` must be between 1 and 63.

Using OpenSSL, create a CA and one leaf per node:

```shell
# Cluster CA
openssl ecparam -name prime256v1 -genkey -noout -out cluster-ca-key.pem
openssl req -x509 -new -key cluster-ca-key.pem -days 3650 \
    -subj "/CN=ella-cluster-ca" -out cluster-ca.pem

# Per-node leaf (repeat for node-id 1, 2, 3)
NODE_ID=1
openssl ecparam -name prime256v1 -genkey -noout -out node-${NODE_ID}-key.pem
openssl req -new -key node-${NODE_ID}-key.pem \
    -subj "/CN=ella-node-${NODE_ID}" -out node-${NODE_ID}.csr
openssl x509 -req -in node-${NODE_ID}.csr \
    -CA cluster-ca.pem -CAkey cluster-ca-key.pem -CAcreateserial \
    -days 365 -out node-${NODE_ID}-cert.pem \
    -extfile <(printf "extendedKeyUsage=serverAuth,clientAuth")
```

Copy `cluster-ca.pem`, the node's own `node-N-cert.pem`, and `node-N-key.pem` to each host (for example, under `/etc/ella/`). Never distribute the CA private key or a leaf key meant for a different node.

## 2. Configure each node

On every node, add a `cluster` block to the configuration file. The example below is for node 1; change `node-id`, `bind-address`, and the TLS paths on each of the other two nodes. The `peers` list and `bootstrap-expect` must be identical on all three.

```yaml title="core.yaml"
interfaces:
  n2:
    address: "10.0.0.1"
    port: 38412
  n3:
    name: "n3"
  n6:
    name: "eth0"
  api:
    address: "10.0.0.1"
    port: 5002
xdp:
  attach-mode: "native"
cluster:
  enabled: true
  node-id: 1
  bind-address: "10.0.0.1:7000"
  bootstrap-expect: 3
  peers:
    - "10.0.0.1:7000"
    - "10.0.0.2:7000"
    - "10.0.0.3:7000"
  tls:
    ca: "/etc/ella/cluster-ca.pem"
    cert: "/etc/ella/node-1-cert.pem"
    key: "/etc/ella/node-1-key.pem"
```

Key points validated by the config loader:

- `peers` entries are `host:port` (not URLs) and must include this node's own `bind-address` (or `advertise-address` if set).
- The leaf certificate's CN must match `ella-node-<node-id>`.
- `bootstrap-expect` must be ≤ `len(peers)`.

See the [Configuration File](../reference/config_file.md#clustering) reference for every available field.

## 3. Start the nodes

Start the Ella Core daemon on all three hosts. Order does not matter — each node probes its peers and waits until `bootstrap-expect` peers are reachable. The node with the lowest `node-id` then bootstraps Raft; the other two automatically join as voters.

=== "Snap"

    ```shell
    sudo snap start --enable ella-core.cored
    ```

=== "Source"

    ```shell
    sudo ./main -config core.yaml
    ```

## 4. Initialize the cluster

Open any node's UI in a browser (for example `https://10.0.0.1:5002`) and create the first admin user when prompted. The write is replicated through Raft to the other members, so you only do this once.

Log in. Writes issued to a follower are transparently forwarded to the leader.

## 5. Verify the cluster

Navigate to the **Cluster** page. All three nodes should appear with suffrage **Voter** and one should be flagged as **Leader**. The page combines the static membership view with live autopilot data — `failureTolerance` should equal `1` for a three-node cluster, and every node should report **Alive** and **Healthy**.

## Add a node

To grow the cluster (for example, from three to five), configure the new node as described in steps 1–3, but set `initial-suffrage: nonvoter` so it catches up without affecting quorum, and include its address in every other node's `peers` list on the next restart.

In the UI:

1. Open the **Cluster** page and click **Add Member**.
2. Fill in the new node's details:
     - **Node ID**: The `cluster.node-id` configured on the new node (1–63).
     - **Cluster Address**: The new node's `bind-address` (e.g. `10.0.0.4:7000`).
     - **API Address**: The new node's API URL (e.g. `https://10.0.0.4:5002`).
     - **Suffrage**: Leave on **Non-voter**.
3. Click **Add**.

Autopilot promotes the non-voter to a full voter automatically once it has been healthy and up-to-date for ten seconds. To promote immediately, click the promote action in the row. See [Scaling the cluster](../explanation/high_availability.md#scaling-the-cluster).

## Remove a node

A node must be drained before it can be removed. See [Draining a node](../explanation/high_availability.md#draining-a-node) for what drain does.

On the **Cluster** page:

1. Click the drain action on the target row and confirm. Wait for the row's drain state to reach **Drained**.
2. Click the delete action on the same row and confirm.

If the target node is already unreachable and cannot be drained, the delete action offers a force option.

!!! note
    Every action in this guide can also be performed via the REST API. See the [Cluster API reference](../reference/api/cluster.md) for the full surface.
