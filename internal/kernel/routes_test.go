// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package kernel

import (
	"net"
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

func TestPrefixFromIPNet_V4Default(t *testing.T) {
	tests := []struct {
		name  string
		ipNet *net.IPNet
		want  string
	}{
		{
			name:  "v4 default as netlink synthesises it",
			ipNet: &net.IPNet{IP: net.IPv4zero, Mask: net.CIDRMask(0, 32)},
			want:  "0.0.0.0/0",
		},
		{
			name:  "v6 default",
			ipNet: &net.IPNet{IP: net.IPv6zero, Mask: net.CIDRMask(0, 128)},
			want:  "::/0",
		},
		{
			name:  "non-default v4 via RTA_DST, 4-byte",
			ipNet: &net.IPNet{IP: net.IP{10, 0, 0, 0}, Mask: net.CIDRMask(8, 32)},
			want:  "10.0.0.0/8",
		},
		{
			name:  "v4-mapped with a v6-relative prefix length",
			ipNet: &net.IPNet{IP: net.IPv4(10, 0, 0, 0), Mask: net.CIDRMask(104, 128)},
			want:  "10.0.0.0/8",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := prefixFromIPNet(tt.ipNet)
			if !ok {
				t.Fatalf("prefixFromIPNet(%v) returned !ok", tt.ipNet)
			}

			if got.String() != tt.want {
				t.Errorf("got %q, want %q", got.String(), tt.want)
			}

			if got.Addr().Is4In6() {
				t.Errorf("got an IPv4-mapped address: %q", got.String())
			}
		})
	}
}

func TestPrefixFromIPNet_RejectsAbsentMask(t *testing.T) {
	// A missing mask must not be read as zero ones, which would invent a
	// default route out of a host route.
	if p, ok := prefixFromIPNet(&net.IPNet{IP: net.IP{10, 0, 0, 1}}); ok {
		t.Errorf("expected !ok for absent mask, got %q", p.String())
	}
}

func TestAddrFromNetIP_Unmaps(t *testing.T) {
	got, ok := addrFromNetIP(net.IPv4(10, 0, 20, 129))
	if !ok {
		t.Fatal("addrFromNetIP returned !ok")
	}

	if !got.Is4() || got.String() != "10.0.20.129" {
		t.Errorf("got %q (Is4=%v), want 10.0.20.129 (Is4=true)", got.String(), got.Is4())
	}
}

func TestPrefixToIPNet_V4Mapped(t *testing.T) {
	p := netip.PrefixFrom(netip.MustParseAddr("::ffff:0.0.0.0"), 0)

	ipNet := prefixToIPNet(p)
	if len(ipNet.IP) != net.IPv4len {
		t.Errorf("got a %d-byte IP, want %d", len(ipNet.IP), net.IPv4len)
	}

	if len(ipNet.Mask) != net.IPv4len {
		t.Errorf("got a %d-byte mask, want %d", len(ipNet.Mask), net.IPv4len)
	}

	if ipNet.String() != "0.0.0.0/0" {
		t.Errorf("got %q, want %q", ipNet.String(), "0.0.0.0/0")
	}
}

func TestGwOrVia_V4MappedDestination(t *testing.T) {
	p := netip.PrefixFrom(netip.MustParseAddr("::ffff:0.0.0.0"), 0)

	gw, via := gwOrVia(p, netip.MustParseAddr("10.0.20.129"))
	if via != nil {
		t.Errorf("expected nil Via for an IPv4-mapped dst with an IPv4 gw, got %+v", via)
	}

	if gw == nil {
		t.Fatal("expected non-nil Gw")
	}

	if len(gw) != net.IPv4len {
		t.Errorf("got a %d-byte Gw, want %d", len(gw), net.IPv4len)
	}
}
