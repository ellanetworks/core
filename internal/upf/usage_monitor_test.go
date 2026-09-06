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
