// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"context"
	"fmt"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
	"go.uber.org/zap"
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

type stubReportHandler struct {
	err   error
	calls int
}

func (s *stubReportHandler) HandleDownlinkDataReport(context.Context, *models.DownlinkDataReport) error {
	return nil
}

func (s *stubReportHandler) HandleUsageReports(context.Context, []*models.UsageReport) error {
	s.calls++

	return s.err
}

func (s *stubReportHandler) SendFlowReports(context.Context, []*models.FlowReportRequest) error {
	return nil
}

func TestReportUsageKeepsDrainedCountersWhenTheOutcomeIsUnknown(t *testing.T) {
	smf := &stubReportHandler{err: fmt.Errorf("propose: %w", models.ErrUsageOutcomeUnknown)}
	u := &UPF{se: &engine.SessionEngine{}, smf: smf}

	batch := []sessionUsage{
		{seid: 1, localSeid: 1, uvol: 500, drained: map[uint32]uint64{1: 500}},
		{seid: 2, localSeid: 2, dvol: 300, drained: map[uint32]uint64{2: 300}},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("reportUsage reached the URR restore after an unknown outcome: %v", r)
		}
	}()

	u.reportUsage(context.Background(), batch)

	if smf.calls != 1 {
		t.Fatalf("the batch was reported %d times, want once", smf.calls)
	}
}

func fieldByKey(fields []zap.Field, key string) (zap.Field, bool) {
	for _, f := range fields {
		if f.Key == key {
			return f, true
		}
	}

	return zap.Field{}, false
}

func TestBatchFieldsSumsVolumeAcrossTheBatch(t *testing.T) {
	fields := batchFields([]sessionUsage{
		{localSeid: 7, uvol: 500, dvol: 300},
		{localSeid: 8, uvol: 70, dvol: 20},
	})

	uvol, ok := fieldByKey(fields, "uplink_volume")
	if !ok || uvol.Integer != 570 {
		t.Errorf("uplink_volume = %v, want 570", uvol.Integer)
	}

	dvol, ok := fieldByKey(fields, "downlink_volume")
	if !ok || dvol.Integer != 320 {
		t.Errorf("downlink_volume = %v, want 320", dvol.Integer)
	}

	if _, ok := fieldByKey(fields, "seid"); ok {
		t.Error("a multi-session batch must not claim a single SEID")
	}
}

func TestBatchFieldsNamesTheSessionOfASingleSessionFlush(t *testing.T) {
	fields := batchFields([]sessionUsage{{localSeid: 42, uvol: 500, dvol: 300}})

	seid, ok := fieldByKey(fields, "seid")
	if !ok {
		t.Fatalf("a single-session flush lost its SEID: %v", fields)
	}

	if seid.Integer != 42 {
		t.Errorf("seid = %v, want 42", seid.Integer)
	}
}
