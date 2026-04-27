Ella Core includes an embedded BGP speaker that advertises `/32` host routes for each active subscriber IP. This guide walks through enabling BGP, adding a peer, and verifying the configuration.

For background on how Ella Core uses BGP, see the [BGP Route Advertisement](https://docs.ellanetworks.com/explanation/bgp/index.md) explanation.

NAT must be disabled

NAT and BGP are mutually exclusive. Disable NAT from the Networking > NAT tab before proceeding.

## Enable BGP

1. Open the Ella Core UI and navigate to **Networking > BGP**.
1. Edit the BGP settings:
   - **Local AS**: Your autonomous system number (e.g. `64512`).
   - **Router ID**: A unique IPv4 address identifying this BGP speaker, typically the N6 interface IP (e.g. `192.168.5.10`).
   - **Listen Address**: The address and port to listen on (default `:179`). Change this only if you need BGP on a non-standard port.
1. Click **Save**.
1. Toggle **BGP** to **ON**.

## Add a BGP peer

1. In the **Networking > BGP** tab, scroll to the **Peers** section.
1. Click **Create**.
1. Fill in the peer details:
   - **Address**: The IPv4 address of the upstream router (e.g. `192.168.5.1`).
   - **Remote AS**: The AS number of the peer (e.g. `64513`).
   - **Hold Time**: BGP hold timer in seconds (default `90`). The keepalive interval is derived as hold time / 3 per RFC 4271.
   - **Password** (optional): MD5 authentication password. Must match the peer's configuration.
   - **Description** (optional): A label for the peer.
1. Click **Create**.

## Verify advertised routes

View the advertised subscriber routes in the **Advertised Routes** table. The next-hop is always the N6 interface address.

## Configure the upstream router

Ella Core advertises routes but does not receive them. You still need to configure the upstream router to:

1. Peer with Ella Core (matching the AS number and address configured above).
1. Accept all routes for subscriber data networks (e.g. `192.168.0.0/24`)

Consult your router's documentation for BGP peering configuration.

Note

All steps in this guide can also be performed via the REST API. See the [BGP API reference](https://docs.ellanetworks.com/reference/api/networking/#bgp) for details.
