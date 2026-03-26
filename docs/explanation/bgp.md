# BGP Route Advertisement

## What is BGP?

BGP is a protocol that allows exchanging routing information between autonomous systems.

## When is BGP needed?

Subscriber devices receive IPs from the data network pool. When NAT is not used, the external network needs to know how to route packets back to the UE through Ella Core. BGP might be needed in enterprise deployments where routable subscriber IPs are required.

## How does BGP work in Ella Core?

Ella Core embeds a BGP speaker that automatically advertises a `/32` host route for each active subscriber IP:

1. A subscriber establishes a PDU session and receives an IP (e.g. `10.45.0.3`).
2. Ella Core announces the route `10.45.0.3/32` to all configured BGP peers, with the N6 interface address as the next-hop.
3. Upstream routers install the route, and return traffic flows through the N6 interface to the UPF, which delivers it to the subscriber over GTP-U.
4. When the PDU session is released, Ella Core withdraws the `/32` route.

This means routing state always reflects the set of currently connected subscribers with no manual intervention.

## Advertise-only

Ella Core's BGP speaker is advertise-only. It does **not** receive or install routes from BGP peers into the kernel routing table. If operators need return routes (e.g. a default route via an upstream router), they should configure them as static routes in Ella Core's Routes tab or directly on the host.
