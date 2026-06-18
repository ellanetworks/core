// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"net"
	"net/netip"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
)

// encodeTransportLayerAddress builds the APER BitString for the NGAP
// TransportLayerAddress IE per 3GPP TS 38.414 Section 5.1:
//   - 32 bits:  IPv4 only
//   - 128 bits: IPv6 only
//   - 160 bits: IPv4 (4B) || IPv6 (16B) — dual-stack
func encodeTransportLayerAddress(ipv4, ipv6 netip.Addr) (aper.BitString, error) {
	b, err := models.EncodeTransportLayerAddress(ipv4, ipv6)
	if err != nil {
		return aper.BitString{}, err
	}

	return aper.BitString{Bytes: b, BitLength: uint64(len(b) * 8)}, nil
}

// ParseTransportLayerAddress extracts IPv4 and/or IPv6 from a NGAP TransportLayerAddress
// per 3GPP TS 38.414 Section 5.1.
func ParseTransportLayerAddress(bs aper.BitString) (ipv4 net.IP, ipv6 net.IP) {
	switch {
	case bs.BitLength == 32 && len(bs.Bytes) >= 4:
		ipv4 = net.IP(bs.Bytes[0:4])
	case bs.BitLength == 128 && len(bs.Bytes) >= 16:
		ipv6 = net.IP(bs.Bytes[0:16])
	case bs.BitLength == 160 && len(bs.Bytes) >= 20:
		ipv4 = net.IP(bs.Bytes[0:4])
		ipv6 = net.IP(bs.Bytes[4:20])
	}

	return
}
