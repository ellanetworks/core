package kernel

import (
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"
)

func TestAddrToVia(t *testing.T) {
	// IPv4 address → FAMILY_V4
	v4 := netip.MustParseAddr("192.168.1.1")

	via := addrToVia(v4)
	if via == nil {
		t.Fatal("expected non-nil Via for valid IPv4 address")
	}

	if via.AddrFamily != netlink.FAMILY_V4 {
		t.Errorf("expected FAMILY_V4, got %d", via.AddrFamily)
	}

	if len(via.Addr) != 4 {
		t.Errorf("expected 4-byte address, got %d", len(via.Addr))
	}

	// IPv6 address → FAMILY_V6
	v6 := netip.MustParseAddr("2001:db8::1")

	via = addrToVia(v6)
	if via == nil {
		t.Fatal("expected non-nil Via for valid IPv6 address")
	}

	if via.AddrFamily != netlink.FAMILY_V6 {
		t.Errorf("expected FAMILY_V6, got %d", via.AddrFamily)
	}

	if len(via.Addr) != 16 {
		t.Errorf("expected 16-byte address, got %d", len(via.Addr))
	}

	// Invalid address → nil
	if addrToVia(netip.Addr{}) != nil {
		t.Error("expected nil for invalid address")
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
