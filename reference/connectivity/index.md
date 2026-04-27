# Connectivity

Ella Core uses 4 different interfaces by default:

- **API**: The HTTP API and UI (HTTPS:5002)
- **N2**: The control plane interface between Ella Core and the 5G Radio (SCTP:38412)
- **N3**: The user plane interface between Ella Core and the 5G Radio (SCTP:2152)
- **N6**: The user plane interface between Ella Core and the internet

Connectivity in Ella Core

# Combining interfaces

It is possible to combine interfaces in the following manners.

## Combined N2 and N3

Many gNodeBs can use a single network link towards the core. In this case, N2 and N3 can be combined by using the same interface name for both of them in the configuration file.

Combined N2 and N3

## Combined API and N6

The API interface is often the management interface with internet access, and the N6 interface also requires internet access. They can be combined by using the same interface name for both in the configuration file.

Combined API and N6

## Combined API/N6 and combined N2/N3

It is possible to use both combination together to reduce the requirements to 2 interfaces.

Combined All

One or both of these interfaces can be virtual interfaces, with `veth`. When using veth with native XDP mode, an additional XDP program must be attached to the peer interface — see the [explanation](https://docs.ellanetworks.com/explanation/user_plane_packet_processing_with_ebpf/#xdp-redirect-on-veth-pairs) and the [setup guide](https://docs.ellanetworks.com/how_to/native_xdp_veth/index.md) for details.

## Combined on one interface

Ella Core can also be run with a single network interface. It can be achieved by using the same interface name in the configuration file, or by using VLANs.

# Using VLANs

It is possible to use VLAN interfaces, with or without combining interfaces as described previously. In this case, the configuration file should contain the name of the VLAN interface, not the parent interface.

# IPv6 and dual-stack support

Ella Core supports IPv6 and dual-stack on the following interfaces:

- api
- n2
- n3

They can be configured specifically with an IPv6 address to use IPv6. When specifying an interface, Ella Core will use all the non link-local addresses on the interface; if the interface is configured for dual-stack, Ella Core will use dual-stack on that interface.

## Dual-stack N3 transport

The N3 interface (between the UPF and the gNB) supports both IPv4 and IPv6 transport for GTP-U tunnels. At startup Ella Core automatically resolves both IPv4 and IPv6 addresses from the configured N3 interface and advertises them to the gNB via a 160-bit NGAP `TransportLayerAddress` (per 3GPP TS 38.414 Section 5.1). The gNB selects its preferred address family when it responds; subsequent GTP-U encapsulation and decapsulation use the matching outer header type. IPv4-only and IPv6-only N3 configurations are also supported.
