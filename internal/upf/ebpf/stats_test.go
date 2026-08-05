// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cilium/ebpf"
)

// TestUplinkStatistics checks that uplink byte and action counters accumulate in
// uplink_statistics.
//
// XDP BPF_PROG_TEST_RUN runs with ingress_ifindex == 1, and the entrypoint
// selects the stats map from ingress_ifindex == n3_ifindex/n6_ifindex. So the
// program is loaded with n3_ifindex == 1 (matching the test-run ingress) to
// classify the packets as N3; n6_ifindex == 1 (loopback) serves the in-path MTU
// check.
//
// These assert the map selection and the per-action counters only. The byte
// counter follows the forwarding verdict, which under BPF_PROG_TEST_RUN depends
// on the host's routing table — the reason it used to be recorded before
// routing. TestUplinkByteCounterFollowsVerdict covers it where a frame really
// leaves and the verdict is known.
func TestUplinkStatistics(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid    = 0x57415453
		packets = 5
	)

	obj := loadProgramConfig(t, false, false, 1, 1, 0, 0)
	putForwardingUplinkPDR(t, obj, teid, 0)

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)
	for i := 0; i < packets; i++ {
		runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))
	}

	_, actionsSum := sumStats(t, obj.UplinkStatistics)

	if actionsSum != packets {
		t.Errorf("uplink frames accounted = %d, want %d", actionsSum, packets)
	}

	// Nothing should have been classified as downlink.
	if _, d := sumStats(t, obj.DownlinkStatistics); d != 0 {
		t.Errorf("downlink frames accounted = %d, want 0", d)
	}
}

// TestUplinkStatisticsIPv6 checks that uplink accounting also works for an inner
// IPv6 packet (the byte counter and per-action counter are version-independent).
func TestUplinkStatisticsIPv6(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid    = 0x57415436
		packets = 5
	)

	obj := loadProgramConfig(t, false, false, 1, 1, 0, 0)
	putForwardingUplinkPDR(t, obj, teid, 0)

	inner := innerIPv6UDP(testUEv6, 53)
	for i := 0; i < packets; i++ {
		runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))
	}

	_, actionsSum := sumStats(t, obj.UplinkStatistics)

	if actionsSum != packets {
		t.Errorf("uplink frames accounted = %d, want %d", actionsSum, packets)
	}
}

// Forwarded plus dropped; the two are disjoint.
func sumStats(t *testing.T, m *ebpf.Map) (bytes, frames uint64) {
	t.Helper()

	var stats []N3N6EntrypointUpfStatistic
	if err := m.Lookup(uint32(0), &stats); err != nil {
		t.Fatalf("read statistics map: %v", err)
	}

	for _, s := range stats {
		bytes += s.ByteCounter.Bytes

		for _, a := range s.ForwardedActions {
			frames += a
		}

		for _, d := range s.DropReasons {
			frames += d
		}
	}

	return bytes, frames
}

// The invariant that makes app_upf_datapath_drop_total comparable against
// app_upf_datapath_forward_total.
func TestEveryFrameIsAccountedExactlyOnce(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x41434354

	obj := loadProgramConfig(t, false, false, 1, 1, 0, 0)

	// One forwarded flow and one dropped, so both families are exercised.
	putForwardingUplinkPDR(t, obj, teid, 0)

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)

	const (
		forwarded = 3
		unknown   = 2 // no session for this TEID
	)

	for i := 0; i < forwarded; i++ {
		runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))
	}

	for i := 0; i < unknown; i++ {
		runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid+1, inner))
	}

	_, frames := sumStats(t, obj.UplinkStatistics)
	if want := uint64(forwarded + unknown); frames != want {
		t.Errorf("accounted %d frames, want %d: every frame must be counted once, in exactly one family", frames, want)
	}

	if got := DropCount(obj, Uplink, "unspecified"); got != 0 {
		t.Errorf("%d frames were dropped without a recorded reason; every drop site must use drop_with()/abort_with()", got)
	}
}

// TestUplinkByteCounterFollowsVerdict: the exported throughput counter counts
// what left, not what arrived. A packet the datapath drops must not inflate it —
// the URR charge has always worked this way, and the two disagreeing meant
// operator-facing throughput overstated forwarded traffic.
func TestUplinkByteCounterFollowsVerdict(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulTEID = 0x42433031
		ueSP   = 1400
		srvDP  = 80
	)

	f := setupT2(t, false)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})

	capFD := f.captureN6(t)

	before, _ := sumStats(t, f.obj.UplinkStatistics)

	// Forwarded: routable destination with a resolved neighbour.
	inner := ipv4Packet(ueIP, serverIP, 6,
		tcpSegmentChecksummed(ueIP, serverIP, ueSP, srvDP, bytesOf(40)))
	f.injectUplink(t, uplinkGPDU(ulTEID, inner))

	if captureMatching(capFD, time.Second, func(fr []byte) bool {
		return isInnerIPv4(fr, 6, serverIP)
	}) == nil {
		t.Fatal("uplink did not egress on N6")
	}

	forwarded, _ := sumStats(t, f.obj.UplinkStatistics)
	if want := before + uint64(ethHdrLen+len(inner)); forwarded != want {
		t.Errorf("byte_counter after a forwarded frame = %d, want %d", forwarded, want)
	}

	// Dropped: spoofed source, refused by the anti-spoof check.
	spoofed := ipv4Packet([4]byte{203, 0, 113, 9}, serverIP, 6,
		tcpSegmentChecksummed([4]byte{203, 0, 113, 9}, serverIP, ueSP, srvDP, bytesOf(40)))
	f.injectUplink(t, uplinkGPDU(ulTEID, spoofed))

	time.Sleep(150 * time.Millisecond)

	if dropped, _ := sumStats(t, f.obj.UplinkStatistics); dropped != forwarded {
		t.Errorf("byte_counter moved to %d for a dropped frame, want it to stay at %d",
			dropped, forwarded)
	}
}
