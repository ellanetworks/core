// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package ngap

import (
	"fmt"
	"net/netip"

	"github.com/free5gc/aper"
)

// encodeTransportLayerAddress builds the APER BitString for the NGAP
// TransportLayerAddress IE per 3GPP TS 38.414 Section 5.1:
//   - 32 bits:  IPv4 only
//   - 128 bits: IPv6 only
//   - 160 bits: IPv4 (4B) || IPv6 (16B) — dual-stack
func encodeTransportLayerAddress(ipv4, ipv6 netip.Addr) (aper.BitString, error) {
	haveV4 := ipv4.IsValid() && ipv4.Is4()
	haveV6 := ipv6.IsValid() && ipv6.Is6()

	switch {
	case haveV4 && haveV6:
		// 160-bit dual-stack: IPv4 (4 bytes) || IPv6 (16 bytes)
		buf := make([]byte, 20)
		v4 := ipv4.As4()
		copy(buf[0:4], v4[:])

		v6 := ipv6.As16()
		copy(buf[4:20], v6[:])

		return aper.BitString{Bytes: buf, BitLength: 160}, nil
	case haveV6:
		// 128-bit IPv6 only
		v6 := ipv6.As16()
		b := make([]byte, 16)
		copy(b, v6[:])

		return aper.BitString{Bytes: b, BitLength: 128}, nil
	case haveV4:
		// 32-bit IPv4 only
		v4 := ipv4.As4()
		b := make([]byte, 4)
		copy(b, v4[:])

		return aper.BitString{Bytes: b, BitLength: 32}, nil
	default:
		return aper.BitString{}, fmt.Errorf("encodeTransportLayerAddress: both IPv4 and IPv6 addresses are invalid")
	}
}
