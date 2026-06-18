// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"fmt"
	"net/netip"
)

// EncodeTransportLayerAddress encodes a GTP-U transport endpoint as the octets of
// the S1AP/NGAP Transport Layer Address (TS 36.413 / TS 38.413, semantics per
// TS 36.414): 4 octets for IPv4, 16 for IPv6, or 20 (IPv4 followed by IPv6) when
// the endpoint has both. It mirrors how the gNB N3 endpoint is signalled on the
// 5G side so the 4G S1-U endpoint is advertised the same way.
func EncodeTransportLayerAddress(v4, v6 netip.Addr) ([]byte, error) {
	haveV4 := v4.IsValid() && v4.Is4()
	haveV6 := v6.IsValid() && v6.Is6()

	switch {
	case haveV4 && haveV6:
		a4 := v4.As4()
		a6 := v6.As16()
		b := make([]byte, 20)
		copy(b[0:4], a4[:])
		copy(b[4:20], a6[:])

		return b, nil
	case haveV6:
		a6 := v6.As16()
		b := make([]byte, 16)
		copy(b, a6[:])

		return b, nil
	case haveV4:
		a4 := v4.As4()
		b := make([]byte, 4)
		copy(b, a4[:])

		return b, nil
	default:
		return nil, fmt.Errorf("no valid transport layer address")
	}
}

// DecodeTransportLayerAddress decodes the octets of an S1AP/NGAP Transport Layer
// Address (TS 36.413 / TS 38.413) into its IPv4 and/or IPv6 endpoints: 4 octets
// yield an IPv4 address, 16 an IPv6 address, and 20 both (IPv4 followed by IPv6).
// Either returned address may be the zero value when that family is absent.
func DecodeTransportLayerAddress(b []byte) (v4, v6 netip.Addr, err error) {
	switch len(b) {
	case 4:
		v4, _ = netip.AddrFromSlice(b)
	case 16:
		v6, _ = netip.AddrFromSlice(b)
	case 20:
		v4, _ = netip.AddrFromSlice(b[0:4])
		v6, _ = netip.AddrFromSlice(b[4:20])
	default:
		return netip.Addr{}, netip.Addr{}, fmt.Errorf("transport layer address: unexpected length %d", len(b))
	}

	return v4, v6, nil
}
