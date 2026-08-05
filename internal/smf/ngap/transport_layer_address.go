// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"net"
	"net/netip"

	"github.com/ellanetworks/core/internal/models"
	libngap "github.com/ellanetworks/core/ngap"
)

// encodeTransportLayerAddress builds the NGAP TransportLayerAddress IE per
// 3GPP TS 38.414:
//   - 32 bits:  IPv4 only
//   - 128 bits: IPv6 only
//   - 160 bits: IPv4 (4B) || IPv6 (16B) — dual-stack
func encodeTransportLayerAddress(ipv4, ipv6 netip.Addr) (libngap.TransportLayerAddress, error) {
	b, err := models.EncodeTransportLayerAddress(ipv4, ipv6)
	if err != nil {
		return nil, err
	}

	return libngap.TransportLayerAddress(b), nil
}

// ParseTransportLayerAddress adapts the library's netip form to the net.IP the
// SMF's session state carries.
func ParseTransportLayerAddress(a libngap.TransportLayerAddress) (ipv4 net.IP, ipv6 net.IP) {
	v4, v6 := a.IPs()

	if v4.IsValid() {
		ipv4 = net.IP(v4.AsSlice())
	}

	if v6.IsValid() {
		ipv6 = net.IP(v6.AsSlice())
	}

	return ipv4, ipv6
}
