# Deploy a High Availability Cluster

Ella Core can be deployed as a high-availability cluster to provide redundancy and failover capabilities. See [High Availability](https://docs.ellanetworks.com/core/explanation/high_availability/index.md) for more information.

## Prerequisites

- Three hosts meeting the standard [system requirements](https://docs.ellanetworks.com/core/reference/system_reqs/index.md).

- Ella Core installed on each host via the [Install](https://docs.ellanetworks.com/core/how_to/install/index.md) guide with `cluster.bind-address` set in the configuration file.

- A Kubernetes cluster meeting the [Kubernetes requirements](https://docs.ellanetworks.com/core/reference/system_reqs/#kubernetes).

## 1. Deploy the nodes

Start Ella Core on each host.

Create the namespace:

```
kubectl create namespace ella
kubectl label namespace ella pod-security.kubernetes.io/enforce=privileged pod-security.kubernetes.io/warn=privileged
```

Create `values.yaml`. Set `master` to the host interface that connects to your radios, and each node's `ranAddress` to its N2/N3 address:

```
nodes:
  - ranAddress: 192.168.252.3/24
  - ranAddress: 192.168.252.4/24
  - ranAddress: 192.168.252.5/24
ran:
  networkAttachment:
    master: eth1
```

Deploy Ella Core:

```
helm install ella-core oci://ghcr.io/ellanetworks/charts/ella-core --version v1.22.0 --namespace ella -f values.yaml
```

Open each node's API in a separate terminal:

```
kubectl -n ella port-forward pod/ella-core-1-0 5002:5002
kubectl -n ella port-forward pod/ella-core-2-0 5003:5002
kubectl -n ella port-forward pod/ella-core-3-0 5004:5002
```

The API addresses of nodes 1, 2 and 3 are `http://localhost:5002`, `http://localhost:5003` and `http://localhost:5004`.

## 2. Initialize node 1

Open a browser at node 1's API address and initialize it normally.

## 3. Create a join token for node 2

Still in node 1, open the **Cluster** page and click **Add Node**, click **Mint Token**, then copy the token.

## 4. Join node 2

Open a browser at node 2's API address and click **Join an existing cluster instead**. Paste the token, enter node 1's cluster address, and click **Join**.

Node 1's cluster address is its `cluster.bind-address`.

Node 1's cluster address is `ella-core-1-0.ella-core-headless.ella.svc.cluster.local:7000`.

## 5. Add node 3

Repeat steps 3 and 4 for node 3.

## 6. Verify

On the **Cluster** page, all three nodes appear as **Healthy**.
