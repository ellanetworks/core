// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package bgp

import (
	"net/netip"
	"testing"
)

func TestBuildPathRejectsIPv6PrefixWithoutIPv6NextHop(t *testing.T) {
	b := &BGPService{n6AddrV4: netip.MustParseAddr("10.0.0.1")}

	if _, err := b.buildPath(netip.MustParsePrefix("2001:db8:cafe::/48")); err == nil {
		t.Fatal("expected an error when the N6 interface has no global IPv6 address")
	}
}

func TestBuildPathRejectsIPv4PrefixWithoutIPv4NextHop(t *testing.T) {
	b := &BGPService{n6AddrV6: netip.MustParseAddr("2001:db8::1")}

	if _, err := b.buildPath(netip.MustParsePrefix("10.45.0.0/16")); err == nil {
		t.Fatal("expected an error when the N6 interface has no IPv4 address")
	}
}

func TestBuildPathUsesN6AddressesAsNextHop(t *testing.T) {
	b := &BGPService{
		n6AddrV4: netip.MustParseAddr("10.0.0.1"),
		n6AddrV6: netip.MustParseAddr("2001:db8::1"),
	}

	v4, err := b.buildPath(netip.MustParsePrefix("10.45.0.0/16"))
	if err != nil {
		t.Fatalf("ipv4 path: %v", err)
	}

	if got := v4.Attrs[1].String(); got != "{Nexthop: 10.0.0.1}" {
		t.Fatalf("ipv4 next-hop = %s", got)
	}

	v6, err := b.buildPath(netip.MustParsePrefix("2001:db8:cafe::/48"))
	if err != nil {
		t.Fatalf("ipv6 path: %v", err)
	}

	if got := v6.Attrs[1].String(); got != "{MpReach(ipv6-unicast): {Nexthop: 2001:db8::1, NLRIs: [2001:db8:cafe::/48:0]}}" {
		t.Fatalf("ipv6 next-hop = %s", got)
	}
}
