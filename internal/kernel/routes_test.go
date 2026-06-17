// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package kernel

import (
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"
)

func TestGwOrVia_SameFamily(t *testing.T) {
	// IPv4 dst + IPv4 gw → Gw set, Via nil
	v4 := netip.MustParseAddr("192.168.1.1")
	p4 := netip.MustParsePrefix("10.0.0.0/24")

	gw, via := gwOrVia(p4, v4)
	if gw == nil {
		t.Fatal("expected non-nil Gw for matching families")
	}

	if via != nil {
		t.Error("expected nil Via for matching families")
	}

	// IPv6 dst + IPv6 gw → Gw set, Via nil
	v6 := netip.MustParseAddr("2001:db8::1")
	p6 := netip.MustParsePrefix("2001:db8::/32")

	gw, via = gwOrVia(p6, v6)
	if gw == nil {
		t.Fatal("expected non-nil Gw for matching families")
	}

	if via != nil {
		t.Error("expected nil Via for matching families")
	}
}

func TestGwOrVia_MixedFamily(t *testing.T) {
	// IPv4 dst + IPv6 gw → Gw nil, Via with FAMILY_V6
	v6 := netip.MustParseAddr("2001:db8::1")
	p4 := netip.MustParsePrefix("10.0.0.0/24")

	gw, via := gwOrVia(p4, v6)
	if gw != nil {
		t.Error("expected nil Gw for mismatched families")
	}

	if via == nil {
		t.Fatal("expected non-nil Via for mismatched families")
	}

	if via.AddrFamily != netlink.FAMILY_V6 {
		t.Errorf("expected FAMILY_V6, got %d", via.AddrFamily)
	}

	if len(via.Addr) != 16 {
		t.Errorf("expected 16-byte address, got %d", len(via.Addr))
	}

	// IPv6 dst + IPv4 gw → Gw nil, Via with FAMILY_V4
	v4 := netip.MustParseAddr("192.168.1.1")
	p6 := netip.MustParsePrefix("2001:db8::/32")

	gw, via = gwOrVia(p6, v4)
	if gw != nil {
		t.Error("expected nil Gw for mismatched families")
	}

	if via == nil {
		t.Fatal("expected non-nil Via for mismatched families")
	}

	if via.AddrFamily != netlink.FAMILY_V4 {
		t.Errorf("expected FAMILY_V4, got %d", via.AddrFamily)
	}

	if len(via.Addr) != 4 {
		t.Errorf("expected 4-byte address, got %d", len(via.Addr))
	}
}

func TestGwOrVia_Invalid(t *testing.T) {
	p4 := netip.MustParsePrefix("10.0.0.0/24")

	gw, via := gwOrVia(p4, netip.Addr{})
	if gw != nil {
		t.Error("expected nil Gw for invalid address")
	}

	if via != nil {
		t.Error("expected nil Via for invalid address")
	}
}

func TestPrefixToIPNet(t *testing.T) {
	p := netip.MustParsePrefix("10.0.0.0/24")

	ipNet := prefixToIPNet(p)
	if ipNet.String() != "10.0.0.0/24" {
		t.Errorf("got %q, want %q", ipNet.String(), "10.0.0.0/24")
	}

	p6 := netip.MustParsePrefix("2001:db8::/32")

	ipNet = prefixToIPNet(p6)
	if ipNet.String() != "2001:db8::/32" {
		t.Errorf("got %q, want %q", ipNet.String(), "2001:db8::/32")
	}

	// /0 prefix
	zero := netip.MustParsePrefix("0.0.0.0/0")

	ipNet = prefixToIPNet(zero)
	if ipNet.String() != "0.0.0.0/0" {
		t.Errorf("got %q, want %q", ipNet.String(), "0.0.0.0/0")
	}
}
