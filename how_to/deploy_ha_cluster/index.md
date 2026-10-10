# Deploy a High Availability Cluster

Ella Core can be deployed as a high-availability cluster to provide redundancy and failover capabilities. See [High Availability](https://docs.ellanetworks.com/explanation/high_availability/index.md) for more information.

## Prerequisites

- Three hosts meeting the standard [system requirements](https://docs.ellanetworks.com/reference/system_reqs/index.md).
- Ella Core installed on each host via the [Install](https://docs.ellanetworks.com/how_to/install/index.md) guide with `cluster.bind-address` set in the configuration file.

## 1. Start node 1

Start node 1, open a browser at the API address and initialize it normally.

## 2. Create a join token for node 2

Still in node 1, open the **Cluster** page and click **Add Node**, click **Mint Token**, then copy the token.

## 3. Start node 2

Start node 2, open a browser at the API address and click **Join an existing cluster instead**. Paste the token, enter node 1's cluster address, and click **Join**.

## 4. Add node 3

Repeat steps 2 and 3 for node 3.

## 5. Verify

On the **Cluster** page, all three nodes appear as **Healthy**.
