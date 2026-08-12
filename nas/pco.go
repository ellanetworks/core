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

// Container identifiers reserved to one direction (TS 24.008 §10.5.6.3).
const (
	// PCOContainerPDUSessionID is MS to network: the PDU session identity the MS
	// allocated for the PDN connection (TS 24.007 §11.2.3.1b).
	PCOContainerPDUSessionID uint16 = 0x001A
	PCOContainerSNSSAI       uint16 = 0x001B

	// PCOContainerFiveGSMCause is MS to network: the 5GSM cause the UE returns
	// when it rejects the mapped 5GS QoS parameters of an ESM accept
	// (TS 24.008 §10.5.6.3, TS 24.501 §6.1.4.1). It is Reserved network to MS.
	PCOContainerFiveGSMCause uint16 = 0x0022
)

// The mapped 5GS QoS parameters of a PDN connection, network to MS
// (TS 24.008 §10.5.6.3). They are what makes a PDN connection transferable to
// 5GS with its QoS intact; TS 23.501 §5.17.2.3.1 forbids sending them while the
// network advertises interworking without N26, so they belong to the N26 mode
// alone. Their contents are 5GS elements — TS 24.501 §9.11.4.13 QoS rules,
// §9.11.4.14 Session-AMBR and §9.11.4.12 QoS flow descriptions — passed already
// encoded, since those belong to the 5GS codec and this package sits beneath it.
const (
	PCOContainerQoSRules            uint16 = 0x001C
	PCOContainerSessionAMBR         uint16 = 0x001D
	PCOContainerQoSFlowDescriptions uint16 = 0x001F

	// The two-octet-length forms of the two elements that can outgrow a
	// one-octet length. The same identifiers name the *support indicators* the
	// MS sends to say it can receive them (TS 24.301 §6.5.1.2, TS 24.501
	// §6.4.1.2), which is why each is legal in both directions with different
	// meanings.
	PCOContainerQoSRulesTwoOctet            uint16 = 0x0023
	PCOContainerQoSFlowDescriptionsTwoOctet uint16 = 0x0024
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

// DNSServers sizes the resolver by its own address family, never the one the UE
// asked for: a 16-octet rendering of an IPv4 resolver is ::ffff:a.b.c.d, which
// is not reachable, and an IPv6 one truncated to 4 is dropped. TS 24.008
// §10.5.6.3 lets the network return containers the UE did not request.
func DNSServers(addr netip.Addr) [][]byte {
	if !addr.IsValid() {
		return nil
	}

	addr = addr.Unmap()

	if addr.Is4() {
		b := addr.As4()
		return [][]byte{b[:]}
	}

	b := addr.As16()

	return [][]byte{b[:]}
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

// The two elements that carry this value differ in how much they can hold: the
// protocol configuration options is a type 4 element of at most 253 octets, two
// of which are its IEI and length (TS 24.008 §10.5.6.3), and the extended
// protocol configuration options a type 6 element of at most 65538, three of
// which are its IEI and two-octet length (§10.5.6.3A). 5GS carries only the
// extended form (TS 24.501 §9.11.4.6).
const (
	maxPCOLen         = 251
	maxExtendedPCOLen = 65535
)

// ParseProtocolConfigurationOptions decodes a Protocol Configuration Options
// value: the configuration-protocol octet followed by identifier / length /
// content containers (TS 24.008 §10.5.6.3).
//
// Use [ParseExtendedProtocolConfigurationOptions] for the extended element,
// which holds far more: the containers that carry ATSSS, UE policy and
// service-level-AA data do not fit the classic one.
func ParseProtocolConfigurationOptions(b []byte, dir PCODirection) (ProtocolConfigurationOptions, error) {
	return parsePCO(b, dir, maxPCOLen)
}

// ParseExtendedProtocolConfigurationOptions decodes an Extended Protocol
// Configuration Options value (TS 24.008 §10.5.6.3A). It is the same content as
// the classic element under a two-octet length, so only the maximum differs.
func ParseExtendedProtocolConfigurationOptions(b []byte, dir PCODirection) (ProtocolConfigurationOptions, error) {
	return parsePCO(b, dir, maxExtendedPCOLen)
}

func parsePCO(b []byte, dir PCODirection, maxLen int) (ProtocolConfigurationOptions, error) {
	if dir == PCODirectionUnset {
		return ProtocolConfigurationOptions{}, fmt.Errorf("nas: protocol configuration options: no direction given")
	}

	if len(b) > maxLen {
		return ProtocolConfigurationOptions{}, fmt.Errorf(
			"nas: protocol configuration options is %d octets, want at most %d", len(b), maxLen)
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

	// The encoder caps at the extended element's maximum; a value bound for the
	// classic one is bounded further by its own one-octet length, which the
	// writer enforces when the element is framed.
	if len(out)-len(b) > maxExtendedPCOLen {
		return b, fmt.Errorf(
			"nas: protocol configuration options is %d octets, want at most %d", len(out)-len(b), maxExtendedPCOLen)
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

// PDUSessionID returns the identity the MS allocated for the PDN connection.
// Anything but the single octet TS 24.007 §11.2.3.1b defines, within the range a
// UE may allocate, reports absent.
func (p ProtocolConfigurationOptions) PDUSessionID() (uint8, bool) {
	if p.Direction != PCOMSToNetwork {
		return 0, false
	}

	for _, c := range p.Containers {
		if c.ID == PCOContainerPDUSessionID && len(c.Content) == 1 && c.Content[0] >= 1 && c.Content[0] <= 15 {
			return c.Content[0], true
		}
	}

	return 0, false
}

// FiveGSMCause reports the 5GSM cause the MS returned, if any. A UE that
// diagnoses an error in the mapped 5GS QoS parameters still accepts the ESM
// procedure and reports the cause here, having discarded those parameters
// (TS 24.501 §6.1.4.1).
func (p ProtocolConfigurationOptions) FiveGSMCause() (uint8, bool) {
	if p.Direction != PCOMSToNetwork {
		return 0, false
	}

	for _, c := range p.Containers {
		if c.ID == PCOContainerFiveGSMCause && len(c.Content) == 1 {
			return c.Content[0], true
		}
	}

	return 0, false
}

// NewSNSSAIContainer builds the network-to-MS S-NSSAI container: the value part
// of an S-NSSAI element (TS 24.501 §9.11.2.8) followed by one PLMN identity
// (TS 24.008 §10.5.5.36). The value part is passed already encoded, since the
// S-NSSAI element belongs to the 5GS codec and this package sits beneath it.
func NewSNSSAIContainer(snssaiValue []byte, plmn PLMN) (PCOContainer, error) {
	switch len(snssaiValue) {
	case 1, 2, 4, 5, 8:
	default:
		return PCOContainer{}, fmt.Errorf("nas: S-NSSAI container: value part is %d octets, want 1, 2, 4, 5 or 8", len(snssaiValue))
	}

	octets, err := plmn.Octets()
	if err != nil {
		return PCOContainer{}, fmt.Errorf("nas: S-NSSAI container: %w", err)
	}

	content := make([]byte, 0, len(snssaiValue)+len(octets))
	content = append(content, snssaiValue...)
	content = append(content, octets[:]...)

	return PCOContainer{ID: PCOContainerSNSSAI, Content: content}, nil
}

// maxOneOctetContainerLen is what a container's one-octet length field holds.
const maxOneOctetContainerLen = 0xFF

// NewQoSRulesContainer builds the network-to-MS mapped QoS rules container from
// an already-encoded QoS rules value (TS 24.501 §9.11.4.13). twoOctetSupported
// is what [ProtocolConfigurationOptions.SupportsTwoOctetQoSRules] reported for
// the MS's own options: with it the rules go under 0023H, whose length field is
// two octets, and without it under 001CH, which a long rule set does not fit.
func NewQoSRulesContainer(qosRulesValue []byte, twoOctetSupported bool) (PCOContainer, error) {
	return newTwoOctetCapableContainer(
		"QoS rules", PCOContainerQoSRules, PCOContainerQoSRulesTwoOctet, qosRulesValue, twoOctetSupported)
}

// NewQoSFlowDescriptionsContainer is NewQoSRulesContainer for the mapped QoS
// flow descriptions (TS 24.501 §9.11.4.12), under 001FH or its two-octet form
// 0024H.
func NewQoSFlowDescriptionsContainer(descriptionsValue []byte, twoOctetSupported bool) (PCOContainer, error) {
	return newTwoOctetCapableContainer(
		"QoS flow descriptions", PCOContainerQoSFlowDescriptions, PCOContainerQoSFlowDescriptionsTwoOctet,
		descriptionsValue, twoOctetSupported)
}

func newTwoOctetCapableContainer(name string, oneOctetID, twoOctetID uint16, value []byte, twoOctetSupported bool) (PCOContainer, error) {
	if len(value) == 0 {
		return PCOContainer{}, fmt.Errorf("nas: %s container: empty value", name)
	}

	if twoOctetSupported {
		return PCOContainer{ID: twoOctetID, Content: value}, nil
	}

	if len(value) > maxOneOctetContainerLen {
		return PCOContainer{}, fmt.Errorf(
			"nas: %s container: %d octets, and the MS did not indicate support for the two-octet length form",
			name, len(value))
	}

	return PCOContainer{ID: oneOctetID, Content: value}, nil
}

// NewSessionAMBRContainer builds the network-to-MS mapped Session-AMBR container
// from an already-encoded Session-AMBR value (TS 24.501 §9.11.4.14). It has only
// a one-octet-length form, which its fixed six-octet value never outgrows.
func NewSessionAMBRContainer(sessionAMBRValue []byte) (PCOContainer, error) {
	if len(sessionAMBRValue) == 0 || len(sessionAMBRValue) > maxOneOctetContainerLen {
		return PCOContainer{}, fmt.Errorf("nas: Session-AMBR container: value is %d octets", len(sessionAMBRValue))
	}

	return PCOContainer{ID: PCOContainerSessionAMBR, Content: sessionAMBRValue}, nil
}

// SupportsTwoOctetQoSRules reports whether the MS included the QoS rules with
// the length of two octets support indicator (TS 24.301 §6.5.1.2). The
// indicators are MS-to-network only: in the other direction the same identifier
// names the rules themselves.
func (p ProtocolConfigurationOptions) SupportsTwoOctetQoSRules() bool {
	return p.hasSupportIndicator(PCOContainerQoSRulesTwoOctet)
}

// SupportsTwoOctetQoSFlowDescriptions is SupportsTwoOctetQoSRules for the QoS
// flow descriptions indicator.
func (p ProtocolConfigurationOptions) SupportsTwoOctetQoSFlowDescriptions() bool {
	return p.hasSupportIndicator(PCOContainerQoSFlowDescriptionsTwoOctet)
}

func (p ProtocolConfigurationOptions) hasSupportIndicator(id uint16) bool {
	if p.Direction != PCOMSToNetwork {
		return false
	}

	for _, c := range p.Containers {
		if c.ID == id {
			return true
		}
	}

	return false
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
