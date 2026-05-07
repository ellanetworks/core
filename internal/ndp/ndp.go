// Copyright 2026 Ella Networks
//
// Package ndp provides pure-Go helpers for IPv6 Neighbor Discovery Protocol
// (NDP) message parsing and construction, specifically Router Solicitations
// and Router Advertisements used for IPv6 prefix delegation in PDU sessions.
//
// This package has no dependency on SMF, UPF, veth, XDP, or any kernel
// interfaces. All functions operate on raw byte slices.
//
// Reference RFCs:
//   - RFC 4861: Neighbor Discovery for IP version 6 (IPv6)
//   - RFC 4443: ICMPv6
package ndp

// ICMPv6 type constants (RFC 4443 §2.1, RFC 4861 §4).
const (
	ICMPv6TypeRouterSolicitation  = 133
	ICMPv6TypeRouterAdvertisement = 134

	// ICMPv6 protocol number in the IPv6 Next Header field.
	ICMPv6ProtocolNumber = 58
)

// NDP option type constants (RFC 4861 §4.6).
const (
	NDPOptionSourceLinkLayerAddress = 1
	NDPOptionTargetLinkLayerAddress = 2
	NDPOptionPrefixInformation      = 3
	NDPOptionMTU                    = 5
)
