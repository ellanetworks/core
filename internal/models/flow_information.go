package models

type FlowInformation struct {
	// Defines a packet filter for an IP flow.Refer to subclause 5.4.2 of 3GPP TS 29.212 [23] for encoding.
	FlowDescription string
	// An identifier of packet filter.
	PackFiltID string
	// The packet shall be sent to the UE.
	PacketFilterUsage bool
	// Contains the Ipv4 Type-of-Service and mask field or the Ipv6 Traffic-Class field and mask field.
	TosTrafficClass string
	// the security parameter index of the IPSec packet.
	Spi string
	// the Ipv6 flow label header field.
	FlowLabel     string
	FlowDirection FlowDirectionRm
}
