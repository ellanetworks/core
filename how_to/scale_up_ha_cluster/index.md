# Scale Up a High Availability Cluster

This guide walks through adding a node to an existing Ella Core high-availability cluster.

## Prerequisites

- A running HA cluster deployed (see [Deploy a High Availability Cluster](https://docs.ellanetworks.com/how_to/deploy_ha_cluster/index.md)).
- A prepared host with Ella Core installed per the [Install](https://docs.ellanetworks.com/how_to/install/index.md) guide with `cluster.bind-address` set in the configuration file.

## Add a node

1. Open the **Cluster** page on any node in the cluster.
1. Click **Add Node**, click **Mint Token**, and copy the token.
1. Start Ella Core on the new host.
1. Open the new node's UI in a browser. Click on **Join an existing cluster instead**, paste the token, enter the cluster address of a node already in the cluster, and click **Join**.
1. On the **Cluster** page, verify the new node appears and is shown as **Healthy**.
