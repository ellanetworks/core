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

// TestParseGTPTruncatedExtension checks that a malformed GTP-U packet never
// aborts the XDP data path, whether or not its TEID matches an installed PDR.
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
				t.Fatalf("teid=%#x: malformed GTP-U packet returned XDP_ABORTED; the parser must fail closed", tc.teid)
			}

			if delta := GetN3Aborted(obj) - abortedBefore; delta != 0 {
				t.Fatalf("teid=%#x: XDP_ABORTED counter advanced by %d, want 0", tc.teid, delta)
			}
		})
	}
}

// requireProgTestRun skips when the test cannot run privileged, unless
// EBPF_REQUIRE_PRIVILEGED is set, which makes the missing privilege fatal.
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

// loadUplinkTestObjects loads the N3/N6 program with one forwarding uplink PDR
// for teid. n3_ifindex 0 makes the test-run packet (ingress_ifindex 0) take the
// N3 path; n6_ifindex 1 (loopback) gives the in-path MTU check a real device so
// the parser is the only remaining source of an abort.
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
		OuterHeaderRemoval: 0,                 // OHR_GTP_U_UDP_IPv4
		IMSI:               "001010000000001", // non-numeric IMSI zeroes the FAR
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

// malformedUplinkGTPv4 builds a GTP-U frame that sets the E flag but omits the
// optional header word the flag implies.
func malformedUplinkGTPv4(teid uint32) []byte {
	gtp := make([]byte, 8)
	gtp[0] = 0x34 // version=1, PT=1, E=1
	gtp[1] = 0xFF // GTPU_G_PDU
	binary.BigEndian.PutUint32(gtp[4:8], teid)

	return wrapIPv4UDP(gtp, GTPUDPPort)
}

const GTPUDPPort = 2152

// wrapIPv4UDP wraps payload in Ethernet/IPv4/UDP headers. Checksums are left
// zero; the XDP parse path validates lengths and bounds, not checksums.
func wrapIPv4UDP(payload []byte, dstPort uint16) []byte {
	const (
		ethLen = 14
		ipLen  = 20
		udpLen = 8
	)

	frame := make([]byte, ethLen+ipLen+udpLen+len(payload))

	eth := frame[:ethLen]
	copy(eth[0:6], []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x02})
	copy(eth[6:12], []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01})
	binary.BigEndian.PutUint16(eth[12:14], 0x0800) // ETH_P_IP

	ip := frame[ethLen : ethLen+ipLen]
	ip[0] = 0x45 // version 4, IHL 5
	binary.BigEndian.PutUint16(ip[2:4], uint16(ipLen+udpLen+len(payload)))
	ip[8] = 64 // TTL
	ip[9] = 17 // IPPROTO_UDP
	copy(ip[12:16], []byte{10, 0, 0, 1})
	copy(ip[16:20], []byte{10, 0, 0, 2})

	udp := frame[ethLen+ipLen : ethLen+ipLen+udpLen]
	binary.BigEndian.PutUint16(udp[0:2], 2152)
	binary.BigEndian.PutUint16(udp[2:4], dstPort)
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpLen+len(payload)))

	copy(frame[ethLen+ipLen+udpLen:], payload)

	return frame
}
