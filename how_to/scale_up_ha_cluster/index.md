# Scale Up a High Availability Cluster

This guide walks through adding a node to an existing Ella Core high-availability cluster. For background on quorum, voter counts, and failover, see [High Availability](https://docs.ellanetworks.com/explanation/high_availability/index.md). To bring up the initial cluster, see [Deploy a High Availability Cluster](https://docs.ellanetworks.com/how_to/deploy_ha_cluster/index.md).

## Prerequisites

- A running cluster deployed via [Deploy a High Availability Cluster](https://docs.ellanetworks.com/how_to/deploy_ha_cluster/index.md).
- Admin credentials for the Ella Core UI, or an admin API token.
- A prepared host meeting the [system requirements](https://docs.ellanetworks.com/reference/system_reqs/index.md), with Ella Core installed per the [Install](https://docs.ellanetworks.com/how_to/install/index.md) guide. Do **not** start the service yet.

## Add a node

1. On any existing node, open the Ella Core UI and navigate to the **Cluster** page.

1. Click **Add Node**, select the next free node ID (for example `4`), click **Mint Token**, and copy the token.

1. On the new host, create `core.yaml` using the same shape as the other nodes. List every node — including the new one — in `peers`, and paste the token into `join-token`:

   core.yaml (new node)

   ```
   cluster:
     enabled: true
     node-id: 4
     bind-address: "10.0.0.4:7000"
     peers:
       - "10.0.0.1:7000"
       - "10.0.0.2:7000"
       - "10.0.0.3:7000"
       - "10.0.0.4:7000"
     join-token: "ejYM..."
   ```

1. Start Ella Core on the new host:

   ```
   sudo snap start --enable ella-core.cored
   ```

1. On the **Cluster** page, verify the new node appears and is shown as **Healthy**. Autopilot promotes it to voter automatically after a short stabilization window.

## Verify the new cluster size

On the **Cluster** page, confirm:

- The expected number of voters is listed.
- Exactly one node is **Leader**.
- Every listed node is **Healthy**.
- **Failure tolerance** matches the expected value (`1` for 3 voters, `2` for 5 voters).

## Keep peer configs in sync

On every existing node, add the new node's `host:port` to `cluster.peers` in `core.yaml`. The change takes effect at the next restart; no immediate restart is required.

Note

All steps in this guide can also be performed via the REST API. See the [Cluster API reference](https://docs.ellanetworks.com/reference/api/cluster/index.md) for details.
