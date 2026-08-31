// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

func event(ifindex uint32, family uint32, addr []byte) []byte {
	b := make([]byte, 24)
	binary.NativeEndian.PutUint32(b[0:4], ifindex)
	binary.NativeEndian.PutUint32(b[4:8], family)
	copy(b[8:], addr)

	return b
}

func TestParseNoNeighEvent(t *testing.T) {
	v4 := netip.MustParseAddr("10.3.0.7")
	v6 := netip.MustParseAddr("fe80::1234")

	for _, tc := range []struct {
		name    string
		in      []byte
		ifindex int
		addr    netip.Addr
		ok      bool
	}{
		{"ipv4", event(3, 2, v4.AsSlice()), 3, v4, true},
		{"ipv6 link-local", event(7, 10, v6.AsSlice()), 7, v6, true},
		{"unknown family", event(3, 99, v4.AsSlice()), 0, netip.Addr{}, false},
		{"zero ifindex", event(0, 2, v4.AsSlice()), 0, netip.Addr{}, false},
		{"short record", make([]byte, 12), 0, netip.Addr{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ifindex, addr, ok := parseNoNeighEvent(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}

			if !tc.ok {
				return
			}

			if ifindex != tc.ifindex {
				t.Errorf("ifindex = %d, want %d", ifindex, tc.ifindex)
			}

			if addr != tc.addr {
				t.Errorf("addr = %v, want %v", addr, tc.addr)
			}
		})
	}
}
