// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"encoding/binary"
	"errors"
	"os"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
)

// TestParseGTPTruncatedExtension is a regression test for GHSA-jq33-34hf-27q6.
//
// parse_gtp() advances the packet cursor by a fixed optional-header word
// whenever any of the GTP-U E/S/PN flags is set, without checking that those
// bytes are present. A packet carrying a valid uplink TEID but a truncated
// optional header then reaches remove_gtp_header(), which fails, so the XDP
// program returns XDP_ABORTED instead of handling the malformed packet
// gracefully.
//
// The data plane must fail closed: a malformed GTP-U packet must never abort
// the path, regardless of whether its TEID matches an installed PDR. This test
// asserts that correct behavior, so it FAILS against the current vulnerable
// parser and passes once parse_gtp bounds-checks the optional header.
func TestParseGTPTruncatedExtension(t *testing.T) {
	requireProgTestRun(t)

	const validTEID = 0x11223344

	obj := loadUplinkTestObjects(t, validTEID)

	tests := []struct {
		name string
		teid uint32
	}{
		{name: "valid TEID, truncated extension", teid: validTEID},
		{name: "unknown TEID, truncated extension", teid: 0xDEADBEEF},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			abortedBefore := GetN3Aborted(obj)

			action := runXDP(t, obj.UpfN3N6EntrypointFunc, malformedUplinkGTPv4(tc.teid))

			if action == XDP_ABORTED {
				t.Fatalf("malformed GTP-U packet (teid=%#x) returned XDP_ABORTED; "+
					"parse_gtp must fail closed and not abort the data path", tc.teid)
			}

			if delta := GetN3Aborted(obj) - abortedBefore; delta != 0 {
				t.Fatalf("teid=%#x: XDP_ABORTED counter advanced by %d, want 0", tc.teid, delta)
			}
		})
	}
}

// requireProgTestRun skips when the environment cannot exercise
// BPF_PROG_TEST_RUN, e.g. on a developer machine without privileges. CI sets
// EBPF_REQUIRE_PRIVILEGED so that a missing privileged run fails the build
// instead of silently passing as a skip.
func requireProgTestRun(t *testing.T) {
	t.Helper()

	if os.Geteuid() != 0 {
		const msg = "BPF_PROG_TEST_RUN requires root/CAP_BPF"
		if os.Getenv("EBPF_REQUIRE_PRIVILEGED") != "" {
			t.Fatal(msg)
		}

		t.Skip(msg + "; skipping")
	}

	if err := rlimit.RemoveMemlock(); err != nil {
		t.Fatalf("cannot remove memlock rlimit: %v", err)
	}
}

// loadUplinkTestObjects loads the N3/N6 program and installs a single
// forwarding uplink PDR for teid.
//
// n3_ifindex is set to 0 so that the test-run default ingress_ifindex (0) is
// treated as the N3 uplink. n6_ifindex is set to 1 (loopback) so the in-path
// bpf_check_mtu() call resolves a real device and succeeds for small packets,
// ensuring the only XDP_ABORTED source under test is the GTP parser.
func loadUplinkTestObjects(t *testing.T, teid uint32) *BpfObjects {
	t.Helper()

	obj := NewBpfObjects(false, false, 0 /* n3 */, 1 /* n6=lo */, 0, 0)
	if err := obj.Load(); err != nil {
		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			t.Fatalf("load N3/N6 objects: verifier error: %+v", ve)
		}

		t.Fatalf("load N3/N6 objects: %v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	pdr := PdrInfo{
		OuterHeaderRemoval: 0,                 // OHR_GTP_U_UDP_IPv4 -> decapsulation branch
		IMSI:               "001010000000001", // must be numeric or the FAR is zeroed
		Far:                FarInfo{Action: 0x02 /* FAR_FORW */},
		Qer:                QerInfo{GateStatusUL: 0 /* GATE_STATUS_OPEN */, MaxBitrateUL: 0 /* unlimited */},
	}
	if err := obj.PutPdrUplink(teid, pdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	return obj
}

func runXDP(t *testing.T, prog *ebpf.Program, packet []byte) uint32 {
	t.Helper()

	action, err := prog.Run(&ebpf.RunOptions{Data: packet})
	if err != nil {
		if errors.Is(err, ebpf.ErrNotSupported) {
			t.Skipf("BPF_PROG_TEST_RUN for XDP not supported on this kernel: %v", err)
		}

		t.Fatalf("run XDP program: %v", err)
	}

	return action
}

// malformedUplinkGTPv4 builds an Ethernet/IPv4/UDP(2152) frame carrying an
// 8-byte GTP-U base header with the E flag set and message type G_PDU, but no
// optional/extension bytes following it.
func malformedUplinkGTPv4(teid uint32) []byte {
	gtp := make([]byte, 8)
	gtp[0] = 0x34 // version=1, PT=1, E=1
	gtp[1] = 0xFF // GTPU_G_PDU
	// gtp[2:4] message length = 0
	binary.BigEndian.PutUint32(gtp[4:8], teid)

	return wrapIPv4UDP(gtp, GTPUDPPort)
}

// GTPUDPPort mirrors GTP_UDP_PORT in the C data plane.
const GTPUDPPort = 2152

// wrapIPv4UDP prepends Ethernet + IPv4 + UDP headers around payload, addressed
// to dstPort. Header checksums are left zero: the XDP parse path validates
// lengths and bounds, not checksums.
func wrapIPv4UDP(payload []byte, dstPort uint16) []byte {
	const (
		ethLen = 14
		ipLen  = 20
		udpLen = 8
	)

	frame := make([]byte, ethLen+ipLen+udpLen+len(payload))

	eth := frame[:ethLen]
	copy(eth[0:6], []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x02})  // dst MAC
	copy(eth[6:12], []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}) // src MAC
	binary.BigEndian.PutUint16(eth[12:14], 0x0800)              // ETH_P_IP

	ip := frame[ethLen : ethLen+ipLen]
	ip[0] = 0x45 // version 4, IHL 5
	binary.BigEndian.PutUint16(ip[2:4], uint16(ipLen+udpLen+len(payload)))
	ip[8] = 64 // TTL
	ip[9] = 17 // IPPROTO_UDP
	copy(ip[12:16], []byte{10, 0, 0, 1})
	copy(ip[16:20], []byte{10, 0, 0, 2})

	udp := frame[ethLen+ipLen : ethLen+ipLen+udpLen]
	binary.BigEndian.PutUint16(udp[0:2], 2152) // source port (arbitrary)
	binary.BigEndian.PutUint16(udp[2:4], dstPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpLen+len(payload)))

	copy(frame[ethLen+ipLen+udpLen:], payload)

	return frame
}
