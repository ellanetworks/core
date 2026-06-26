# Deploy a High Availability Cluster

Ella Core can be deployed as a high-availability cluster to provide redundancy and failover capabilities. See [High Availability](https://docs.ellanetworks.com/explanation/high_availability/index.md) for more information.

## Prerequisites

- Three hosts meeting the standard [system requirements](https://docs.ellanetworks.com/reference/system_reqs/index.md).
- Ella Core installed on each host via the [Install](https://docs.ellanetworks.com/how_to/install/index.md) guide. Do **not** start the service yet.
- A reachable TCP port on each host for inter-node traffic (this guide uses `7000`).

## 1. Configure node 1

Put this in `core.yaml` on node 1. Adjust interface names, addresses, and ports to match the host.

core.yaml (node 1)

```
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

```
sudo snap start --enable ella-core.cored
```

## 3. Create the admin user

Open `https://10.0.0.1:5002` in a browser, create the admin, and log in.

## 4. Add node 2

On node 1, open the **Cluster** page and click **Add Node**. Select node ID `2`, click **Mint Token**, then copy the token.

Create `core.yaml` on node 2 using the same shape as node 1, with `bind-address: "10.0.0.2:7000"`. Paste the copied token block over the placeholder:

core.yaml (node 2, cluster block)

```
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

```
sudo snap start --enable ella-core.cored
```

## 5. Add node 3

Repeat step 4 on node 3.

## 6. Verify

On the **Cluster** page, all three nodes appear as **Voter**, one as **Leader**, all **Healthy**.
