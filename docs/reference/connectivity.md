---
description: Reference of the networking interfaces.
---

# Connectivity

Ella Core uses 4 different interfaces by default:

- **API**: The HTTP API and UI (HTTPS:5002)
- **N2 / S1-MME**: The control plane interface between Ella Core and the Radio (4G: `SCTP:36412`, 5G: `SCTP:38412`)
- **N3 / S1-U**: The user plane interface between Ella Core and the Radio (UDP:2152)
- **N6 / SGi**: The user plane interface between Ella Core and the internet

<figure markdown="span">
  ![Connectivity](../images/connectivity.svg){ width="800" }
  <figcaption>Connectivity in Ella Core</figcaption>
</figure>

# Combining interfaces

It is possible to combine interfaces in the following manners.

## Combined N2 and N3

Many radios can use a single network link towards the core. In this case,
N2 and N3 can be combined by using the same interface name for both of them in the
configuration file.

<figure markdown="span">
  ![Combined N2/N3](../images/combined_n2_n3.svg){ width="800" }
  <figcaption>Combined N2 and N3</figcaption>
</figure>

## Combined API and N6

The API interface is often the management interface with internet access,
and the N6 interface also requires internet access. They can be combined by using
the same interface name for both in the configuration file.

<figure markdown="span">
  ![Combined API/N6](../images/combined_api_n6.svg){ width="800" }
  <figcaption>Combined API and N6</figcaption>
</figure>

## Combined API/N6 and combined N2/N3

It is possible to use both combination together to reduce the requirements
to 2 interfaces.

<figure markdown="span">
  ![Combined All](../images/combined_all.svg){ width="800" }
  <figcaption>Combined All</figcaption>
</figure>

One or both of these interfaces can be virtual interfaces, with `veth`. See [Datapath constraints](#datapath-constraints).

## Combined on one interface

Ella Core can also be run with a single network interface. It can be achieved by
using the same interface name in the configuration file, or by using VLANs.

# Using VLANs

It is possible to use VLAN interfaces, with or without combining interfaces
as described previously. In this case, the configuration file should contain
the name of the VLAN interface, not the parent interface.

# Datapath constraints

The datapath attaches to N3 and N6 at the hook set by `datapath.attach-mode`. The shape of those interfaces constrains which modes work.

- **veth, `xdp-native`**: an XDP program must be attached to the peer interface, see the [explanation](../explanation/user_plane_packet_processing_with_ebpf.md#xdp-redirect-on-veth-pairs) and the [setup guide](../how_to/native_xdp_veth.md).
- **veth, `xdp-generic`**: TX checksum offload must be disabled on both ends, see the [explanation](../explanation/user_plane_packet_processing_with_ebpf.md#checksum-offload-on-veth-pairs).
- **Any interface, `tcx` or `xdp-generic`**: the interface must not deliver merged packets, see [Disable merged packets](../how_to/disable_merged_packets.md). 

# NAT

NAT is IPv4-only and applies to uplink traffic leaving N6, sourced from the N6
address. It is configured on the `Networking` page of the UI or through the
[Networking API](api/networking.md).

- Source ports are allocated from 1024-32767.
- Downlink traffic reaches a subscriber only when it matches a translation the
  subscriber's own traffic created. Anything else is dropped and counted in
  `app_upf_datapath_drop_total{reason="nat_unsolicited"}`.
- Traffic NAT cannot translate is dropped and counted in the same metric under
  another `nat_` reason: IP fragments, and protocols without ports such as ESP,
  GRE and SCTP. Protocols that embed addresses in their payload, such as FTP in
  active mode, do not work.

# IPv6 and dual-stack support

Ella Core supports IPv6 and dual-stack on the following interfaces:

- api
- n2
- n3

They can be configured specifically with an IPv6 address to use IPv6. When specifying an interface, Ella Core will use all the non link-local addresses on the interface; if the interface is configured for dual-stack, Ella Core will use dual-stack on that interface.

## Dual-stack N3 / S1-U transport

Both the N3 (5G) and S1-U (4G) interfaces support IPv4 and IPv6 transport for GTP-U tunnels. At startup Ella Core automatically resolves both IPv4 and IPv6 addresses from the configured interface and advertises them to the radio — to the gNB in the NGAP `TransportLayerAddress` (per 3GPP TS 38.414 Section 5.1), and to the eNB in the S1AP Transport Layer Address (per 3GPP TS 36.413) — each a 160-bit field carrying both families. The gNB selects its preferred address family in its response; for 4G, when the eNB offers both families, the IPv6 endpoint is used. Subsequent GTP-U encapsulation and decapsulation use the matching outer header type. IPv4-only and IPv6-only configurations are also supported.
