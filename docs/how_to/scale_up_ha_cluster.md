---
description: Add nodes to a running Ella Core high-availability cluster.
---

# Scale Up a High Availability Cluster

This guide walks through adding a node to an existing Ella Core high-availability cluster.

## Prerequisites

- A running HA cluster deployed (see [Deploy a High Availability Cluster](deploy_ha_cluster.md)).
- A prepared host with Ella Core installed per the [Install](install.md) guide with `cluster.bind-address` set in the configuration file.

## Add a node

1. Open the **Cluster** page on any node in the cluster.
2. Click **Add Node**, click **Mint Token**, and copy the token.
3. Start Ella Core on the new host.
4. Open the new node's UI in a browser. Click on **Join an existing cluster instead**, paste the token, enter the cluster address of a node already in the cluster, and click **Join**.
5. On the **Cluster** page, verify the new node appears and is shown as **Healthy**.
