---
description: Enable UE-to-UE communication with local switching
---

# Enable UE-to-UE communication

Local Switching allows two UEs served by Ella Core to communicate directly with each other.

## Enable local switching

1. Open the Ella Core UI and navigate to **Networking > Data Networks**, then open the data network of the UEs.
2. In **Slices**, toggle **Local switch** to **ON** on the slice of the UEs.

Local switching only connects UEs on the same slice and data network.

The change takes effect on the user plane immediately.

!!! note
    These steps can also be performed via the REST API. See the [Slices API reference](../reference/api/slices.md#update-a-data-network-of-a-slice) for details.
