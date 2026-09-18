---
description: How to run Ella Core interfaces inside VRFs
---

Any interface used by Ella Core (N2, N3, N6, API, cluster) can live inside a Linux VRF (Virtual Routing and Forwarding) domain. Ella Core has no VRF setting of its own: it discovers VRF membership from the host at startup and automatically binds its listeners and sockets to the right VRF device, programs routes and neighbours into the VRF's routing table, and scopes BGP sessions accordingly.

## Assign the interfaces to a VRF

Create a VRF domain on the host and assign each interface Ella Core uses to the VRF you want it in. You can do this with [netplan](https://netplan.readthedocs.io/en/stable/netplan-yaml/#vrfs), or following your distribution's documentation.

!!! note
    VRF creation and interface assignment must be done **before** starting Ella Core: VRF membership is discovered at startup, and interfaces moved into or out of a VRF while Ella Core is running will not be picked up.

!!! warning "N3 and N6 must share a VRF"
    The N3 and N6 interfaces must be assigned to the **same** VRF (or both left in the main routing table). Ella Core validates this at startup and refuses to start if they are in different VRFs.

!!! warning "Route fall-through"
    If the Linux kernel does not find a matching route in the routing table for a VRF, it will lookup in the main table. To prevent this, refer to the [vrf documentation](https://www.kernel.org/doc/html/latest/networking/vrf.html) for the Linux kernel.

## Configure Ella Core

Nothing VRF-specific goes in the configuration file: reference the interfaces by name or address as usual. Ella Core resolves each interface's VRF membership from the host.

```yaml
interfaces:
  n2:
    address: "10.1.0.2"
  n3:
    name: "n3"
  n6:
    name: "n6"
  api:
    address: "10.1.0.2"
    port: 5002
```

With `n3` and `n6` enslaved to a VRF named `up-vrf`, Ella Core will:

- Bind the GTP-U endpoint, End Marker sockets, and BGP speaker to `up-vrf`.
- Install subscriber routes, framed routes, and neighbour entries into the VRF's routing table.
- Enslave its internal veth pairs (`veth-smf`, `veth-xdp`, `veth-buf`, `veth-buf-xdp`) to `up-vrf` so downlink buffering and IPv6 router advertisements follow the user-plane routing domain.

## Put the cluster interface in a VRF

In a [high availability](deploy_ha_cluster.md) deployment, the interface carrying `cluster.bind-address` can be in its own VRF. Ella Core derives the VRF from that address and uses it for both directions: the cluster listener binds to it, and every outbound cluster connection. Peers must therefore be reachable from inside that VRF.

```yaml
cluster:
  enabled: true
  node-id: 1
  bind-address: "10.100.0.11:7000"
```

!!! warning "A wildcard bind address disables VRF binding"
    With `cluster.bind-address` set to a wildcard such as `0.0.0.0:7000`, Ella Core cannot determine the right VRF to bind to; therefore it disables VRF binding.

## Combining VRFs with VLANs

A VLAN interface does **not** inherit VRF membership from its parent device: assigning `n3` to a VRF does not place `n3.100` in it. When the interface Ella Core uses is a VLAN interface, assign the VLAN device itself to the VRF — in netplan, list the VLAN under the VRF's `interfaces`, not the physical parent. If only the physical parent is enslaved, the VLAN interface stays in the main routing table and VRF isolation will not apply to it.

See [Use VLANs](vlan.md) for referencing VLAN interfaces in the configuration file.

## Verify

Confirm that each interface Ella Core uses is enslaved to the intended VRF, and that subscriber routes and neighbour entries appear in the VRF's routing table rather than the main table. Routes installed by Ella Core carry protocol `236`.

!!! note
    When BGP is enabled, the listen address scopes which VRF the speaker binds to. See [Advertise subscriber routes with BGP](bgp.md) for details.
