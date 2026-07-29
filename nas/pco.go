// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"
	"net/netip"
)

// Protocol/container identifiers for the (extended) Protocol Configuration
// Options (TS 24.008 §10.5.6.3), shared by EPS (TS 24.301) and 5GS (TS 24.501).
// A request (uplink) and its answer (downlink) share the container identifier.
const (
	PCOContainerDNSServerIPv6Address      uint16 = 0x0003
	PCOContainerIPAddressAllocationViaNAS uint16 = 0x000A
	PCOContainerDNSServerIPv4Address      uint16 = 0x000D
	PCOContainerIPv4LinkMTU               uint16 = 0x0010
)

// PCOConfigProtocolPPP is the configuration-protocol octet for use with the IP
// PDP/PDN context: extension bit set, protocol PPP (TS 24.008 §10.5.6.3).
const PCOConfigProtocolPPP uint8 = 0x80

// PCODirection is the direction a Protocol Configuration Options element
// travels. It has to be known to read one: TS 24.008 figure 10.5.136 gives a
// handful of container identifiers a two-octet length instead of a one-octet
// one, and the set differs by direction. Framing an element with the wrong
// direction desynchronizes every container after the first such identifier.
type PCODirection uint8

const (
	// PCODirectionUnset is the zero value. It names no direction, so an element
	// built from a literal that omits Direction fails to encode rather than
	// silently taking the downlink framing.
	PCODirectionUnset PCODirection = iota
	// PCONetworkToMS is the downlink direction, which has the wider two-octet set.
	PCONetworkToMS
	// PCOMSToNetwork is the uplink direction.
	PCOMSToNetwork
)

// twoOctetLength reports whether a container identifier carries a two-octet
// length in this direction (TS 24.008 figure 10.5.136).
func (d PCODirection) twoOctetLength(id uint16) bool {
	switch id {
	case 0x0041, // service-level-AA container
		0x0051, // SDNAEPC EAP message
		0x0056: // UE policy container
		return true
	case 0x0023, // QoS rules
		0x0024, // QoS flow descriptions
		0x0030, // ATSSS response
		0x0031, // DNS server security information
		0x0032: // ECS address
		return d == PCONetworkToMS
	default:
		return false
	}
}

// PCOContainer is one protocol or container of a Protocol Configuration Options
// information element: its identifier and its content, uninterpreted
// (TS 24.008 §10.5.6.3).
type PCOContainer struct {
	ID      uint16
	Content []byte
}

// ProtocolConfigurationOptions is the (extended) Protocol Configuration Options
// information element value — the configuration-protocol octet and the
// containers that follow it, in wire order (TS 24.008 §10.5.6.3). Containers the
// library does not interpret are carried through unchanged, so the element
// round-trips.
type ProtocolConfigurationOptions struct {
	ConfigProtocol uint8
	Containers     []PCOContainer

	// Direction is the direction the element travels, which decides how each
	// container's length is framed. Encoding uses the same rule decoding did, so
	// an element re-encodes exactly as it arrived.
	Direction PCODirection
}

// NewProtocolConfigurationOptions builds the network-to-UE (downlink) options:
// one DNS-server container per address — IPv4 (4 octets) under 0x000D, IPv6
// (16 octets) under 0x0003 — and, when ipv4LinkMTU is non-zero, the IPv4 Link
// MTU (container 0x0010, a 2-octet MTU in octets). IPv6 has no PCO MTU
// container; the IPv6 link MTU is delivered via the Router Advertisement
// (TS 24.501). An address that is neither 4 nor 16 octets is skipped.
func NewProtocolConfigurationOptions(dnsServers [][]byte, ipv4LinkMTU uint16) ProtocolConfigurationOptions {
	p := ProtocolConfigurationOptions{ConfigProtocol: PCOConfigProtocolPPP, Direction: PCONetworkToMS}

	for _, addr := range dnsServers {
		switch len(addr) {
		case 4:
			p.Containers = append(p.Containers, PCOContainer{ID: PCOContainerDNSServerIPv4Address, Content: addr})
		case 16:
			p.Containers = append(p.Containers, PCOContainer{ID: PCOContainerDNSServerIPv6Address, Content: addr})
		}
	}

	if ipv4LinkMTU != 0 {
		p.Containers = append(p.Containers, PCOContainer{
			ID:      PCOContainerIPv4LinkMTU,
			Content: []byte{uint8(ipv4LinkMTU >> 8), uint8(ipv4LinkMTU)},
		})
	}

	return p
}

// NewRequestedProtocolConfigurationOptions builds the UE-to-network (uplink)
// options: one empty-content container per requested identifier.
func NewRequestedProtocolConfigurationOptions(containerIDs ...uint16) ProtocolConfigurationOptions {
	p := ProtocolConfigurationOptions{ConfigProtocol: PCOConfigProtocolPPP, Direction: PCOMSToNetwork}

	for _, id := range containerIDs {
		p.Containers = append(p.Containers, PCOContainer{ID: id})
	}

	return p
}

// maxPCOLen is the element's longest value: TS 24.008 §10.5.6.3 caps the whole
// element at 253 octets, two of which are its IEI and length.
const maxPCOLen = 251

// ParseProtocolConfigurationOptions decodes a Protocol Configuration Options
// value: the configuration-protocol octet followed by identifier / length /
// content containers (TS 24.008 §10.5.6.3).
func ParseProtocolConfigurationOptions(b []byte, dir PCODirection) (ProtocolConfigurationOptions, error) {
	if dir == PCODirectionUnset {
		return ProtocolConfigurationOptions{}, fmt.Errorf("nas: protocol configuration options: no direction given")
	}

	if len(b) > maxPCOLen {
		return ProtocolConfigurationOptions{}, fmt.Errorf(
			"nas: protocol configuration options is %d octets, want at most %d", len(b), maxPCOLen)
	}

	r := NewReader(b)

	proto, err := r.U8()
	if err != nil {
		return ProtocolConfigurationOptions{}, err
	}

	p := ProtocolConfigurationOptions{ConfigProtocol: proto, Direction: dir}

	for r.Remaining() > 0 {
		id, err := r.U16()
		if err != nil {
			return ProtocolConfigurationOptions{}, err
		}

		read := r.LV
		if dir.twoOctetLength(id) {
			read = r.LVE
		}

		content, err := read()
		if err != nil {
			return ProtocolConfigurationOptions{}, err
		}

		p.Containers = append(p.Containers, PCOContainer{ID: id, Content: content})
	}

	return p, nil
}

// AppendBinary encodes the Protocol Configuration Options value onto b.
func (p ProtocolConfigurationOptions) AppendBinary(b []byte) ([]byte, error) {
	if p.Direction == PCODirectionUnset {
		return b, fmt.Errorf("nas: protocol configuration options: Direction is unset, so its containers cannot be framed")
	}

	w := NewWriter(b)

	w.U8(p.ConfigProtocol)

	for _, c := range p.Containers {
		w.U16(c.ID)

		if p.Direction.twoOctetLength(c.ID) {
			w.LVE(c.Content)
			continue
		}

		w.LV(c.Content)
	}

	out, err := w.Result(b)
	if err != nil {
		return b, err
	}

	if len(out)-len(b) > maxPCOLen {
		return b, fmt.Errorf(
			"nas: protocol configuration options is %d octets, want at most %d", len(out)-len(b), maxPCOLen)
	}

	return out, nil
}

// MarshalBinary encodes the Protocol Configuration Options value.
func (p ProtocolConfigurationOptions) MarshalBinary() ([]byte, error) { return p.AppendBinary(nil) }

// Empty reports whether there is nothing to convey, in which case the sender
// omits the information element.
func (p ProtocolConfigurationOptions) Empty() bool { return len(p.Containers) == 0 }

// ContainerIDs returns the container identifiers carried, in wire order. It
// inspects which containers a UE requested; the contents are not returned.
func (p ProtocolConfigurationOptions) ContainerIDs() []uint16 {
	if len(p.Containers) == 0 {
		return nil
	}

	ids := make([]uint16, 0, len(p.Containers))
	for _, c := range p.Containers {
		ids = append(ids, c.ID)
	}

	return ids
}

// DNSServers returns the DNS-server addresses carried, IPv4 and IPv6 together,
// in wire order. A container whose content is not a 4- or 16-octet address is
// skipped.
func (p ProtocolConfigurationOptions) DNSServers() []netip.Addr {
	var out []netip.Addr

	for _, c := range p.Containers {
		switch c.ID {
		case PCOContainerDNSServerIPv4Address, PCOContainerDNSServerIPv6Address:
			if addr, ok := netip.AddrFromSlice(c.Content); ok {
				out = append(out, addr)
			}
		}
	}

	return out
}

// IPv4LinkMTU returns the IPv4 Link MTU in octets and whether a well-formed
// container carried it.
func (p ProtocolConfigurationOptions) IPv4LinkMTU() (uint16, bool) {
	for _, c := range p.Containers {
		if c.ID == PCOContainerIPv4LinkMTU && len(c.Content) == 2 {
			return uint16(c.Content[0])<<8 | uint16(c.Content[1]), true
		}
	}

	return 0, false
}
