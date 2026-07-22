// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// Protocol/container identifiers for the (extended) protocol configuration
// options (TS 24.008 §10.5.6.3). A request (uplink) and its answer (downlink)
// share the container identifier.
const (
	PCOContainerDNSServerIPv6Address uint16 = 0x0003
	PCOContainerDNSServerIPv4Address uint16 = 0x000D
	PCOContainerIPv4LinkMTU          uint16 = 0x0010
)

// pcoConfigProtocolPPP is the first PCO octet: extension bit set, configuration
// protocol PPP for use with the IP PDU session (TS 24.008 §10.5.6.3).
const pcoConfigProtocolPPP uint8 = 0x80

// BuildProtocolConfigurationOptions encodes the network-to-UE (extended) Protocol
// Configuration Options value (TS 24.008 §10.5.6.3): one DNS-server container per
// address — IPv4 (4 octets) under 0x000D, IPv6 (16 octets) under 0x0003 — and,
// when ipv4LinkMTU is non-zero, the IPv4 Link MTU (container 0x0010, a 2-octet MTU
// in octets). It returns the IE value (without the IEI/length), nil when there is
// nothing to advertise. IPv6 has no PCO MTU container; the IPv6 link MTU is
// delivered via the Router Advertisement (TS 24.501).
func BuildProtocolConfigurationOptions(dnsServers [][]byte, ipv4LinkMTU uint16) []byte {
	if len(dnsServers) == 0 && ipv4LinkMTU == 0 {
		return nil
	}

	var w common.Writer

	w.U8(pcoConfigProtocolPPP)

	for _, addr := range dnsServers {
		switch len(addr) {
		case 4:
			w.U16(PCOContainerDNSServerIPv4Address)
			w.U8(4)
			w.Raw(addr)
		case 16:
			w.U16(PCOContainerDNSServerIPv6Address)
			w.U8(16)
			w.Raw(addr)
		}
	}

	if ipv4LinkMTU != 0 {
		w.U16(PCOContainerIPv4LinkMTU)
		w.U8(2)
		w.U16(ipv4LinkMTU)
	}

	return w.Bytes()
}

// ParsePCOContainerIDs reads a PCO IE content and returns the protocol/container
// identifiers it carries, in order (TS 24.008 §10.5.6.3). It is used to inspect
// a UE's requested containers; the container contents are not returned.
func ParsePCOContainerIDs(content []byte) ([]uint16, error) {
	r := common.NewReader(content)

	if _, err := r.U8(); err != nil { // configuration protocol octet
		return nil, err
	}

	var ids []uint16

	for r.Remaining() > 0 {
		id, err := r.U16()
		if err != nil {
			return nil, err
		}

		length, err := r.U8()
		if err != nil {
			return nil, err
		}

		if _, err := r.Bytes(int(length)); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, nil
}
