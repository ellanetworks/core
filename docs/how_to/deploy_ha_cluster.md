---
description: Step-by-step instructions to deploy Ella Core as a high-availability cluster.
---

# Deploy a High Availability Cluster (beta)

See [High Availability](../explanation/high_availability.md) for how the cluster works.

!!! info "Beta feature"
    High availability is currently in beta. It is available for testing and feedback in the `main` branch but not recommended for production use yet. Expect breaking changes as we iterate on the design and implementation.

## Prerequisites

- Three hosts meeting the standard [system requirements](../reference/system_reqs.md).
- Ella Core installed on each host via the [Install](install.md) guide. Do **not** start the service yet.
- A reachable TCP port on each host for inter-node traffic (this guide uses `7000`).

## 1. Configure node 1

Put this in `core.yaml` on node 1. Adjust interface names, addresses, and ports to match the host.

```yaml title="core.yaml (node 1)"
logging:
  system:
    level: "info"
    output: "stdout"
  audit:
    output: "stdout"
db:
  path: "/var/snap/ella-core/common/ella.db"
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
  peers:
    - "10.0.0.1:7000"
    - "10.0.0.2:7000"
    - "10.0.0.3:7000"
```

## 2. Start node 1

```shell
sudo snap start --enable ella-core.cored
```

## 3. Create the admin user

Open `https://10.0.0.1:5002` in a browser, create the admin, and log in.

## 4. Add node 2

On node 1, open the **Cluster** page and click **Mint Join Token** with `nodeID: 2`. Copy the returned token string.

Configure node 2 with its own `core.yaml`, using the same shape as node 1, with `node-id: 2` and `bind-address: "10.0.0.2:7000"`. Add `join-token` under `cluster`:

```yaml title="core.yaml (node 2, cluster block)"
cluster:
  enabled: true
  node-id: 2
  bind-address: "10.0.0.2:7000"
  peers:
    - "10.0.0.1:7000"
    - "10.0.0.2:7000"
    - "10.0.0.3:7000"
  join-token: "ejYM..."
```

Start node 2:

```shell
sudo snap start --enable ella-core.cored
```

The daemon consumes the token on first boot, joins the cluster, and ignores the field on subsequent starts.

## 5. Add node 3

Repeat step 4 with `node-id: 3` and a freshly-minted token for that node-id.

## 6. Verify

On the **Cluster** page, all three nodes appear as **Voter**, one as
**Leader**, all **Healthy**.

<figure markdown="span">
  ![Ella Core HA Cluster](../images/ha_cluster.png){ width="800" }
</figure>
