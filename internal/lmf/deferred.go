// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	maxReportingValue  = 8639999
	infiniteReportSpan = maxReportingValue * time.Second
)

type DeferredStatus string

const (
	DeferredStatusActive     DeferredStatus = db.DeferredLocationRequestActive
	DeferredStatusTerminated DeferredStatus = db.DeferredLocationRequestTerminated
)

type TerminationCause string

const (
	TerminationNormal    TerminationCause = "NORMAL_TERMINATION"
	TerminationByUE      TerminationCause = "TERMINATION_BY_UE"
	TerminationByNetwork TerminationCause = "TERMINATION_BY_NETWORK"
)

type TerminationReason string

const (
	TerminationReasonUEDeregistered TerminationReason = "ue_deregistered"
	TerminationReasonUEMobility     TerminationReason = "ue_mobility"
	TerminationReasonPositioning    TerminationReason = "positioning_failed"
	TerminationReasonNodeRestart    TerminationReason = "node_restart"
	TerminationReasonNodeShutdown   TerminationReason = "node_shutdown"
)

var (
	ErrInvalidPeriodicEventInfo = errors.New("invalid periodic event info")
	ErrInvalidScheduledLocTime  = errors.New("scheduled location time must be in the future")
	ErrDeferredRequestExists    = errors.New("an active deferred location request already exists for this UE")
	ErrDeferredNotRunning       = errors.New("deferred location requests are not running")
)

type PeriodicEventInfo struct {
	ReportingAmount     int64
	ReportingInterval   int64
	ReportingInfinite   bool
	ReportingIntervalMs *int64
}

type DeferredRequestParams struct {
	SUPI              etsi.SUPI
	Location          LocationRequest
	PeriodicEventInfo PeriodicEventInfo
	ScheduledLocTime  *time.Time
}

const LdrTypePeriodic = "PERIODIC"

type DeferredRequest struct {
	ID                 string
	SUPI               string
	LdrType            string
	Location           LocationRequest
	PeriodicEventInfo  PeriodicEventInfo
	ScheduledLocTime   *time.Time
	Status             DeferredStatus
	TerminationCause   TerminationCause
	TerminationReason  TerminationReason
	EventReportCounter int64
	CreatedAt          time.Time
	TerminatedAt       *time.Time
}

type LocationReport struct {
	Seq        int64
	ReportedAt time.Time
	Result     *models.LocationResult
}

func (p *PeriodicEventInfo) accept() error {
	if p.ReportingIntervalMs != nil && (*p.ReportingIntervalMs < 1 || *p.ReportingIntervalMs > 999) {
		return fmt.Errorf("%w: reporting_interval_ms must be 1..999", ErrInvalidPeriodicEventInfo)
	}

	p.ReportingIntervalMs = nil

	if p.ReportingInterval < 1 || p.ReportingInterval > maxReportingValue {
		return fmt.Errorf("%w: reporting_interval must be 1..%d", ErrInvalidPeriodicEventInfo, maxReportingValue)
	}

	if p.ReportingInfinite {
		p.ReportingAmount = maxReportingValue

		return nil
	}

	if p.ReportingAmount < 1 || p.ReportingAmount > maxReportingValue {
		return fmt.Errorf("%w: reporting_amount must be 1..%d", ErrInvalidPeriodicEventInfo, maxReportingValue)
	}

	if p.ReportingAmount*p.ReportingInterval > maxReportingValue {
		return fmt.Errorf("%w: reporting_amount x reporting_interval must not exceed %d", ErrInvalidPeriodicEventInfo, maxReportingValue)
	}

	return nil
}

type deferredRuns struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	byID   map[string]*deferredRun
	bySUPI map[string]string
}

type deferredRun struct {
	id       string
	supi     etsi.SUPI
	core     Core
	location LocationRequest
	interval time.Duration
	amount   int64
	infinite bool
	start    time.Time
	cancel   context.CancelFunc
	done     chan struct{}
}

func (l *LMF) Start(ctx context.Context) {
	l.terminateAllDeferred(ctx, TerminationReasonNodeRestart)

	l.deferred.start(ctx)
}

func (d *deferredRuns) start(parent context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	d.cancel = cancel
	d.ctx = ctx
	d.byID = make(map[string]*deferredRun)
	d.bySUPI = make(map[string]string)
}

func (d *deferredRuns) stop() bool {
	d.mu.Lock()
	cancel := d.cancel
	d.cancel = nil
	d.ctx = nil
	d.mu.Unlock()

	if cancel == nil {
		return false
	}

	cancel()

	return true
}

func (r *deferredRun) stop() {
	r.cancel()
}

func (r *deferredRun) bind(parent context.Context) context.Context {
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel

	return ctx
}

func (l *LMF) Stop(ctx context.Context) {
	if !l.deferred.stop() {
		return
	}

	done := make(chan struct{})

	go func() {
		l.deferred.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		logger.LmfLog.Warn("deferred location requests did not stop in time")
	}

	sweepCtx, cancelSweep := context.WithTimeout(context.WithoutCancel(ctx), deferredWriteTimeout)
	defer cancelSweep()

	l.terminateAllDeferred(sweepCtx, TerminationReasonNodeShutdown)
}

func (l *LMF) terminateAllDeferred(ctx context.Context, reason TerminationReason) {
	r := string(reason)

	n, err := l.db.TerminateActiveDeferredLocationRequests(ctx, db.DeferredLocationTermination{
		Cause:  string(TerminationByNetwork),
		Reason: &r,
		At:     time.Now(),
	})
	if err != nil {
		logger.LmfLog.Error("failed to terminate deferred location requests", zap.String("reason", r), zap.Error(err))
		return
	}

	if n > 0 {
		logger.LmfLog.Info("terminated deferred location requests", zap.Int("count", n), zap.String("reason", r))
	}
}

func (l *LMF) CreateDeferredRequest(ctx context.Context, params DeferredRequestParams) (*DeferredRequest, error) {
	if err := params.Location.Validate(); err != nil {
		return nil, err
	}

	info := params.PeriodicEventInfo
	if err := info.accept(); err != nil {
		return nil, err
	}

	if params.ScheduledLocTime != nil && !params.ScheduledLocTime.After(time.Now()) {
		return nil, ErrInvalidScheduledLocTime
	}

	if !l.isUERegistered(params.SUPI) {
		return nil, fmt.Errorf("UE not registered: %w", ErrNotFound)
	}

	row, err := toDeferredRow(params, info)
	if err != nil {
		return nil, err
	}

	l.deferred.mu.Lock()
	defer l.deferred.mu.Unlock()

	if l.deferred.ctx == nil {
		return nil, ErrDeferredNotRunning
	}

	if _, ok := l.deferred.bySUPI[params.SUPI.String()]; ok {
		return nil, ErrDeferredRequestExists
	}

	if err := l.db.CreateDeferredLocationRequest(ctx, row); err != nil {
		if errors.Is(err, db.ErrAlreadyExists) {
			return nil, ErrDeferredRequestExists
		}

		return nil, fmt.Errorf("store deferred location request: %w", err)
	}

	start := time.UnixMilli(row.CreatedAt)
	if params.ScheduledLocTime != nil {
		start = *params.ScheduledLocTime
	}

	run := &deferredRun{
		id:       row.ID,
		supi:     params.SUPI,
		core:     l.servingCore(params.SUPI),
		location: params.Location,
		interval: time.Duration(info.ReportingInterval) * time.Second,
		amount:   info.ReportingAmount,
		infinite: info.ReportingInfinite,
		start:    start,
		done:     make(chan struct{}),
	}

	runCtx := run.bind(l.deferred.ctx)

	l.deferred.byID[run.id] = run
	l.deferred.bySUPI[params.SUPI.String()] = run.id

	l.deferred.wg.Go(func() {
		defer close(run.done)
		defer l.forgetRun(run)

		l.runDeferred(runCtx, run)
	})

	logger.LmfLog.Info("deferred location request activated",
		zap.String("id", run.id),
		logger.SUPI(params.SUPI.String()),
		zap.String("method", string(params.Location.Method)),
		zap.Int64("reporting_interval_s", info.ReportingInterval),
		zap.Int64("reporting_amount", info.ReportingAmount),
		zap.Bool("reporting_infinite", info.ReportingInfinite),
	)

	return fromDeferredRow(row)
}

func (l *LMF) forgetRun(run *deferredRun) {
	run.stop()

	l.deferred.mu.Lock()
	defer l.deferred.mu.Unlock()

	if l.deferred.byID[run.id] == run {
		delete(l.deferred.byID, run.id)
		delete(l.deferred.bySUPI, run.supi.String())
	}
}

func (l *LMF) GetDeferredRequest(ctx context.Context, id string) (*DeferredRequest, error) {
	row, err := l.db.GetDeferredLocationRequest(ctx, id)
	if err != nil {
		return nil, err
	}

	return fromDeferredRow(row)
}

func (l *LMF) ListDeferredRequests(ctx context.Context, page, perPage int, supi *string, status *DeferredStatus) ([]DeferredRequest, int, error) {
	filters := &db.DeferredLocationRequestFilters{SUPI: supi}

	if status != nil {
		s := string(*status)
		filters.Status = &s
	}

	rows, total, err := l.db.ListDeferredLocationRequests(ctx, page, perPage, filters)
	if err != nil {
		return nil, 0, err
	}

	out := make([]DeferredRequest, 0, len(rows))

	for i := range rows {
		r, err := fromDeferredRow(&rows[i])
		if err != nil {
			return nil, 0, err
		}

		out = append(out, *r)
	}

	return out, total, nil
}

func (l *LMF) DeleteDeferredRequest(ctx context.Context, id string) error {
	l.deferred.mu.Lock()
	run := l.deferred.byID[id]
	l.deferred.mu.Unlock()

	if run != nil {
		run.stop()

		select {
		case <-run.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return l.db.DeleteDeferredLocationRequest(ctx, id)
}

func (l *LMF) ListLocationReports(ctx context.Context, id string, afterSeq int64, limit int) ([]LocationReport, error) {
	if _, err := l.db.GetDeferredLocationRequest(ctx, id); err != nil {
		return nil, err
	}

	rows, err := l.db.ListLocationReports(ctx, id, afterSeq, limit)
	if err != nil {
		return nil, err
	}

	out := make([]LocationReport, 0, len(rows))

	for _, row := range rows {
		var result models.LocationResult
		if err := json.Unmarshal([]byte(row.LocationData), &result); err != nil {
			return nil, fmt.Errorf("decode location report %d: %w", row.Seq, err)
		}

		out = append(out, LocationReport{Seq: row.Seq, ReportedAt: dbwriter.FromEpochMillis(row.ReportedAt), Result: &result})
	}

	return out, nil
}

func toDeferredRow(params DeferredRequestParams, info PeriodicEventInfo) (*db.DeferredLocationRequest, error) {
	row := &db.DeferredLocationRequest{
		SUPI:               params.SUPI.String(),
		LdrType:            LdrTypePeriodic,
		Method:             string(params.Location.Method),
		ReportingIntervalS: &info.ReportingInterval,
		ReportingAmount:    &info.ReportingAmount,
		ReportingInfinite:  info.ReportingInfinite,
	}

	if params.Location.Mode != "" {
		mode := string(params.Location.Mode)
		row.Mode = &mode
	}

	if params.Location.MeasurementQuantities != nil {
		b, err := json.Marshal(params.Location.MeasurementQuantities)
		if err != nil {
			return nil, fmt.Errorf("encode measurement quantities: %w", err)
		}

		q := string(b)
		row.MeasurementQuantities = &q
	}

	if params.ScheduledLocTime != nil {
		t := dbwriter.EpochMillis(*params.ScheduledLocTime)
		row.ScheduledLocTime = &t
	}

	return row, nil
}

func fromDeferredRow(row *db.DeferredLocationRequest) (*DeferredRequest, error) {
	r := &DeferredRequest{
		ID:      row.ID,
		SUPI:    row.SUPI,
		LdrType: row.LdrType,
		Location: LocationRequest{
			Method: RequestedMethod(row.Method),
		},
		PeriodicEventInfo: PeriodicEventInfo{
			ReportingInfinite: row.ReportingInfinite,
		},
		Status:             DeferredStatus(row.Status),
		EventReportCounter: row.LastReportSeq,
		CreatedAt:          dbwriter.FromEpochMillis(row.CreatedAt),
	}

	if row.ReportingAmount != nil {
		r.PeriodicEventInfo.ReportingAmount = *row.ReportingAmount
	}

	if row.ReportingIntervalS != nil {
		r.PeriodicEventInfo.ReportingInterval = *row.ReportingIntervalS
	}

	if row.Mode != nil {
		r.Location.Mode = models.PositioningMode(*row.Mode)
	}

	if row.MeasurementQuantities != nil {
		var q models.MeasurementQuantities
		if err := json.Unmarshal([]byte(*row.MeasurementQuantities), &q); err != nil {
			return nil, fmt.Errorf("decode measurement quantities: %w", err)
		}

		r.Location.MeasurementQuantities = &q
	}

	if row.ScheduledLocTime != nil {
		t := dbwriter.FromEpochMillis(*row.ScheduledLocTime)
		r.ScheduledLocTime = &t
	}

	if row.TerminationCause != nil {
		r.TerminationCause = TerminationCause(*row.TerminationCause)
	}

	if row.TerminationReason != nil {
		r.TerminationReason = TerminationReason(*row.TerminationReason)
	}

	if row.TerminatedAt != nil {
		t := dbwriter.FromEpochMillis(*row.TerminatedAt)
		r.TerminatedAt = &t
	}

	return r, nil
}
