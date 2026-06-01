// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
)

// ethHdrLen is the size of an Ethernet header, the offset of the inner packet in
// a decapsulated frame.
const ethHdrLen = 14

// TestParseGTPTruncatedExtension checks that a malformed GTP-U packet fails
// closed: the parser rejects it and the packet is passed to the kernel
// (XDP_PASS) rather than aborting the data path, whether or not its TEID matches
// an installed PDR.
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
			action := runXDP(t, obj.UpfN3N6EntrypointFunc, malformedUplinkGTPv4(tc.teid))

			if action != XDP_PASS {
				t.Fatalf("teid=%#x: malformed GTP-U packet got XDP action %d, want XDP_PASS (%d)", tc.teid, action, XDP_PASS)
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

// loadN3N6Program loads the N3/N6 program. n3_ifindex 0 makes the test-run
// packet (ingress_ifindex 0) take the N3 path; n6_ifindex 1 (loopback) gives the
// in-path MTU check a real device so a parse failure is the only source of an
// abort.
func loadN3N6Program(t *testing.T) *BpfObjects {
	t.Helper()

	obj := NewBpfObjects(false, false, 0, 1, 0, 0)
	if err := obj.Load(); err != nil {
		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			t.Fatalf("load N3/N6 objects: verifier error: %+v", ve)
		}

		t.Fatalf("load N3/N6 objects: %v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	return obj
}

// putForwardingUplinkPDR installs an uplink PDR for teid that forwards (FAR FORW,
// QER gate open, unlimited rate) and applies SDF filter filterIndex; 0 disables
// filtering.
func putForwardingUplinkPDR(t *testing.T, obj *BpfObjects, teid, filterIndex uint32) {
	t.Helper()

	pdr := PdrInfo{
		OuterHeaderRemoval: 0,                 // OHR_GTP_U_UDP_IPv4
		IMSI:               "001010000000001", // non-numeric IMSI zeroes the FAR
		Far:                FarInfo{Action: 0x02 /* FAR_FORW */},
		Qer:                QerInfo{GateStatusUL: 0 /* GATE_STATUS_OPEN */, MaxBitrateUL: 0 /* unlimited */},
		FilterMapIndex:     filterIndex,
	}
	if err := obj.PutPdrUplink(teid, pdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}
}

func loadUplinkTestObjects(t *testing.T, teid uint32) *BpfObjects {
	t.Helper()

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, 0)

	return obj
}

func runXDP(t *testing.T, prog *ebpf.Program, packet []byte) uint32 {
	t.Helper()

	action, _ := runXDPOut(t, prog, packet)

	return action
}

// runXDPOut runs the program and returns its action together with the resulting
// packet. BPF_PROG_TEST_RUN re-slices the output buffer to the packet's final
// length, so a head adjustment (GTP decapsulation/encapsulation) is reflected.
func runXDPOut(t *testing.T, prog *ebpf.Program, packet []byte) (uint32, []byte) {
	t.Helper()

	opts := &ebpf.RunOptions{
		Data:    packet,
		DataOut: make([]byte, len(packet)+256), // headroom for encapsulation growth
	}

	action, err := prog.Run(opts)
	if err != nil {
		if errors.Is(err, ebpf.ErrNotSupported) {
			t.Skipf("BPF_PROG_TEST_RUN for XDP not supported on this kernel: %v", err)
		}

		t.Fatalf("run XDP program: %v", err)
	}

	return action, opts.DataOut
}

// TestGTPDecapsulation checks that a well-formed uplink G-PDU is decapsulated to
// its inner IP packet: the outer GTP-U/UDP/IP headers are stripped and the inner
// packet is preserved byte for byte. The final action depends on the host's
// routing table, so the assertion is on the output packet, not the verdict.
func TestGTPDecapsulation(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x21222324

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, 0)

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)

	action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(teid, inner))

	// The exact forwarding code (XDP_TX vs XDP_REDIRECT) depends on the host
	// FIB, but the decapsulated packet must not be dropped or aborted.
	if action == XDP_DROP || action == XDP_ABORTED {
		t.Fatalf("decapsulated packet got XDP action %d, want a forwarding action", action)
	}

	if len(out) != ethHdrLen+len(inner) {
		t.Fatalf("decapsulated frame length = %d, want %d", len(out), ethHdrLen+len(inner))
	}

	if proto := binary.BigEndian.Uint16(out[12:14]); proto != 0x0800 {
		t.Fatalf("ethertype = %#04x, want 0x0800 (IPv4)", proto)
	}

	if !bytes.Equal(out[ethHdrLen:], inner) {
		t.Fatalf("inner packet altered by decapsulation:\n got %x\nwant %x", out[ethHdrLen:], inner)
	}
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
