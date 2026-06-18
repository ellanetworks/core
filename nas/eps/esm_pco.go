// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// Protocol Configuration Options container identifiers (TS 24.008 §10.5.6.3).
const (
	pcoContainerDNSServerIPv6Address uint16 = 0x0003
	pcoContainerDNSServerIPv4Address uint16 = 0x000D
	pcoContainerIPv4LinkMTU          uint16 = 0x0010
)

// pcoConfigProtocolPPP is the first PCO octet: extension bit set, configuration
// protocol PPP for use with the IP PDP/PDN context (TS 24.008 §10.5.6.3).
const pcoConfigProtocolPPP uint8 = 0x80

// BuildProtocolConfigurationOptions encodes the network-to-UE Protocol
// Configuration Options value (TS 24.008 §10.5.6.3): one DNS-server container
// per address — IPv4 (4 octets) under 0x000D, IPv6 (16 octets) under 0x0003 —
// and, when ipv4LinkMTU is non-zero, the IPv4 Link MTU (container 0x0010, a
// 2-octet MTU in octets). It returns the IE value (without the IEI/length), nil
// when there is nothing to advertise. IPv6 has no PCO MTU container; the IPv6
// link MTU is delivered via the Router Advertisement (TS 24.301 clause 6).
func BuildProtocolConfigurationOptions(dnsServers [][]byte, ipv4LinkMTU uint16) []byte {
	if len(dnsServers) == 0 && ipv4LinkMTU == 0 {
		return nil
	}

	var w common.Writer

	w.U8(pcoConfigProtocolPPP)

	for _, addr := range dnsServers {
		switch len(addr) {
		case 4:
			w.U16(pcoContainerDNSServerIPv4Address)
			w.U8(4)
			w.Raw(addr)
		case 16:
			w.U16(pcoContainerDNSServerIPv6Address)
			w.U8(16)
			w.Raw(addr)
		}
	}

	if ipv4LinkMTU != 0 {
		w.U16(pcoContainerIPv4LinkMTU)
		w.U8(2)
		w.U16(ipv4LinkMTU)
	}

	return w.Bytes()
}

// ParseProtocolConfigurationOptions decodes a network-to-UE PCO value (the
// inverse of BuildProtocolConfigurationOptions, TS 24.008 §10.5.6.3): the
// configuration-protocol octet followed by id/length/content containers. It
// returns the DNS-server addresses (4-octet IPv4 or 16-octet IPv6, in order) and
// the IPv4 Link MTU (0 when absent). Unknown containers are skipped.
func ParseProtocolConfigurationOptions(pco []byte) (dnsServers [][]byte, ipv4LinkMTU uint16, err error) {
	r := common.NewReader(pco)

	if _, err = r.U8(); err != nil { // configuration protocol octet
		return nil, 0, err
	}

	for r.Remaining() > 0 {
		id, idErr := r.U16()
		if idErr != nil {
			return nil, 0, idErr
		}

		length, lenErr := r.U8()
		if lenErr != nil {
			return nil, 0, lenErr
		}

		content, contentErr := r.Bytes(int(length))
		if contentErr != nil {
			return nil, 0, contentErr
		}

		switch id {
		case pcoContainerDNSServerIPv4Address, pcoContainerDNSServerIPv6Address:
			dnsServers = append(dnsServers, content)
		case pcoContainerIPv4LinkMTU:
			if len(content) == 2 {
				ipv4LinkMTU = uint16(content[0])<<8 | uint16(content[1])
			}
		}
	}

	return dnsServers, ipv4LinkMTU, nil
}
