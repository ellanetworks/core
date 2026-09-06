// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"context"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

func TestStopUsageMonitorWaitsForExit(t *testing.T) {
	u := &UPF{ctx: context.Background()}

	u.startUsageMonitor(context.Background(), time.Hour)

	done := u.usageDone

	select {
	case <-done:
		t.Fatal("usage monitor exited before it was stopped")
	default:
	}

	u.stopUsageMonitor()

	select {
	case <-done:
	default:
		t.Error("stopUsageMonitor returned while the usage monitor was still running")
	}
}

func TestStopUsageMonitorWithoutStart(t *testing.T) {
	u := &UPF{}

	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		u.stopUsageMonitor()
	}()

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("stopUsageMonitor blocked when no monitor had been started")
	}
}

func TestMonitorUsageReturnsWithinTheFlushBudget(t *testing.T) {
	u := &UPF{ctx: context.Background()}

	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)

		u.monitorUsage(time.Hour, stop)
	}()

	close(stop)

	select {
	case <-done:
	case <-time.After(usageFlushTimeout + 5*time.Second):
		t.Fatal("monitorUsage did not return after stop")
	}
}

func pdr(pdrID, urrID uint32, ueIP string) engine.SPDRInfo {
	info := engine.SPDRInfo{
		PdrID:   pdrID,
		PdrInfo: ebpf.PdrInfo{PdrID: pdrID, UrrID: urrID},
	}
	if ueIP != "" {
		info.UEIP = netip.MustParseAddr(ueIP)
	}

	return info
}

func TestSessionURRsDeduplicatesSharedDownlinkURR(t *testing.T) {
	pdrs := map[uint32]engine.SPDRInfo{
		1: pdr(1, 1, ""),
		2: pdr(2, 2, "10.45.0.1"),
		3: pdr(3, 2, "2001:db8::1"),
	}

	got := sessionURRs(pdrs)

	want := []sessionURR{{id: 1, downlink: false}, {id: 2, downlink: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sessionURRs() = %v, want %v", got, want)
	}
}

func TestSessionURRsIsOrderIndependent(t *testing.T) {
	pdrs := map[uint32]engine.SPDRInfo{
		1: pdr(1, 1, ""),
		2: pdr(2, 2, "10.45.0.1"),
		3: pdr(3, 2, "2001:db8::1"),
	}

	first := sessionURRs(pdrs)

	for range 50 {
		if got := sessionURRs(pdrs); !reflect.DeepEqual(got, first) {
			t.Fatalf("sessionURRs() = %v, want the stable %v", got, first)
		}
	}
}

func TestSessionURRsSkipsUnsetURRID(t *testing.T) {
	pdrs := map[uint32]engine.SPDRInfo{
		1: pdr(1, 0, ""),
		2: pdr(2, 2, "10.45.0.1"),
	}

	got := sessionURRs(pdrs)

	want := []sessionURR{{id: 2, downlink: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sessionURRs() = %v, want %v", got, want)
	}
}

func TestSessionURRsWithoutPDRs(t *testing.T) {
	if got := sessionURRs(map[uint32]engine.SPDRInfo{}); len(got) != 0 {
		t.Fatalf("sessionURRs() = %v, want none", got)
	}
}

func TestSessionURRsSharedByBothDirectionsReportsDownlink(t *testing.T) {
	pdrs := map[uint32]engine.SPDRInfo{
		1: pdr(1, 7, ""),
		2: pdr(2, 7, "10.45.0.1"),
	}

	got := sessionURRs(pdrs)

	want := []sessionURR{{id: 7, downlink: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sessionURRs() = %v, want %v", got, want)
	}
}

func usageList(n int) []sessionUsage {
	all := make([]sessionUsage, n)
	for i := range all {
		all[i] = sessionUsage{seid: uint64(i), localSeid: uint64(i)}
	}

	return all
}

func TestUsageChunksCoversEverySessionExactlyOnce(t *testing.T) {
	for _, n := range []int{1, 4, 5, 6, 2000, 2001, 4100} {
		seen := map[uint64]int{}

		for _, chunk := range usageChunks(usageList(n), 5) {
			if len(chunk) > 5 {
				t.Fatalf("n=%d: chunk of %d exceeds the batch size", n, len(chunk))
			}

			for _, u := range chunk {
				seen[u.seid]++
			}
		}

		if len(seen) != n {
			t.Fatalf("n=%d: covered %d sessions, want %d", n, len(seen), n)
		}

		for seid, count := range seen {
			if count != 1 {
				t.Fatalf("n=%d: seid %d appeared %d times, want once", n, seid, count)
			}
		}
	}
}

func TestUsageChunksOfNothingIsNothing(t *testing.T) {
	if got := usageChunks(nil, 5); got != nil {
		t.Fatalf("usageChunks(nil) = %v, want nil", got)
	}
}

func TestUsageChunksWithANonPositiveSizeStaysWhole(t *testing.T) {
	got := usageChunks(usageList(7), 0)

	if len(got) != 1 || len(got[0]) != 7 {
		t.Fatalf("usageChunks(7, 0) produced %d chunks, want one of 7", len(got))
	}
}
