// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
	"time"

	"github.com/cilium/ebpf/ringbuf"
)

const (
	t2ReasmDev  = "ellt2rs"
	t2ReasmPeer = "ellt2rsp"
)

func TestFragmentedTransportHandedToStackForReassembly(t *testing.T) {
	requireProgTestRun(t)

	const teid = uint32(0x5A5A00f1)

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, 0)

	if err := obj.SetReasmVethIfindex(0x7fff); err != nil {
		t.Fatalf("set reassembly veth: %v", err)
	}

	action := runXDP(t, obj.UpfEntryFunc, firstFragmentGPDU(teid))

	if action != ActionPass {
		t.Fatalf("first fragment of a GTP-U transport got XDP action %d, want ActionPass (%d): the kernel reassembles it", action, ActionPass)
	}

	if got := DropCount(obj, Uplink, "fragmented_transport"); got != 0 {
		t.Errorf("fragmented_transport = %d, want 0", got)
	}

	if got := GetDatapathCounters(obj)[Uplink].Forwarded[ActionPass]; got != 1 {
		t.Errorf("uplink pass = %d, want 1", got)
	}
}

func setupReasmVeth(t *testing.T, f *t2) int {
	t.Helper()

	_, _ = ipCmd("link", "del", t2ReasmDev)

	addVethPair(t, t2ReasmDev, t2ReasmPeer)

	dev := ifByName(t, t2ReasmDev)

	attachDatapath(t, f.obj, f.obj.UpfUplinkFunc, dev.Index)

	if err := f.obj.SetReasmVethIfindex(dev.Index); err != nil {
		t.Fatalf("set reassembly veth: %v", err)
	}

	return ifByName(t, t2ReasmPeer).Index
}

func TestReassembledGPDUIsDecapsulatedAndForwarded(t *testing.T) {
	requireProgTestRun(t)

	const ulTEID = 0x52454153

	f := setupT2(t, false)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})

	peer := setupReasmVeth(t, f)

	inner := ipv4Packet(ueIP, serverIP, 17, udpDatagram(4000, 5201, bytesOf(1472)))

	n6 := f.captureN6(t)

	inject(t, peer, uplinkGPDU(ulTEID, inner))

	got := captureMatching(n6, time.Second, func(fr []byte) bool {
		return isInnerIPv4(fr, 17, serverIP)
	})
	if got == nil {
		t.Fatal("the reassembled G-PDU's inner packet did not egress on N6")
	}

	if !bytes.Equal(got[ethHdrLen:], inner) {
		t.Errorf("forwarded packet differs from the inner packet: got %d bytes, want %d", len(got)-ethHdrLen, len(inner))
	}
}

func TestReassemblyVethDoesNotLoopFragmentsOrForeignFrames(t *testing.T) {
	requireProgTestRun(t)

	const ulTEID = 0x52454154

	f := setupT2(t, false)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})

	peer := setupReasmVeth(t, f)

	inject(t, peer, firstFragmentGPDU(ulTEID))
	inject(t, peer, ethFrame(0x0800, ipv4Packet(testGNBIP, testUPFN3IP, 17, udpDatagram(4000, 4001, bytesOf(64)))))

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if DropCount(f.obj, Uplink, "fragmented_transport") == 1 && DropCount(f.obj, Uplink, "reinject_unowned") >= 1 {
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Errorf("fragmented_transport = %d, reinject_unowned = %d: a re-injected fragment must not go back to the stack, nor anything else the datapath does not own",
		DropCount(f.obj, Uplink, "fragmented_transport"), DropCount(f.obj, Uplink, "reinject_unowned"))
}

func openFragTxReader(t *testing.T, obj *BpfObjects) *ringbuf.Reader {
	t.Helper()

	r, err := ringbuf.NewReader(obj.FragTxMap)
	if err != nil {
		t.Fatalf("open frag_tx_map reader: %v", err)
	}

	t.Cleanup(func() { _ = r.Close() })

	return r
}

func shrinkN3MTU(t *testing.T) {
	t.Helper()

	if out, err := ipCmd("link", "set", t2N3Dev, "mtu", "1280"); err != nil {
		t.Fatalf("shrink N3 MTU: %v: %s", err, out)
	}
}

func TestDownlinkDFClearIsHandedOffForOuterFragmentation(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x46524754
		qfi  = 7
	)

	f := setupT2(t, false)
	putDownlinkPDR(t, f.obj, ueIP, teid, testUPFN3IP, testGNBIP, qfi)
	shrinkN3MTU(t)

	if err := f.obj.SetFragTxEnabled(true); err != nil {
		t.Fatalf("enable outer fragmentation: %v", err)
	}

	ring := openFragTxReader(t, f.obj)
	n3 := f.captureN3(t)

	inner := ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 4001, bytesOf(1400)))
	f.injectDownlink(t, ethFrame(0x0800, inner))

	pkt := readRingRecord(t, ring)
	if pkt == nil {
		t.Fatal("no encapsulated packet was handed off for outer fragmentation")
	}

	if len(pkt) != gtpV4EncapLen+len(inner) {
		t.Fatalf("handed-off packet is %d bytes, want %d (outer headers + inner)", len(pkt), gtpV4EncapLen+len(inner))
	}

	if pkt[0]>>4 != 4 || !bytes.Equal(pkt[12:16], testUPFN3IP[:]) || !bytes.Equal(pkt[16:20], testGNBIP[:]) {
		t.Errorf("outer IPv4 header = % x, want %v -> %v", pkt[:20], testUPFN3IP, testGNBIP)
	}

	if dport := binary.BigEndian.Uint16(pkt[22:24]); dport != GTPUDPPort {
		t.Errorf("outer UDP destination port = %d, want %d", dport, GTPUDPPort)
	}

	if got := binary.BigEndian.Uint32(pkt[32:36]); got != teid {
		t.Errorf("GTP-U TEID = %#x, want %#x", got, teid)
	}

	if !bytes.Equal(pkt[gtpV4EncapLen:], inner) {
		t.Error("the encapsulated inner packet differs from the one sent")
	}

	if fr := captureMatching(n3, 200*time.Millisecond, func(fr []byte) bool {
		return len(fr) > 1280
	}); fr != nil {
		t.Error("an oversized frame egressed on N3")
	}

	counters := GetDatapathCounters(f.obj)[Downlink]
	if got := counters.Forwarded[ActionSlowPath]; got != 1 {
		t.Errorf("slow-path forwards = %d, want 1", got)
	}

	if got := DropCount(f.obj, Downlink, "df_not_set"); got != 0 {
		t.Errorf("df_not_set = %d, want 0", got)
	}
}

func TestDownlinkDFSetStillTriggersFragmentationNeeded(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x46524755

	f := setupT2(t, false)
	putDownlinkPDR(t, f.obj, ueIP, teid, testUPFN3IP, testGNBIP, 7)
	shrinkN3MTU(t)

	if err := f.obj.SetFragTxEnabled(true); err != nil {
		t.Fatalf("enable outer fragmentation: %v", err)
	}

	ring := openFragTxReader(t, f.obj)
	capFD := f.captureN6(t)

	big := withDF(ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 4001, bytesOf(1400))))
	f.injectDownlink(t, ethFrame(0x0800, big))

	if got := captureMatching(capFD, time.Second, func(fr []byte) bool {
		if !isInnerIPv4(fr, 1, serverIP) {
			return false
		}

		icmp := fr[ethHdrLen+20:]

		return len(icmp) >= 8 && icmp[0] == 3 && icmp[1] == 4
	}); got == nil {
		t.Fatal("a DF-set packet did not produce an ICMP fragmentation-needed")
	}

	ring.SetDeadline(time.Now().Add(200 * time.Millisecond))

	var rec ringbuf.Record
	if err := ring.ReadInto(&rec); err == nil {
		t.Error("a DF-set packet was handed off for outer fragmentation")
	}
}

func TestDownlinkDFClearDroppedWhenOuterFragmentationUnavailable(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x46524756

	f := setupT2(t, false)
	putDownlinkPDR(t, f.obj, ueIP, teid, testUPFN3IP, testGNBIP, 7)
	shrinkN3MTU(t)

	f.injectDownlink(t, ethFrame(0x0800, ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 4001, bytesOf(1400)))))

	deadline := time.Now().Add(time.Second)
	for DropCount(f.obj, Downlink, "df_not_set") != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("df_not_set = %d, want 1", DropCount(f.obj, Downlink, "df_not_set"))
		}

		time.Sleep(20 * time.Millisecond)
	}
}

func TestDownlinkIPv6AtMinimumMTUIsOuterFragmented(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x46524736
		qfi  = 7
	)

	serverV6 := [16]byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	f := setupT2(t, false)
	putDownlinkPDRv6UE(t, f.obj, netip.MustParseAddr("2001:db8::"), teid, testUPFN3IP, testGNBIP, qfi)
	shrinkN3MTU(t)

	if err := f.obj.SetFragTxEnabled(true); err != nil {
		t.Fatalf("enable outer fragmentation: %v", err)
	}

	ring := openFragTxReader(t, f.obj)

	inner := ipv6Packet(serverV6, testUEv6, 17, udpDatagram(4000, 53, bytesOf(1280-40-8)))
	f.injectDownlink(t, ethFrame(0x86DD, inner))

	pkt := readRingRecord(t, ring)
	if pkt == nil {
		t.Fatal("a 1280-byte IPv6 packet was not handed off: a link must carry 1280 bytes (RFC 8200 §5)")
	}

	if !bytes.Equal(pkt[gtpV4EncapLen:], inner) {
		t.Error("the encapsulated inner packet differs from the one sent")
	}
}

func TestDownlinkIPv6AboveMinimumMTUAdvertises1280(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x46524737

	serverV6 := [16]byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	f := setupT2(t, false)
	putDownlinkPDRv6UE(t, f.obj, netip.MustParseAddr("2001:db8::"), teid, testUPFN3IP, testGNBIP, 7)
	shrinkN3MTU(t)

	if err := f.obj.SetFragTxEnabled(true); err != nil {
		t.Fatalf("enable outer fragmentation: %v", err)
	}

	capFD := f.captureN6(t)

	big := ipv6Packet(serverV6, testUEv6, 17, udpDatagram(4000, 53, bytesOf(1300)))
	f.injectDownlink(t, ethFrame(0x86DD, big))

	got := captureMatching(capFD, time.Second, func(fr []byte) bool {
		if len(fr) < ethHdrLen+40+8 || fr[12] != 0x86 || fr[13] != 0xDD {
			return false
		}

		return fr[ethHdrLen+6] == 58 && fr[ethHdrLen+40] == 2
	})
	if got == nil {
		t.Fatal("did not capture an ICMPv6 packet-too-big on the N6 side")
	}

	if adv := binary.BigEndian.Uint32(got[ethHdrLen+40+4 : ethHdrLen+40+8]); adv != 1280 {
		t.Errorf("advertised MTU = %d, want 1280: below it the tunnel fragments instead", adv)
	}
}

func TestDownlinkS1UDFClearIsHandedOffForOuterFragmentation(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x46524734

	f := setupT2(t, false)
	putDownlinkPDRS1U(t, f.obj, ueIP, teid, testUPFN3IP, testGNBIP)
	shrinkN3MTU(t)

	if err := f.obj.SetFragTxEnabled(true); err != nil {
		t.Fatalf("enable outer fragmentation: %v", err)
	}

	ring := openFragTxReader(t, f.obj)

	inner := ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 4001, bytesOf(1400)))
	f.injectDownlink(t, ethFrame(0x0800, inner))

	pkt := readRingRecord(t, ring)
	if pkt == nil {
		t.Fatal("no S1-U packet was handed off for outer fragmentation")
	}

	if len(pkt) != gtpV4EncapLenS1U+len(inner) {
		t.Fatalf("handed-off packet is %d bytes, want %d (S1-U headers + inner)", len(pkt), gtpV4EncapLenS1U+len(inner))
	}

	if got := binary.BigEndian.Uint32(pkt[32:36]); got != teid {
		t.Errorf("GTP-U TEID = %#x, want %#x", got, teid)
	}

	if !bytes.Equal(pkt[gtpV4EncapLenS1U:], inner) {
		t.Error("the encapsulated inner packet differs from the one sent")
	}
}

func TestReassembledLocalSwitchIsHandedOffForOuterFragmentation(t *testing.T) {
	requireProgTestRun(t)

	const (
		teidA = uint32(0x4C534631)
		teidB = uint32(0x4C534632)
	)

	ueB := [4]byte{10, 45, 0, 2}

	f := setupT2(t, false)

	ulPdr := PdrInfo{
		IMSI:   "001010000000001",
		Far:    FarInfo{Action: 0x02},
		UEIPv4: netip.AddrFrom4(ueIP),
	}
	if err := f.obj.PutPdrUplink(teidA, localSwitched(ulPdr)); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	if err := f.obj.PutPdrDownlink(netip.AddrFrom4(ueB), localSwitched(ipv4OuterDownlinkPDR(teidB, testUPFN3IP, testGNBIP, 5))); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	peer := setupReasmVeth(t, f)
	shrinkN3MTU(t)

	if err := f.obj.SetFragTxEnabled(true); err != nil {
		t.Fatalf("enable outer fragmentation: %v", err)
	}

	ring := openFragTxReader(t, f.obj)

	inner := ipv4Packet(ueIP, ueB, 17, udpDatagram(4000, 4001, bytesOf(1400)))
	inject(t, peer, uplinkGPDU(teidA, inner))

	pkt := readRingRecord(t, ring)
	if pkt == nil {
		t.Fatal("an oversized UE-to-UE packet was not handed off for outer fragmentation")
	}

	if got := binary.BigEndian.Uint32(pkt[32:36]); got != teidB {
		t.Errorf("GTP-U TEID = %#x, want the destination UE's %#x", got, teidB)
	}

	if !bytes.Equal(pkt[gtpV4EncapLen:], inner) {
		t.Error("the encapsulated inner packet differs from the one sent")
	}
}

func lockRouteMTUToGNB(t *testing.T) {
	t.Helper()

	if out, err := ipCmd("route", "replace", ip4String(testGNBIP)+"/32", "dev", t2N3Dev, "mtu", "lock", "1280"); err != nil {
		t.Fatalf("lock the route MTU to the gNB: %v: %s", err, out)
	}
}

func TestDownlinkSmallerRouteMTUIsHandedOffForOuterFragmentation(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x46524760

	f := setupT2(t, false)
	putDownlinkPDR(t, f.obj, ueIP, teid, testUPFN3IP, testGNBIP, 7)
	lockRouteMTUToGNB(t)

	if err := f.obj.SetFragTxEnabled(true); err != nil {
		t.Fatalf("enable outer fragmentation: %v", err)
	}

	ring := openFragTxReader(t, f.obj)
	n3 := f.captureN3(t)

	inner := withDF(ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 4001, bytesOf(1300))))
	f.injectDownlink(t, ethFrame(0x0800, inner))

	pkt := readRingRecord(t, ring)
	if pkt == nil {
		t.Fatal("a packet over the route MTU was not handed off for outer fragmentation")
	}

	if !bytes.Equal(pkt[gtpV4EncapLen:], inner) {
		t.Error("the encapsulated inner packet differs from the one sent")
	}

	if fr := captureMatching(n3, 200*time.Millisecond, func(fr []byte) bool {
		return len(fr) > 1280
	}); fr != nil {
		t.Error("a frame over the route MTU egressed on N3")
	}

	if got := GetDatapathCounters(f.obj)[Downlink].Forwarded[ActionSlowPath]; got != 1 {
		t.Errorf("slow-path forwards = %d, want 1", got)
	}

	if got := DropCount(f.obj, Downlink, "fib_frag_needed"); got != 0 {
		t.Errorf("fib_frag_needed = %d, want 0", got)
	}
}

func TestDownlinkSmallerRouteMTUDroppedWhenOuterFragmentationUnavailable(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x46524761

	f := setupT2(t, false)
	putDownlinkPDR(t, f.obj, ueIP, teid, testUPFN3IP, testGNBIP, 7)
	lockRouteMTUToGNB(t)

	f.injectDownlink(t, ethFrame(0x0800, ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 4001, bytesOf(1300)))))

	deadline := time.Now().Add(time.Second)
	for DropCount(f.obj, Downlink, "fib_frag_needed") != 1 {
		if time.Now().After(deadline) {
			t.Fatalf("fib_frag_needed = %d, want 1", DropCount(f.obj, Downlink, "fib_frag_needed"))
		}

		time.Sleep(20 * time.Millisecond)
	}
}
