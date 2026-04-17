// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"net/netip"
	"testing"
)

func TestIPToIn6Addr_IPv4(t *testing.T) {
	addr := netip.MustParseAddr("192.168.1.1")
	got := IPToIn6Addr(addr)

	// IPv4-mapped IPv6: bytes 0-9 = 0x00, bytes 10-11 = 0xff, bytes 12-15 = IPv4
	want := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 192, 168, 1, 1}
	if got != want {
		t.Errorf("IPToIn6Addr(IPv4): got %v, want %v", got, want)
	}
}

func TestIPToIn6Addr_IPv6(t *testing.T) {
	addr := netip.MustParseAddr("2001:db8::1")
	got := IPToIn6Addr(addr)
	want := addr.As16()

	if got != want {
		t.Errorf("IPToIn6Addr(IPv6): got %v, want %v", got, want)
	}
}

func TestIn6AddrToIP_IPv4Mapped(t *testing.T) {
	in6 := [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 10, 0, 0, 1}
	got := In6AddrToIP(in6)
	want := netip.MustParseAddr("10.0.0.1")

	if got != want {
		t.Errorf("In6AddrToIP(IPv4-mapped): got %v, want %v", got, want)
	}
}

func TestIn6AddrToIP_IPv6Native(t *testing.T) {
	want := netip.MustParseAddr("2001:db8::cafe")
	in6 := IPToIn6Addr(want)
	got := In6AddrToIP(in6)

	if got != want {
		t.Errorf("In6AddrToIP(IPv6): got %v, want %v", got, want)
	}
}

func TestIPToIn6Addr_RoundTrip_IPv4(t *testing.T) {
	original := netip.MustParseAddr("172.16.0.42")
	got := In6AddrToIP(IPToIn6Addr(original))

	if got != original {
		t.Errorf("round-trip IPv4: got %v, want %v", got, original)
	}
}

func TestIPToIn6Addr_RoundTrip_IPv6(t *testing.T) {
	original := netip.MustParseAddr("fd00::1")
	got := In6AddrToIP(IPToIn6Addr(original))

	if got != original {
		t.Errorf("round-trip IPv6: got %v, want %v", got, original)
	}
}
