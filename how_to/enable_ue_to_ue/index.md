# Enable UE-to-UE communication

By default, traffic from one UE destined for another UE on the same UPF is routed out over N6 and back, which requires the upstream network to route between subscriber subnets and typically fails when NAT is enabled. Local switching forwards such traffic directly inside the user plane, so two UEs served by the same Ella Core instance can reach each other without traversing N6.

Local switching is disabled by default for improved security.

Warning

Enabling local switching lets subscribers initiate connections to each other. Review your [policies](https://docs.ellanetworks.com/reference/api/policies/index.md) and ensure they only permit the traffic your deployment intends to allow between UEs.

## Enable local switching

1. Open the Ella Core UI and navigate to **Networking > Local Switch**.
1. Toggle **Local switch** to **ON**.

The change takes effect on the user plane immediately.

Note

These steps can also be performed via the REST API. See the [Local Switch API reference](https://docs.ellanetworks.com/reference/api/networking/#local-switch) for details.
