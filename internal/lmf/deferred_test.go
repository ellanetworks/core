// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/lmf/models"
)

func testDB(t *testing.T) *db.Database {
	t.Helper()

	database, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	return database
}

func deferredTestSUPI(t *testing.T) etsi.SUPI {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatalf("supi: %v", err)
	}

	return supi
}

func registeredLMF(t *testing.T) (*LMF, etsi.SUPI) {
	t.Helper()

	amfInstance := amf.New(nil, nil, nil)
	supi := deferredTestSUPI(t)

	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("add UE: %v", err)
	}

	l := New(amfInstance, nil, testDB(t))
	l.Start(context.Background())

	t.Cleanup(func() { l.Stop(context.Background()) })

	return l, supi
}

func TestPeriodicEventInfoAccept(t *testing.T) {
	ms := int64(500)
	badMs := int64(1000)

	for _, tc := range []struct {
		name    string
		in      PeriodicEventInfo
		want    PeriodicEventInfo
		wantErr bool
	}{
		{"plain", PeriodicEventInfo{ReportingAmount: 10, ReportingInterval: 30}, PeriodicEventInfo{ReportingAmount: 10, ReportingInterval: 30}, false},
		{"maximum duration", PeriodicEventInfo{ReportingAmount: 1, ReportingInterval: 8639999}, PeriodicEventInfo{ReportingAmount: 1, ReportingInterval: 8639999}, false},
		{"duration too long", PeriodicEventInfo{ReportingAmount: 2, ReportingInterval: 8639999}, PeriodicEventInfo{}, true},
		{"zero interval", PeriodicEventInfo{ReportingAmount: 1}, PeriodicEventInfo{}, true},
		{"zero amount", PeriodicEventInfo{ReportingInterval: 1}, PeriodicEventInfo{}, true},
		{"infinite sets the maximum amount", PeriodicEventInfo{ReportingInterval: 60, ReportingInfinite: true}, PeriodicEventInfo{ReportingAmount: 8639999, ReportingInterval: 60, ReportingInfinite: true}, false},
		{"milliseconds are ignored", PeriodicEventInfo{ReportingAmount: 10, ReportingInterval: 60, ReportingIntervalMs: &ms}, PeriodicEventInfo{ReportingAmount: 10, ReportingInterval: 60}, false},
		{"milliseconds do not excuse an invalid interval", PeriodicEventInfo{ReportingAmount: 10, ReportingIntervalMs: &ms}, PeriodicEventInfo{}, true},
		{"milliseconds out of range", PeriodicEventInfo{ReportingAmount: 10, ReportingInterval: 1, ReportingIntervalMs: &badMs}, PeriodicEventInfo{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in

			err := got.accept()
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidPeriodicEventInfo) {
					t.Fatalf("err = %v, want ErrInvalidPeriodicEventInfo", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.ReportingAmount != tc.want.ReportingAmount || got.ReportingInterval != tc.want.ReportingInterval ||
				got.ReportingInfinite != tc.want.ReportingInfinite || got.ReportingIntervalMs != nil {
				t.Fatalf("accepted %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TS 23.273 §4.1a.5.1
func TestPeriodicScheduleLateReportRestartsTheTimer(t *testing.T) {
	start := time.Unix(1000, 0)
	s := newPeriodicSchedule(start, 10*time.Second, 3, false)

	if !s.end.Equal(start.Add(30 * time.Second)) {
		t.Fatalf("end = %v, want start + 30 s", s.end)
	}

	s.reported(start.Add(time.Second))

	if !s.due.Equal(start.Add(10 * time.Second)) {
		t.Fatalf("on-time report: due = %v, want start + 10 s", s.due)
	}

	s.reported(start.Add(14 * time.Second))

	if !s.due.Equal(start.Add(20 * time.Second)) {
		t.Fatalf("report within its slot: due = %v, want start + 20 s", s.due)
	}

	s.reported(start.Add(31 * time.Second))

	if !s.due.Equal(start.Add(41 * time.Second)) {
		t.Fatalf("report after its slot: due = %v, want the report time + 10 s", s.due)
	}

	if !s.finished(start.Add(25 * time.Second)) {
		t.Fatal("not finished after the requested amount")
	}
}

// TS 23.273 §4.1a.5.1
func TestPeriodicScheduleEndsAtTheReportingDuration(t *testing.T) {
	start := time.Unix(1000, 0)
	s := newPeriodicSchedule(start, 10*time.Second, 3, false)

	s.reported(start.Add(25 * time.Second))

	if !s.finished(start.Add(30 * time.Second)) {
		t.Fatal("not finished at amount x interval")
	}

	infinite := newPeriodicSchedule(start, time.Second, 8639999, true)
	if infinite.amount != 0 || !infinite.end.Equal(start.Add(8639999*time.Second)) {
		t.Fatalf("infinite schedule = %+v, want no amount and the maximum duration", infinite)
	}
}

func TestCreateDeferredRequestValidation(t *testing.T) {
	l, supi := registeredLMF(t)
	ctx := context.Background()

	past := time.Now().Add(-time.Minute)

	unknown, err := etsi.NewSUPIFromIMSI("001010000000099")
	if err != nil {
		t.Fatal(err)
	}

	valid := PeriodicEventInfo{ReportingAmount: 2, ReportingInterval: 60}

	for _, tc := range []struct {
		name   string
		params DeferredRequestParams
		want   error
	}{
		{"unregistered UE", DeferredRequestParams{SUPI: unknown, Location: LocationRequest{Method: RequestedCellID}, PeriodicEventInfo: valid}, ErrNotFound},
		{"invalid info", DeferredRequestParams{SUPI: supi, Location: LocationRequest{Method: RequestedCellID}}, ErrInvalidPeriodicEventInfo},
		{"scheduled in the past", DeferredRequestParams{SUPI: supi, Location: LocationRequest{Method: RequestedCellID}, PeriodicEventInfo: valid, ScheduledLocTime: &past}, ErrInvalidScheduledLocTime},
		{"invalid mode", DeferredRequestParams{SUPI: supi, Location: LocationRequest{Method: RequestedGNSS, Mode: models.PositioningModeUEAssisted}, PeriodicEventInfo: valid}, ErrUnsupportedMode},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.CreateDeferredRequest(ctx, tc.params); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}

	future := time.Now().Add(time.Hour)
	params := DeferredRequestParams{SUPI: supi, Location: LocationRequest{Method: RequestedCellID}, PeriodicEventInfo: valid, ScheduledLocTime: &future}

	created, err := l.CreateDeferredRequest(ctx, params)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if created.Status != DeferredStatusActive || created.PeriodicEventInfo.ReportingInterval != 60 {
		t.Fatalf("created = %+v", created)
	}

	if _, err := l.CreateDeferredRequest(ctx, params); !errors.Is(err, ErrDeferredRequestExists) {
		t.Fatalf("second request for the same UE: err = %v, want ErrDeferredRequestExists", err)
	}

	if err := l.DeleteDeferredRequest(ctx, created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := l.CreateDeferredRequest(ctx, params); err != nil {
		t.Fatalf("create right after delete: %v", err)
	}
}

func startTestRun(t *testing.T, l *LMF, supi etsi.SUPI, interval time.Duration, amount int64) string {
	t.Helper()

	created, err := l.CreateDeferredRequest(context.Background(), DeferredRequestParams{
		SUPI:              supi,
		Location:          LocationRequest{Method: RequestedCellID},
		PeriodicEventInfo: PeriodicEventInfo{ReportingAmount: amount, ReportingInterval: 1},
		ScheduledLocTime:  ptrTime(time.Now().Add(time.Hour)),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	l.deferred.mu.Lock()
	run := l.deferred.byID[created.ID]
	l.deferred.mu.Unlock()

	run.cancel()

	waitFor(t, func() bool {
		l.deferred.mu.Lock()
		defer l.deferred.mu.Unlock()

		return l.deferred.byID[created.ID] == nil
	})

	runCtx, cancel := context.WithCancel(l.deferred.ctx)

	fast := &deferredRun{
		id:       created.ID,
		supi:     supi,
		core:     CoreAMF,
		location: LocationRequest{Method: RequestedCellID},
		interval: interval,
		amount:   amount,
		start:    time.Now(),
		cancel:   cancel,
		done:     make(chan struct{}),
	}

	l.deferred.mu.Lock()
	l.deferred.byID[fast.id] = fast
	l.deferred.bySUPI[supi.String()] = fast.id
	l.deferred.mu.Unlock()

	l.deferred.wg.Go(func() {
		defer close(fast.done)
		defer l.forgetRun(fast)

		l.runDeferred(runCtx, fast)
	})

	return created.ID
}

func TestDeferredRunReportsUntilTheAmountIsReached(t *testing.T) {
	l, supi := registeredLMF(t)

	var fixes atomic.Int32

	l.fixOverride = func(context.Context, etsi.SUPI, LocationRequest) (*models.LocationResult, error) {
		fixes.Add(1)
		return &models.LocationResult{SUPI: supi.String()}, nil
	}

	id := startTestRun(t, l, supi, 30*time.Millisecond, 3)

	got := waitForEnd(t, l, id)

	if got.TerminationCause != TerminationNormal || got.TerminationReason != "" || got.EventReportCounter != 3 {
		t.Fatalf("ended request = %+v, want NORMAL_TERMINATION after 3 reports", got)
	}

	reports, err := l.ListLocationReports(context.Background(), id, 0, 10)
	if err != nil {
		t.Fatalf("list reports: %v", err)
	}

	if len(reports) != 3 || reports[0].Seq != 1 || reports[2].Seq != 3 || reports[0].Result.SUPI != supi.String() {
		t.Fatalf("reports = %+v", reports)
	}

	if fixes.Load() != 3 {
		t.Fatalf("fixes = %d, want 3", fixes.Load())
	}
}

// TS 23.273 §4.1a.5.1
func TestDeferredRunRetriesAFailedFixInsteadOfSkippingTheReport(t *testing.T) {
	l, supi := registeredLMF(t)

	var attempts atomic.Int32

	l.fixOverride = func(context.Context, etsi.SUPI, LocationRequest) (*models.LocationResult, error) {
		if attempts.Add(1) <= 2 {
			return nil, ErrRefreshTimeout
		}

		return &models.LocationResult{SUPI: supi.String()}, nil
	}

	id := startTestRun(t, l, supi, 100*time.Millisecond, 10)

	got := waitForEnd(t, l, id)

	if got.TerminationCause != TerminationNormal {
		t.Fatalf("ended request = %+v, want NORMAL_TERMINATION", got)
	}

	if got.EventReportCounter == 0 || got.EventReportCounter >= 10 {
		t.Fatalf("reports = %d, want fewer than 10: retries delay the reports within the fixed duration", got.EventReportCounter)
	}

	if int64(attempts.Load()) != got.EventReportCounter+2 {
		t.Fatalf("attempts = %d, reports = %d: failed attempts must not consume a report", attempts.Load(), got.EventReportCounter)
	}
}

func TestDeferredRunEndsWhenTheUELeaves(t *testing.T) {
	for _, reason := range []TerminationReason{TerminationReasonUEDeregistered, TerminationReasonUEMobility} {
		t.Run(string(reason), func(t *testing.T) {
			l, supi := registeredLMF(t)

			l.fixOverride = func(context.Context, etsi.SUPI, LocationRequest) (*models.LocationResult, error) {
				t.Error("no fix expected for a UE that left")
				return nil, errors.New("unexpected")
			}

			l.availabilityOverride = func(etsi.SUPI) TerminationReason { return reason }

			id := startTestRun(t, l, supi, 30*time.Millisecond, 3)

			got := waitForEnd(t, l, id)

			if got.TerminationCause != TerminationByNetwork || got.TerminationReason != reason || got.EventReportCounter != 0 {
				t.Fatalf("ended request = %+v, want TERMINATION_BY_NETWORK / %s", got, reason)
			}
		})
	}
}

func TestDeleteDeferredRequestStopsTheRun(t *testing.T) {
	l, supi := registeredLMF(t)

	l.fixOverride = func(context.Context, etsi.SUPI, LocationRequest) (*models.LocationResult, error) {
		return &models.LocationResult{SUPI: supi.String()}, nil
	}

	id := startTestRun(t, l, supi, 20*time.Millisecond, 1000)

	waitFor(t, func() bool {
		got, err := l.GetDeferredRequest(context.Background(), id)
		return err == nil && got.EventReportCounter > 0
	})

	if err := l.DeleteDeferredRequest(context.Background(), id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	waitFor(t, func() bool {
		l.deferred.mu.Lock()
		defer l.deferred.mu.Unlock()

		return l.deferred.byID[id] == nil
	})

	if _, err := l.GetDeferredRequest(context.Background(), id); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("get after delete: err = %v, want ErrNotFound", err)
	}

	if err := l.DeleteDeferredRequest(context.Background(), id); !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("second delete: err = %v, want ErrNotFound", err)
	}
}

func TestDeferredRunEndsOnAPermanentPositioningError(t *testing.T) {
	l, supi := registeredLMF(t)

	var attempts atomic.Int32

	l.fixOverride = func(context.Context, etsi.SUPI, LocationRequest) (*models.LocationResult, error) {
		attempts.Add(1)
		return nil, ErrNoLocationEstimate
	}

	id := startTestRun(t, l, supi, 30*time.Millisecond, 100)

	got := waitForEnd(t, l, id)

	if got.TerminationCause != TerminationByNetwork || got.TerminationReason != TerminationReasonPositioning || attempts.Load() != 1 {
		t.Fatalf("ended request = %+v after %d attempts, want positioning_failed after 1", got, attempts.Load())
	}
}

func TestStopAndStartTerminateActiveRequests(t *testing.T) {
	l, supi := registeredLMF(t)
	ctx := context.Background()

	created, err := l.CreateDeferredRequest(ctx, DeferredRequestParams{
		SUPI:              supi,
		Location:          LocationRequest{Method: RequestedCellID},
		PeriodicEventInfo: PeriodicEventInfo{ReportingAmount: 2, ReportingInterval: 60},
		ScheduledLocTime:  ptrTime(time.Now().Add(time.Hour)),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	l.Stop(ctx)

	got, err := l.GetDeferredRequest(ctx, created.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.Status != DeferredStatusTerminated || got.TerminationCause != TerminationByNetwork || got.TerminationReason != TerminationReasonNodeShutdown {
		t.Fatalf("after Stop = %+v, want TERMINATION_BY_NETWORK / node_shutdown", got)
	}

	if _, err := l.CreateDeferredRequest(ctx, DeferredRequestParams{
		SUPI:              supi,
		Location:          LocationRequest{Method: RequestedCellID},
		PeriodicEventInfo: PeriodicEventInfo{ReportingAmount: 2, ReportingInterval: 60},
	}); !errors.Is(err, ErrDeferredNotRunning) {
		t.Fatalf("create after Stop: err = %v, want ErrDeferredNotRunning", err)
	}

	interval, amount := int64(60), int64(2)

	leftover := &db.DeferredLocationRequest{SUPI: supi.String(), LdrType: "PERIODIC", Method: "cell_id", ReportingIntervalS: &interval, ReportingAmount: &amount}
	if err := l.db.CreateDeferredLocationRequest(ctx, leftover); err != nil {
		t.Fatalf("seed: %v", err)
	}

	l.Start(ctx)

	got, err = l.GetDeferredRequest(ctx, leftover.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.TerminationReason != TerminationReasonNodeRestart {
		t.Fatalf("after Start = %+v, want node_restart", got)
	}
}

func TestUELocksSerialisePerUE(t *testing.T) {
	var locks ueLocks

	ctx := context.Background()

	unlock, err := locks.acquire(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}

	other, err := locks.acquire(ctx, "b")
	if err != nil {
		t.Fatalf("a different UE must not wait: %v", err)
	}

	other()

	waitCtx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()

	if _, err := locks.acquire(waitCtx, "a"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second holder of the same UE: err = %v, want DeadlineExceeded", err)
	}

	unlock()

	again, err := locks.acquire(ctx, "a")
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}

	again()

	locks.mu.Lock()
	defer locks.mu.Unlock()

	if len(locks.locks) != 0 {
		t.Fatalf("locks leaked: %d", len(locks.locks))
	}
}

func waitForEnd(t *testing.T, l *LMF, id string) *DeferredRequest {
	t.Helper()

	var got *DeferredRequest

	waitFor(t, func() bool {
		r, err := l.GetDeferredRequest(context.Background(), id)
		if err != nil {
			return false
		}

		got = r

		return r.Status == DeferredStatusTerminated
	})

	return got
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("condition not met within 5 s")
}

func ptrTime(t time.Time) *time.Time { return &t }
