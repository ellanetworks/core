# Advertising and receiving routes via BGP

## What is BGP?

BGP is a protocol that allows exchanging routing information between autonomous systems.

## When is BGP needed?

Subscriber devices receive IPs from the data network pool. When NAT is not used, the external network needs to know how to route packets back to the subscriber through Ella Core. BGP might be needed in enterprise deployments where routable subscriber IPs are required.

## How does BGP work in Ella Core?

### Advertise subscriber routes

Ella Core embeds a BGP speaker that automatically advertises a `/32` host route for each active subscriber IP:

1. A subscriber establishes a PDU session and receives an IP (e.g. `10.45.0.3`).
1. Ella Core announces the route `10.45.0.3/32` to all configured BGP peers, with the N6 interface address as the next-hop.
1. Upstream routers install the route, and return traffic flows through the N6 interface to Ella Core, which delivers it to the subscriber over GTP-U.
1. When the PDU session is released, Ella Core withdraws the `/32` route.

This means routing state always reflects the set of currently connected subscribers with no manual intervention.

### Receive routes from BGP peers

Ella Core receives routes from BGP peers and installs them into the kernel routing table. This allows operators to manage routes (e.g., a default route via an upstream router) through BGP instead of static routes.

### In an HA cluster

Each node runs its own BGP speaker and advertises `/32` subnets for the sessions it currently hosts.See [High Availability](https://docs.ellanetworks.com/explanation/high_availability/index.md) for the broader cluster model.
