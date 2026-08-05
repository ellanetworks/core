// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"net/netip"
	"sort"
	"testing"
	"time"

	"github.com/cilium/ebpf"
)

// QER rate limiting (utils/qer.h): each accepted packet is charged
// tx_time = size*8/rate against a 5 ms window, so a backlogged sender is
// admitted once every tx_time and receives the configured MBR.

// qerWindow is window_size in utils/qer.h.
const qerWindow = 5 * time.Millisecond

// TestQERUplinkRateLimitDeliversConfiguredRate measures a backlogged sender at
// rates either side of the packet_size*8/window threshold. Wall-clock: median
// of nine admission intervals, ±20%.
func TestQERUplinkRateLimitDeliversConfiguredRate(t *testing.T) {
	requireProgTestRun(t)

	const (
		// The uplink limiter charges the inner packet: ctx->data points past
		// the GTP header when limit_rate_sliding_window is called.
		innerLen = 1250
		samples  = 12
		teid     = 0x51455210
	)

	dst := [4]byte{8, 8, 8, 8}
	frame := uplinkGPDU(teid, innerIPv4UDPSized(dst, 53, innerLen))

	requireUplinkForwards(t, frame, teid)

	tests := []struct {
		name    string
		rateBps uint64
	}{
		// tx_time = 20 ms, above the window.
		{"500 kbit/s", 500_000},
		// tx_time = 2.5 ms, below the window.
		{"4 Mbit/s", 4_000_000},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			txTime := time.Duration(innerLen * 8 * uint64(time.Second) / tc.rateBps)

			obj := loadN3N6Program(t)
			putRateLimitedUplinkPDR(t, obj, teid, tc.rateBps)

			// A backlogged sender: offer the frame as fast as the program can
			// run and record when each packet is admitted.
			accepted, rejected := acceptanceTimes(t, obj.UpfEntryFunc, frame, samples, samples*3*txTime+time.Second)

			// Every rejection must be the limiter's, or the interval below
			// measures something else.
			if got := DropCount(obj, Uplink, "qer_rate_limit"); got != uint64(rejected) {
				t.Fatalf("%d packets were dropped but only %d as qer_rate_limit; another stage is dropping the frame", rejected, got)
			}

			median := medianInterval(accepted)

			delivered := float64(innerLen*8) / median.Seconds()
			ratio := delivered / float64(tc.rateBps)

			if ratio < 0.8 || ratio > 1.2 {
				t.Errorf("delivered %.0f bit/s for a configured %d bit/s (%.0f%%): a backlogged sender is admitted every %v, want tx_time = %v (steady-state interval with the catch-up branch: 2*tx_time - window = %v)",
					delivered, tc.rateBps, ratio*100, median.Round(time.Microsecond), txTime, 2*txTime-qerWindow)
			}
		})
	}
}

// TestQERUplinkRateLimitAdmitsFirstPacket: the window opens at 0, so clamping
// it forward while charging before the eligibility test would reject the first
// packet, never advance the window, and wedge the session permanently.
func TestQERUplinkRateLimitAdmitsFirstPacket(t *testing.T) {
	requireProgTestRun(t)

	const (
		// tx_time = 100 ms, twenty times the window.
		rateBps  = 100_000
		innerLen = 1250
		teid     = 0x51455211
	)

	obj := loadN3N6Program(t)
	putRateLimitedUplinkPDR(t, obj, teid, rateBps)

	frame := uplinkGPDU(teid, innerIPv4UDPSized([4]byte{8, 8, 8, 8}, 53, innerLen))

	if action := runXDP(t, obj.UpfEntryFunc, frame); action == ActionDrop {
		t.Fatalf("first packet of an idle session was dropped, %d as qer_rate_limit: a session that is never charged never opens",
			DropCount(obj, Uplink, "qer_rate_limit"))
	}

	// The second packet arrives far inside the first packet's transmission
	// time, so the limiter must still be holding the line.
	if action := runXDP(t, obj.UpfEntryFunc, frame); action != ActionDrop {
		t.Errorf("second back-to-back packet got XDP action %d, want ActionDrop (%d): the limiter is not enforcing", action, ActionDrop)
	}
}

// putRateLimitedUplinkPDR installs a forwarding uplink PDR whose QER caps the
// uplink at rateBps (0 = unlimited).
func putRateLimitedUplinkPDR(t *testing.T, obj *BpfObjects, teid uint32, rateBps uint64) {
	t.Helper()

	pdr := PdrInfo{
		IMSI:         "001010000000001",
		Far:          FarInfo{Action: 0x02 /* FAR_FORW */},
		Qer:          QerInfo{GateStatusUL: 0 /* GATE_STATUS_OPEN */, MaxBitrateUL: rateBps},
		UEIPv4:       canonicalUEv4,
		UEIPv6Prefix: canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teid, pdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}
}

// requireUplinkForwards fails the test when the frame is not forwarded with the
// limiter disabled: a frame this environment's routing drops would be
// indistinguishable from one the limiter rejected.
func requireUplinkForwards(t *testing.T, frame []byte, teid uint32) {
	t.Helper()

	obj := loadN3N6Program(t)
	putRateLimitedUplinkPDR(t, obj, teid, 0)

	if action := runXDP(t, obj.UpfEntryFunc, frame); action == ActionDrop || action == ActionAborted {
		t.Fatalf("uplink frame got XDP action %d with rate limiting disabled: this environment does not forward it, so the rate measurement below would be meaningless", action)
	}
}

// acceptanceTimes offers frame to prog in a tight loop until n packets are
// admitted, returning the instant of each admission and how many were rejected.
func acceptanceTimes(t *testing.T, prog *ebpf.Program, frame []byte, n int, timeout time.Duration) ([]time.Time, int) {
	t.Helper()

	out := make([]byte, len(frame)+256)
	opts := &ebpf.RunOptions{Data: frame}

	accepted := make([]time.Time, 0, n)
	rejected := 0
	deadline := time.Now().Add(timeout)

	for len(accepted) < n {
		opts.DataOut = out // Run re-slices DataOut to the length produced

		action, err := prog.Run(opts)
		if err != nil {
			t.Fatalf("run XDP program: %v", err)
		}

		now := time.Now()

		if action == ActionDrop {
			rejected++
		} else {
			accepted = append(accepted, now)
		}

		if now.After(deadline) {
			t.Fatalf("only %d of %d packets were admitted within %v", len(accepted), n, timeout)
		}
	}

	return accepted, rejected
}

// medianInterval is the median gap between consecutive admissions. The first
// two are discarded: the window starts empty, so the run opens with a burst
// that is not the steady state.
func medianInterval(accepted []time.Time) time.Duration {
	const skip = 2

	var intervals []time.Duration

	for i := skip + 1; i < len(accepted); i++ {
		intervals = append(intervals, accepted[i].Sub(accepted[i-1]))
	}

	sort.Slice(intervals, func(i, j int) bool { return intervals[i] < intervals[j] })

	return intervals[len(intervals)/2]
}

// TestQERDownlinkWindowSharedAcrossFamilies: an IPv4v6 session gets two
// downlink PDRs referencing one QER, so a window held per PDR gives each family
// the full AMBR.
//
// Deterministic rather than rate-based: tx_time is 100 ms here, so the second
// packet can only be admitted from a separate budget.
func TestQERDownlinkWindowSharedAcrossFamilies(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid     = 0x51455212
		qfi      = 3
		seid     = 0x0BADCAFE
		qerID    = 7
		rateBps  = 100_000
		innerLen = 1250
	)

	obj := loadProgram(t, 1, 0)

	ueIP := [4]byte{10, 45, 0, 2}

	pdr := ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, qfi)
	pdr.SEID = seid
	pdr.QerID = qerID
	pdr.Qer.MaxBitrateDL = rateBps

	if err := obj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
		t.Fatalf("install IPv4 downlink PDR: %v", err)
	}

	if err := obj.PutPdrDownlink(netip.MustParseAddr("2001:db8::"), pdr); err != nil {
		t.Fatalf("install IPv6 downlink PDR: %v", err)
	}

	server4 := [4]byte{8, 8, 8, 8}
	server6 := [16]byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	v4 := ipv4Packet(server4, ueIP, 17, udpDatagram(4000, 53, make([]byte, innerLen-20-8)))
	v6 := ipv6Packet(server6, testUEv6, 17, udpDatagram(4000, 53, make([]byte, innerLen-40-8)))

	if action := runXDP(t, obj.UpfEntryFunc, ethFrame(0x0800, v4)); action == ActionDrop {
		t.Fatalf("first downlink packet was dropped, %d as qer_rate_limit", DropCount(obj, Downlink, "qer_rate_limit"))
	}

	// Same session, other family, immediately after: the budget the IPv4
	// packet just spent is the same budget.
	action := runXDP(t, obj.UpfEntryFunc, ethFrame(0x86DD, v6))

	if got := DropCount(obj, Downlink, "qer_rate_limit"); action != ActionDrop || got != 1 {
		t.Errorf("IPv6 downlink packet got XDP action %d with %d rate-limit drops, want ActionDrop (%d) with 1: the two families are drawing on separate windows, so the session receives twice its downlink AMBR",
			action, got, ActionDrop)
	}
}
