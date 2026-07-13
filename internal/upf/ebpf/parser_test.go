// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import "testing"

// TestMalformedL3FailsClosed checks that malformed or truncated L3 headers do
// not abort the data path: each is passed to the kernel (XDP_PASS) rather than
// returning XDP_ABORTED.
func TestMalformedL3FailsClosed(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	tests := []struct {
		name   string
		packet []byte
	}{
		{"ipv4 options claimed but truncated", ipv4OptionsTruncated()},
		{"truncated ipv6 header", truncatedIPv6()},
		{"truncated vlan tag", truncatedVLAN()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if action := runXDP(t, obj.UpfEntryFunc, tc.packet); action != XDP_PASS {
				t.Fatalf("got XDP action %d, want XDP_PASS (fail closed to kernel)", action)
			}
		})
	}
}

// ipv4OptionsTruncated builds a frame whose IPv4 header claims a 60-byte length
// (IHL 15) but only carries the 20-byte base header.
func ipv4OptionsTruncated() []byte {
	ip := ipv4Packet([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, 17, nil)
	ip[0] = 0x4F // version 4, IHL 15

	return ethFrame(0x0800, ip)
}

// truncatedIPv6 builds a frame with an IPv6 ethertype but fewer than the 40
// header bytes.
func truncatedIPv6() []byte {
	short := make([]byte, 20)
	short[0] = 0x60 // version 6

	return ethFrame(0x86DD, short)
}

// truncatedVLAN builds a frame with an 802.1Q ethertype but no room for the
// VLAN tag.
func truncatedVLAN() []byte {
	return ethFrame(0x8100, []byte{0x00, 0x64})
}
