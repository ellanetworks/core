// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import "testing"

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
			if action := runXDP(t, obj.UpfEntryFunc, tc.packet); action != ActionPass {
				t.Fatalf("got XDP action %d, want ActionPass (fail closed to kernel)", action)
			}
		})
	}
}

func ipv4OptionsTruncated() []byte {
	ip := ipv4Packet([4]byte{10, 0, 0, 1}, [4]byte{10, 0, 0, 2}, 17, nil)
	ip[0] = 0x4F

	return ethFrame(0x0800, ip)
}

func truncatedIPv6() []byte {
	short := make([]byte, 20)
	short[0] = 0x60

	return ethFrame(0x86DD, short)
}

func truncatedVLAN() []byte {
	return ethFrame(0x8100, []byte{0x00, 0x64})
}
