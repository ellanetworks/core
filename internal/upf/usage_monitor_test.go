// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
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

type stubReportHandler struct {
	err     error
	calls   int
	reports []models.UsageReport
}

func (s *stubReportHandler) HandleDownlinkDataReport(context.Context, *models.DownlinkDataReport) error {
	return nil
}

func (s *stubReportHandler) HandleErrorIndicationReport(context.Context, *models.ErrorIndicationReport) error {
	return nil
}

func (s *stubReportHandler) HandleUsageReports(_ context.Context, reports []*models.UsageReport) error {
	s.calls++

	if s.err != nil {
		return s.err
	}

	for _, report := range reports {
		s.reports = append(s.reports, *report)
	}

	return nil
}

func (s *stubReportHandler) SendFlowReports(context.Context, []*models.FlowReportRequest) error {
	return nil
}

func newUsageTestEngine(t *testing.T) *engine.SessionEngine {
	t.Helper()

	se, err := engine.NewSessionEngine("1.2.3.4", "nodeId", netip.MustParseAddr("2.3.4.5"), netip.Addr{}, netip.MustParseAddr("2.3.4.5"), netip.Addr{}, nil, nil)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	se.AddSession(1, engine.NewSession(1))

	return se
}

func TestReportUsageDropsUsageWhenTheOutcomeIsUnknown(t *testing.T) {
	smf := &stubReportHandler{err: fmt.Errorf("propose: %w", models.ErrUsageOutcomeUnknown)}
	se := newUsageTestEngine(t)

	u := &UPF{se: se, smf: smf}

	u.reportUsage(context.Background(), []models.UsageReport{{SEID: 1, IMSI: "001010000000001", UplinkVolume: 500}})

	if smf.calls != 1 {
		t.Fatalf("the batch was reported %d times, want once", smf.calls)
	}

	if held := se.TakeHeldUsage(); len(held) != 0 {
		t.Fatalf("usage with an unknown outcome was kept for another report: %+v", held)
	}
}

func TestFailedReportIsRetriedByTheNextPoll(t *testing.T) {
	smf := &stubReportHandler{err: errors.New("store unavailable")}
	se := newUsageTestEngine(t)

	u := &UPF{se: se, smf: smf}

	u.reportUsage(context.Background(), []models.UsageReport{
		{SEID: 1, IMSI: "001010000000001", UplinkVolume: 500, DownlinkVolume: 300},
		{SEID: 2, IMSI: "001010000000002", UplinkVolume: 70},
	})

	smf.err = nil

	if err := u.pollUsage(context.Background()); err != nil {
		t.Fatalf("pollUsage: %v", err)
	}

	got := make(map[string]models.UsageReport)
	for _, report := range smf.reports {
		got[report.IMSI] = report
	}

	if r := got["001010000000001"]; r.UplinkVolume != 500 || r.DownlinkVolume != 300 {
		t.Errorf("live session's usage = %+v, want 500/300", r)
	}

	if r := got["001010000000002"]; r.UplinkVolume != 70 {
		t.Errorf("deleted session's usage = %+v, want 70 uplink", r)
	}

	if held := se.TakeHeldUsage(); len(held) != 0 {
		t.Fatalf("recorded usage still held: %+v", held)
	}
}

func TestShutdownFlushReportsHeldUsage(t *testing.T) {
	smf := &stubReportHandler{}
	se := newUsageTestEngine(t)

	se.HoldUsage(models.UsageReport{SEID: 2, IMSI: "001010000000002", DownlinkVolume: 40})

	u := &UPF{se: se, smf: smf}

	u.flushUsageAtShutdown()

	if len(smf.reports) != 1 || smf.reports[0].DownlinkVolume != 40 {
		t.Fatalf("shutdown flush reported %+v, want the held 40 downlink bytes", smf.reports)
	}
}

func TestReportFinalUsageSkipsEmptyUsage(t *testing.T) {
	smf := &stubReportHandler{}
	u := &UPF{se: &engine.SessionEngine{}, smf: smf}

	u.ReportFinalUsage(context.Background(), models.UsageReport{SEID: 1})

	if smf.calls != 0 {
		t.Fatalf("empty final usage was reported %d times", smf.calls)
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
	fields := batchFields([]models.UsageReport{
		{SEID: 7, UplinkVolume: 500, DownlinkVolume: 300},
		{SEID: 8, UplinkVolume: 70, DownlinkVolume: 20},
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
	fields := batchFields([]models.UsageReport{{SEID: 42, UplinkVolume: 500, DownlinkVolume: 300}})

	seid, ok := fieldByKey(fields, "seid")
	if !ok {
		t.Fatalf("a single-session flush lost its SEID: %v", fields)
	}

	if seid.Integer != 42 {
		t.Errorf("seid = %v, want 42", seid.Integer)
	}
}
