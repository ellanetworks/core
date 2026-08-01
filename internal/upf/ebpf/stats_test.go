// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"testing"

	"github.com/cilium/ebpf"
)

// TestUplinkStatistics checks that uplink byte and action counters accumulate in
// uplink_statistics.
//
// XDP BPF_PROG_TEST_RUN runs with ingress_ifindex == 1, and the entrypoint
// selects the stats map from ingress_ifindex == n3_ifindex/n6_ifindex. So the
// program is loaded with n3_ifindex == 1 (matching the test-run ingress) to
// classify the packets as N3; n6_ifindex == 1 (loopback) serves the in-path MTU
// check. The byte counter is recorded before routing, so the count is
// deterministic regardless of the (host-dependent) forwarding verdict.
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

	bytesSum, actionsSum := sumStats(t, obj.UplinkStatistics)

	if want := uint64(packets * (ethHdrLen + len(inner))); bytesSum != want {
		t.Errorf("uplink byte_counter = %d, want %d", bytesSum, want)
	}

	if actionsSum != packets {
		t.Errorf("uplink frames accounted = %d, want %d", actionsSum, packets)
	}

	// Nothing should have been classified as downlink.
	if d, _ := sumStats(t, obj.DownlinkStatistics); d != 0 {
		t.Errorf("downlink byte_counter = %d, want 0", d)
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

	bytesSum, actionsSum := sumStats(t, obj.UplinkStatistics)

	if want := uint64(packets * (ethHdrLen + len(inner))); bytesSum != want {
		t.Errorf("uplink byte_counter = %d, want %d", bytesSum, want)
	}

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
