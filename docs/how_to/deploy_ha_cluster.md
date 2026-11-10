---
description: Step-by-step instructions to deploy Ella Core as a high-availability cluster.
---

# Deploy a High Availability Cluster

Ella Core can be deployed as a high-availability cluster to provide redundancy and failover capabilities. See [High Availability](../explanation/high_availability.md) for more information.

=== "From the UI"

    ## Prerequisites

    - Three hosts meeting the standard [system requirements](../reference/system_reqs.md).
    - Ella Core installed on each host via the [Install](install.md) guide with `cluster.bind-address` set in the configuration file.

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

=== "From the configuration file"

    ## Prerequisites

    - Three hosts meeting the standard [system requirements](../reference/system_reqs.md).
    - Ella Core installed on each host via the [Install](install.md) guide.

    ## 1. Create the cluster secret

    On one host, create the secret:

    ```shell
    openssl rand -base64 32 > cluster.secret
    ```

    Copy `cluster.secret` to every host, for example to `/var/snap/ella-core/common/cluster.secret`. Keep it readable by Ella Core only.

    ## 2. Configure every node

    Add the same `cluster` block to the configuration file of every node, listing the three nodes' addresses:

    ```yaml
    cluster:
      bind-address: ":7000"
      join: ["10.0.0.1", "10.0.0.2", "10.0.0.3"]
      bootstrap-expect: 3
      secret-file: "/var/snap/ella-core/common/cluster.secret"
    ```

    ## 3. Start every node

    Start Ella Core on the three nodes.

    ## 4. Initialize

    Open a browser at any node's API address and initialize it normally.

    ## 5. Verify

    On the **Cluster** page, all three nodes appear as **Healthy**.

<figure markdown="span">
  ![Ella Core HA Cluster](../images/ha_cluster.png){ width="800" }
</figure>
