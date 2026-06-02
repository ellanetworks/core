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

// TestEntrypointUnknownInterfaceAborts checks the entrypoint's interface
// dispatch: a packet whose ingress_ifindex matches neither n3_ifindex nor
// n6_ifindex is aborted. The test-run ingress is 1, so loading n3/n6 as 2/3
// makes it match neither.
func TestEntrypointUnknownInterfaceAborts(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 2, 3)

	action := runXDP(t, obj.UpfN3N6EntrypointFunc, ethFrame(0x0800, innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)))

	if action != XDP_ABORTED {
		t.Fatalf("packet on unconfigured interface got XDP action %d, want XDP_ABORTED (%d)", action, XDP_ABORTED)
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

// loadProgram loads the N3/N6 program with the given interface indices.
//
// XDP BPF_PROG_TEST_RUN runs with ingress_ifindex == 1 (loopback). The
// entrypoint tags a packet N3 or N6 by matching that against n3Ifindex/n6Ifindex,
// and the in-path bpf_check_mtu needs a real device (loopback, index 1). GTP
// decap runs from handle_ip4 regardless of the N3/N6 tag, and
// handle_gtp_packet/handle_n6_packet set ctx->interface themselves — so
// verdict/DataOut tests don't depend on the tag; only stats-map selection does
// (see stats_test.go).
func loadProgram(t *testing.T, n3Ifindex, n6Ifindex int) *BpfObjects {
	t.Helper()

	return loadProgramConfig(t, false, false, n3Ifindex, n6Ifindex, 0, 0)
}

// loadProgramVLAN is loadProgram with configurable N3/N6 VLAN IDs.
func loadProgramVLAN(t *testing.T, n3Ifindex, n6Ifindex int, n3Vlan, n6Vlan uint32) *BpfObjects {
	t.Helper()

	return loadProgramConfig(t, false, false, n3Ifindex, n6Ifindex, n3Vlan, n6Vlan)
}

// loadProgramFlow is loadProgram with flow accounting enabled.
func loadProgramFlow(t *testing.T, n3Ifindex, n6Ifindex int) *BpfObjects {
	t.Helper()

	return loadProgramConfig(t, true, false, n3Ifindex, n6Ifindex, 0, 0)
}

func loadProgramConfig(t *testing.T, flowAccounting, masquerade bool, n3Ifindex, n6Ifindex int, n3Vlan, n6Vlan uint32) *BpfObjects {
	t.Helper()

	obj := NewBpfObjects(flowAccounting, masquerade, n3Ifindex, n6Ifindex, n3Vlan, n6Vlan)
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

// loadN3N6Program is the loader for the GTP/uplink tests. n3_ifindex 0 keeps the
// routing ifindex-mismatch check disabled (stable forwarding verdicts) and
// n6_ifindex 1 (loopback) is the valid MTU/egress device. The GTP decap path in
// handle_ip4 runs regardless of the entrypoint's N3/N6 tag; these tests assert on
// the packet/verdict, not the stats-map selection.
func loadN3N6Program(t *testing.T) *BpfObjects {
	t.Helper()

	return loadProgram(t, 0, 1)
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

// TestGTPDecapsulationInnerIPv6 checks that an uplink G-PDU carrying an inner
// IPv6 packet is decapsulated to that IPv6 packet, with the Ethernet protocol
// set to IPv6.
func TestGTPDecapsulationInnerIPv6(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x21222325

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, 0)

	inner := innerIPv6UDP(testUEv6, 53)

	action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(teid, inner))

	if action == XDP_DROP || action == XDP_ABORTED {
		t.Fatalf("decapsulated packet got XDP action %d, want a forwarding action", action)
	}

	if len(out) != ethHdrLen+len(inner) {
		t.Fatalf("decapsulated frame length = %d, want %d", len(out), ethHdrLen+len(inner))
	}

	if proto := binary.BigEndian.Uint16(out[12:14]); proto != 0x86DD {
		t.Fatalf("ethertype = %#04x, want 0x86dd (IPv6)", proto)
	}

	if !bytes.Equal(out[ethHdrLen:], inner) {
		t.Fatalf("inner packet altered by decapsulation:\n got %x\nwant %x", out[ethHdrLen:], inner)
	}
}

// TestGTPForwardIPv4 checks GTP-to-GTP forwarding: when the uplink FAR requests
// outer-header creation, the packet is not decapsulated but its outer IPv4
// source/destination and TEID are rewritten to the FAR's values, with a valid
// outer checksum and the inner packet preserved.
func TestGTPForwardIPv4(t *testing.T) {
	requireProgTestRun(t)

	const (
		lookupTEID = 0x11112222
		outerTEID  = 0x33334444
	)

	local := [4]byte{192, 168, 50, 1}
	remote := [4]byte{203, 0, 113, 9}

	obj := loadN3N6Program(t)
	putForwardingUplinkPDRGTP(t, obj, lookupTEID, local, remote, outerTEID)

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)

	action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(lookupTEID, inner))

	if action == XDP_ABORTED {
		t.Fatal("forwarded packet got XDP_ABORTED")
	}

	if len(out) != ethHdrLen+gtpV4EncapLen+len(inner) {
		t.Fatalf("forwarded frame length = %d, want %d (no decap)", len(out), ethHdrLen+gtpV4EncapLen+len(inner))
	}

	f := parseGTPv4Frame(t, out)

	if !f.outerChecksumOK {
		t.Error("outer IPv4 header checksum is invalid after tunnel rewrite")
	}

	if f.outerSrc != local {
		t.Errorf("outer src IP = %v, want %v (FAR localip)", f.outerSrc, local)
	}

	if f.outerDst != remote {
		t.Errorf("outer dst IP = %v, want %v (FAR remoteip)", f.outerDst, remote)
	}

	if f.teid != outerTEID {
		t.Errorf("rewritten TEID = %#x, want %#x", f.teid, uint32(outerTEID))
	}

	if !bytes.Equal(f.inner, inner) {
		t.Errorf("inner packet altered by tunnel rewrite:\n got %x\nwant %x", f.inner, inner)
	}
}

// TestGTPDecapsulationIPv6Transport checks that a G-PDU received over an IPv6
// transport (outer IPv6/UDP) is decapsulated to its inner IPv4 packet.
func TestGTPDecapsulationIPv6Transport(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x41424344

	obj := loadN3N6Program(t)
	putForwardingUplinkPDRv6Outer(t, obj, teid)

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)

	action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, uplinkGPDUv6(teid, inner))

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

// putForwardingUplinkPDRv6Outer installs an uplink PDR for teid whose outer
// header is removed as GTP-U over IPv6.
func putForwardingUplinkPDRv6Outer(t *testing.T, obj *BpfObjects, teid uint32) {
	t.Helper()

	pdr := PdrInfo{
		OuterHeaderRemoval: 1, // OHR_GTP_U_UDP_IPv6
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02 /* FAR_FORW */},
		Qer:                QerInfo{GateStatusUL: 0 /* GATE_STATUS_OPEN */, MaxBitrateUL: 0 /* unlimited */},
	}
	if err := obj.PutPdrUplink(teid, pdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}
}
