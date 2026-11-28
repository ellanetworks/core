// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ndp_test

import (
	"encoding/binary"
	"errors"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/ndp"
)

func FuzzBuildRA(f *testing.F) {
	ll1 := netip.MustParseAddr("fe80::1").AsSlice()
	ll2 := netip.MustParseAddr("fe80::2").AsSlice()
	allNodes := netip.MustParseAddr("ff02::1").AsSlice()
	gua := netip.MustParseAddr("2001:db8:abcd:1000::").AsSlice()
	unmasked := netip.MustParseAddr("2001:db8:abcd:1234:5678::1").AsSlice()
	v4 := netip.MustParseAddr("10.45.0.1").AsSlice()

	f.Add(ll1, ll2, gua, 64, uint32(1500), uint8(0xc0), uint8(0xc0), uint16(1800), uint32(0xffffffff), uint32(0xffffffff))
	f.Add(ll1, allNodes, gua, 48, uint32(0), uint8(0), uint8(0x80), uint16(0), uint32(0), uint32(0))
	f.Add(ll1, ll2, unmasked, 56, uint32(1280), uint8(0x80), uint8(0x40), uint16(9000), uint32(3600), uint32(1800))
	f.Add(ll1, ll2, gua, 0, uint32(0xffffffff), uint8(0x40), uint8(0), uint16(0xffff), uint32(1), uint32(2))
	f.Add(ll1, ll2, gua, 128, uint32(1), uint8(0), uint8(0), uint16(1), uint32(0), uint32(0))
	f.Add(ll1, ll2, gua, 129, uint32(1500), uint8(0), uint8(0), uint16(1800), uint32(0), uint32(0))
	f.Add(ll1, ll2, gua, -1, uint32(1500), uint8(0), uint8(0), uint16(1800), uint32(0), uint32(0))
	f.Add(ll1, ll2, v4, 24, uint32(1500), uint8(0), uint8(0xc0), uint16(1800), uint32(0), uint32(0))
	f.Add(v4, ll2, gua, 64, uint32(1500), uint8(0), uint8(0xc0), uint16(1800), uint32(0), uint32(0))
	f.Add(ll1, v4, gua, 64, uint32(1500), uint8(0), uint8(0xc0), uint16(1800), uint32(0), uint32(0))
	f.Add([]byte{}, ll2, []byte{}, 64, uint32(1500), uint8(0), uint8(0), uint16(0), uint32(0), uint32(0))

	f.Fuzz(func(t *testing.T, srcB, dstB, prefixB []byte, bits int, mtu uint32, raFlags, pioFlags uint8, lifetime uint16, valid, preferred uint32) {
		src, _ := netip.AddrFromSlice(srcB)
		dst, _ := netip.AddrFromSlice(dstB)
		prefixAddr, _ := netip.AddrFromSlice(prefixB)
		prefix := netip.PrefixFrom(prefixAddr, bits)

		params := ndp.RAParams{
			SrcIP:             src,
			DstIP:             dst,
			CurHopLimit:       64,
			Managed:           raFlags&0x80 != 0,
			Other:             raFlags&0x40 != 0,
			RouterLifetime:    lifetime,
			Prefix:            prefix,
			OnLink:            pioFlags&0x80 != 0,
			Autonomous:        pioFlags&0x40 != 0,
			ValidLifetime:     valid,
			PreferredLifetime: preferred,
			MTU:               mtu,
		}

		ra, err := ndp.BuildRA(params)

		switch {
		case !prefix.IsValid():
			if !errors.Is(err, ndp.ErrMissingPrefix) {
				t.Fatalf("invalid prefix: err = %v", err)
			}

			return
		case !src.Is6() || !dst.Is6():
			if !errors.Is(err, ndp.ErrNotIPv6) {
				t.Fatalf("non-IPv6 address: err = %v", err)
			}

			return
		case err != nil:
			t.Fatalf("BuildRA rejected valid params: %v", err)
		}

		want := 16 + 32
		if mtu > 0 {
			want += 8
		}

		if len(ra) != want {
			t.Fatalf("RA is %d bytes, want %d", len(ra), want)
		}

		if ra[0] != ndp.ICMPv6TypeRouterAdvertisement || ra[1] != 0 || binary.BigEndian.Uint16(ra[2:4]) != 0 {
			t.Fatalf("bad RA header % x", ra[:4])
		}

		if (ra[5]&0x80 != 0) != params.Managed || (ra[5]&0x40 != 0) != params.Other || ra[5]&0x3f != 0 {
			t.Fatalf("RA flags = %#x", ra[5])
		}

		if binary.BigEndian.Uint16(ra[6:8]) != lifetime {
			t.Fatal("router lifetime not encoded")
		}

		pio := ra[16:48]
		if pio[0] != ndp.NDPOptionPrefixInformation || pio[1] != 4 || int(pio[2]) != prefix.Bits() {
			t.Fatalf("bad Prefix Information option % x", pio[:4])
		}

		if (pio[3]&0x80 != 0) != params.OnLink || (pio[3]&0x40 != 0) != params.Autonomous || pio[3]&0x3f != 0 {
			t.Fatalf("PIO flags = %#x", pio[3])
		}

		if binary.BigEndian.Uint32(pio[4:8]) != valid || binary.BigEndian.Uint32(pio[8:12]) != preferred {
			t.Fatal("prefix lifetimes not encoded")
		}

		if netip.AddrFrom16([16]byte(pio[16:32])) != netip.AddrFrom16(prefix.Masked().Addr().As16()) {
			t.Fatalf("prefix %s encoded as % x", prefix, pio[16:32])
		}

		if mtu > 0 {
			opt := ra[48:56]
			if opt[0] != ndp.NDPOptionMTU || opt[1] != 1 || binary.BigEndian.Uint32(opt[4:8]) != mtu {
				t.Fatalf("bad MTU option % x", opt)
			}
		}

		ndp.SetICMPv6Checksum(src, dst, ra)

		if got := ndp.ICMPv6Checksum(src, dst, ra); got != 0 {
			t.Fatalf("checksum verification = %#04x, want 0", got)
		}
	})
}
