---
description: Add nodes to a running Ella Core high-availability cluster.
---

# Scale Up a High Availability Cluster

This guide walks through adding a node to an existing Ella Core high-availability cluster.

## Prerequisites

- A running cluster deployed via [Deploy a High Availability Cluster](deploy_ha_cluster.md).
- Admin credentials for the Ella Core UI, or an admin API token.
- A prepared host meeting the [system requirements](../reference/system_reqs.md), with Ella Core installed per the [Install](install.md) guide. Do **not** start the service yet.

## Add a node

1. On any existing node, open the Ella Core UI and navigate to the **Cluster** page. Note which node carries the **Leader** chip.
2. Open the **Cluster** page on the leader.
3. Click **Add Node**, click **Mint Token**, and copy the token.
4. On the new host, create `core.yaml` using the same shape as the other nodes:

    ```yaml title="core.yaml (new node)"
    cluster:
      bind-address: "10.0.0.4:7000"
    ```

5. Start Ella Core on the new host:

    ```shell
    sudo snap start --enable ella-core.cored
    ```

6. Open `https://10.0.0.4:5002` in a browser. On the **Join a cluster** page, paste the token, enter the cluster address of a node already in the cluster (for example `10.0.0.1:7000`), and click **Join**.

7. On the **Cluster** page, verify the new node appears and is shown as a **Voter** and **Healthy**.

## Verify the new cluster size

On the **Cluster** page, confirm:

- The expected number of voters is listed.
- Exactly one node is **Leader**.
- Every listed node is **Healthy**.
- **Failure tolerance** matches the expected value (`1` for 3 voters, `2` for 5 voters).
