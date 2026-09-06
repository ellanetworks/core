// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"net"
	"sort"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func addr(ip string, flags int, validLft int) netlink.Addr {
	return netlink.Addr{
		IPNet:    &net.IPNet{IP: net.ParseIP(ip)},
		Flags:    flags,
		ValidLft: validLft,
	}
}

func scopedAddr(ip string, flags int, validLft int, scope int) netlink.Addr {
	a := addr(ip, flags, validLft)
	a.Scope = scope

	return a
}

func TestAddrIsUsableRejectsUnfitAddresses(t *testing.T) {
	forever := 0xFFFFFFFF

	cases := []struct {
		name string
		in   netlink.Addr
		want bool
	}{
		{"permanent ipv4", addr("192.0.2.1", unix.IFA_F_PERMANENT, forever), true},
		{"permanent ipv6", addr("2001:db8::1", unix.IFA_F_PERMANENT, forever), true},
		{"dynamic slaac", addr("2001:db8::2", unix.IFA_F_MANAGETEMPADDR, 1102804), true},
		{"loopback", addr("127.0.0.1", unix.IFA_F_PERMANENT, forever), true},
		{"deprecated", addr("2001:db8::3", unix.IFA_F_DEPRECATED, 100), true},
		{"tentative", addr("2001:db8::4", unix.IFA_F_TENTATIVE, forever), false},
		{"dad failed", addr("2001:db8::5", unix.IFA_F_DADFAILED, forever), false},
		{"rfc 4941 temporary", addr("2001:db8::6", unix.IFA_F_SECONDARY, 169059), false},
		{"ipv4 alias", addr("192.0.2.2", unix.IFA_F_SECONDARY, forever), false},
		{"link local ipv6", addr("fe80::1", unix.IFA_F_PERMANENT, forever), false},
		{"link local ipv4", addr("169.254.1.1", unix.IFA_F_PERMANENT, forever), false},
		{"unspecified", addr("::", unix.IFA_F_PERMANENT, forever), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, got := addrIsUsable(tc.in); got != tc.want {
				t.Fatalf("addrIsUsable(%s) = %v, want %v", tc.in.IP, got, tc.want)
			}
		})
	}
}

func TestAddrRanksPermanentThenLongestLived(t *testing.T) {
	forever := 0xFFFFFFFF

	in := []netlink.Addr{
		addr("2001:db8::3267", unix.IFA_F_NOPREFIXROUTE, 3267),
		addr("2001:db8::51aa", unix.IFA_F_MANAGETEMPADDR|unix.IFA_F_NOPREFIXROUTE, 1102804),
		addr("2001:db8::57a7", unix.IFA_F_PERMANENT, forever),
	}

	sort.SliceStable(in, func(i, j int) bool { return addrRanksBefore(in[i], in[j]) })

	want := []string{"2001:db8::57a7", "2001:db8::51aa", "2001:db8::3267"}

	for i, w := range want {
		if got := in[i].IP.String(); got != w {
			t.Fatalf("position %d = %s, want %s", i, got, w)
		}
	}
}

func TestAddrRanksTieBreaksDeterministically(t *testing.T) {
	a := addr("192.0.2.9", unix.IFA_F_PERMANENT, 100)
	b := addr("192.0.2.1", unix.IFA_F_PERMANENT, 100)

	if !addrRanksBefore(b, a) {
		t.Fatal("expected the lower address to rank first")
	}

	if addrRanksBefore(a, b) {
		t.Fatal("ranking must be antisymmetric")
	}
}

func TestAddrRanksWiderScopeFirst(t *testing.T) {
	forever := 0xFFFFFFFF

	in := []netlink.Addr{
		scopedAddr("10.9.9.9", unix.IFA_F_PERMANENT, forever, unix.RT_SCOPE_HOST),
		scopedAddr("192.0.2.7", unix.IFA_F_PERMANENT, forever, unix.RT_SCOPE_LINK),
		scopedAddr("192.168.40.8", unix.IFA_F_NOPREFIXROUTE, 35630, unix.RT_SCOPE_UNIVERSE),
	}

	sort.SliceStable(in, func(i, j int) bool { return addrRanksBefore(in[i], in[j]) })

	want := []string{"192.168.40.8", "192.0.2.7", "10.9.9.9"}

	for i, w := range want {
		if got := in[i].IP.String(); got != w {
			t.Fatalf("position %d = %s, want %s", i, got, w)
		}
	}
}

func TestAddrRanksDeprecatedLast(t *testing.T) {
	forever := 0xFFFFFFFF

	in := []netlink.Addr{
		addr("2001:db8::3", unix.IFA_F_PERMANENT|unix.IFA_F_DEPRECATED, forever),
		addr("2001:db8::4", unix.IFA_F_NOPREFIXROUTE, 3267),
	}

	sort.SliceStable(in, func(i, j int) bool { return addrRanksBefore(in[i], in[j]) })

	want := []string{"2001:db8::4", "2001:db8::3"}

	for i, w := range want {
		if got := in[i].IP.String(); got != w {
			t.Fatalf("position %d = %s, want %s", i, got, w)
		}
	}
}
