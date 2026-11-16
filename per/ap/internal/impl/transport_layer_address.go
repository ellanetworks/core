// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package impl

import (
	"encoding/hex"
	"errors"
	"net/netip"
	"strconv"

	"github.com/ellanetworks/core/per"
)

const (
	tnlIPv4Bits = 32
	tnlIPv6Bits = 128
	tnlDualBits = tnlIPv4Bits + tnlIPv6Bits
)

var ErrTransportLayerAddress = errors.New("ap: invalid transport layer address")

type TransportLayerAddress struct {
	octets string
	length int
}

func TransportLayerAddressFrom(addr netip.Addr) (TransportLayerAddress, error) {
	addr = addr.Unmap()

	switch {
	case addr.Is4():
		a := addr.As4()

		return TransportLayerAddress{octets: string(a[:]), length: tnlIPv4Bits + 1}, nil
	case addr.Is6():
		a := addr.As16()

		return TransportLayerAddress{octets: string(a[:]), length: tnlIPv6Bits + 1}, nil
	default:
		return TransportLayerAddress{}, ErrTransportLayerAddress
	}
}

func TransportLayerAddressFromAddrs(ipv4, ipv6 netip.Addr) (TransportLayerAddress, error) {
	ipv4 = ipv4.Unmap()

	if ipv4.IsValid() && !ipv4.Is4() {
		return TransportLayerAddress{}, ErrTransportLayerAddress
	}

	if ipv6.IsValid() && (!ipv6.Is6() || ipv6.Is4In6()) {
		return TransportLayerAddress{}, ErrTransportLayerAddress
	}

	switch {
	case ipv4.IsValid() && ipv6.IsValid():
		a4, a6 := ipv4.As4(), ipv6.As16()

		return TransportLayerAddress{octets: string(a4[:]) + string(a6[:]), length: tnlDualBits + 1}, nil
	case ipv4.IsValid():
		return TransportLayerAddressFrom(ipv4)
	case ipv6.IsValid():
		return TransportLayerAddressFrom(ipv6)
	default:
		return TransportLayerAddress{}, ErrTransportLayerAddress
	}
}

func TransportLayerAddressFromOctets(b []byte) TransportLayerAddress {
	if len(b) == 0 {
		return TransportLayerAddress{}
	}

	return newTransportLayerAddress(b, len(b)*8)
}

func TransportLayerAddressFromBits(b []byte, nbits int) (TransportLayerAddress, bool) {
	if nbits < 1 || nbits > len(b)*8 {
		return TransportLayerAddress{}, false
	}

	return newTransportLayerAddress(b, nbits), true
}

func newTransportLayerAddress(b []byte, nbits int) TransportLayerAddress {
	n := (nbits + 7) / 8
	octets := make([]byte, n)
	copy(octets, b[:n])

	if rem := nbits % 8; rem != 0 {
		octets[n-1] &= byte(0xff << (8 - rem))
	}

	return TransportLayerAddress{octets: string(octets), length: nbits + 1}
}

func (a TransportLayerAddress) Bits() int { return max(a.length-1, 0) }

func (a TransportLayerAddress) Bytes() []byte { return []byte(a.octets) }

func (a TransportLayerAddress) IsZero() bool { return a.length == 0 }

func (a TransportLayerAddress) Valid() bool {
	switch a.Bits() {
	case tnlIPv4Bits, tnlIPv6Bits, tnlDualBits:
		return true
	default:
		return false
	}
}

func (a TransportLayerAddress) IPv4() netip.Addr {
	if n := a.Bits(); n != tnlIPv4Bits && n != tnlDualBits {
		return netip.Addr{}
	}

	return netip.AddrFrom4([4]byte([]byte(a.octets[:4])))
}

func (a TransportLayerAddress) IPv6() netip.Addr {
	switch a.Bits() {
	case tnlIPv6Bits:
		return netip.AddrFrom16([16]byte([]byte(a.octets)))
	case tnlDualBits:
		return netip.AddrFrom16([16]byte([]byte(a.octets[4:])))
	default:
		return netip.Addr{}
	}
}

func (a TransportLayerAddress) String() string {
	switch a.Bits() {
	case tnlIPv4Bits:
		return a.IPv4().String()
	case tnlIPv6Bits:
		return a.IPv6().String()
	case tnlDualBits:
		return a.IPv4().String() + "," + a.IPv6().String()
	default:
		return hex.EncodeToString([]byte(a.octets)) + "/" + strconv.Itoa(a.Bits())
	}
}

func (a TransportLayerAddress) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if a.IsZero() {
		return ErrTransportLayerAddress
	}

	return per.EncodeBitString(w, enc, 1, tnlDualBits, true, true, true, []byte(a.octets), a.Bits())
}

func (a *TransportLayerAddress) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, nbits, err := per.DecodeBitString(r, enc, 1, tnlDualBits, true, true, true)
	if err != nil {
		return err
	}

	*a = newTransportLayerAddress(b, nbits)

	return nil
}
