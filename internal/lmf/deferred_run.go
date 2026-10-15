// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	deferredWriteTimeout = 5 * time.Second
	maxFixBackoff        = 30 * time.Second
)

var fixLeadTime = map[RequestedMethod]time.Duration{
	RequestedECID: 5 * time.Second,
	RequestedGNSS: 15 * time.Second,
}

type periodicSchedule struct {
	interval time.Duration
	amount   int64
	end      time.Time
	due      time.Time
	count    int64
}

func newPeriodicSchedule(start time.Time, interval time.Duration, amount int64, infinite bool) *periodicSchedule {
	span := infiniteReportSpan
	if !infinite {
		span = time.Duration(amount) * interval
	}

	s := &periodicSchedule{interval: interval, end: start.Add(span), due: start}
	if !infinite {
		s.amount = amount
	}

	return s
}

func (s *periodicSchedule) finished(now time.Time) bool {
	if s.amount > 0 && s.count >= s.amount {
		return true
	}

	return !now.Before(s.end) || !s.due.Before(s.end)
}

func (s *periodicSchedule) reported(reportedAt time.Time, late bool) {
	s.count++

	if late {
		s.due = reportedAt.Add(s.interval)
		return
	}

	s.due = s.due.Add(s.interval)
}

func (l *LMF) runDeferred(ctx context.Context, run *deferredRun) {
	schedule := newPeriodicSchedule(run.start, run.interval, run.amount, run.infinite)
	lead := min(fixLeadTime[run.location.Method], run.interval/2)

	for !schedule.finished(time.Now()) {
		if !sleepUntil(ctx, schedule.due.Add(-lead)) {
			return
		}

		result, late, reason, ok := l.fixUntilSuccess(ctx, run, schedule)
		if ctx.Err() != nil {
			return
		}

		if reason != "" {
			l.endDeferred(run, TerminationByNetwork, reason)
			return
		}

		if !ok {
			break
		}

		reportedAt := time.Now()

		l.storeReport(run, schedule.count+1, reportedAt, result)
		schedule.reported(reportedAt, late)
	}

	if ctx.Err() == nil {
		l.endDeferred(run, TerminationNormal, "")
	}
}

func (l *LMF) fixUntilSuccess(ctx context.Context, run *deferredRun, schedule *periodicSchedule) (*models.LocationResult, bool, TerminationReason, bool) {
	backoff := min(time.Second, run.interval)
	late := false

	for {
		if reason := l.deferredUnavailable(ctx, run); reason != "" {
			return nil, false, reason, false
		}

		fixCtx, cancel := context.WithDeadline(ctx, schedule.end)
		result, err := l.deferredFix(fixCtx, run)

		cancel()

		if err == nil {
			return result, late, "", true
		}

		if ctx.Err() != nil {
			return nil, false, "", false
		}

		logger.LmfLog.Warn("deferred location fix failed; retrying",
			zap.String("id", run.id),
			logger.SUPI(run.supi.String()),
			zap.Duration("retry_in", backoff),
			zap.Error(err),
		)

		late = true

		if !time.Now().Add(backoff).Before(schedule.end) || !sleepUntil(ctx, time.Now().Add(backoff)) {
			return nil, false, "", false
		}

		backoff = min(2*backoff, run.interval, maxFixBackoff)
	}
}

func (l *LMF) deferredFix(ctx context.Context, run *deferredRun) (*models.LocationResult, error) {
	if l.fixOverride != nil {
		return l.fixOverride(ctx, run.supi, run.location)
	}

	maxAge := int32(max(run.interval/time.Second, 1))

	var result *models.LocationResult

	err := l.withUE(ctx, run.supi, func() error {
		var err error

		result, err = l.locate(ctx, run.supi, run.location, min(maxAge, l.locationMaxAge()))

		return err
	})

	return result, err
}

func (l *LMF) deferredUnavailable(ctx context.Context, run *deferredRun) TerminationReason {
	if l.availabilityOverride != nil {
		return l.availabilityOverride(run.supi)
	}

	registeredOnAMF := l.amf != nil && l.amf.IsUERegistered(run.supi)
	registeredOnMME := l.mme != nil && l.mme.IsUERegistered(run.supi)

	switch {
	case run.core == CoreAMF && registeredOnAMF, run.core == CoreMME && registeredOnMME:
		return ""
	case registeredOnAMF || registeredOnMME:
		return TerminationReasonUEMobility
	case l.registeredOnAnotherNode(ctx, run.supi):
		return TerminationReasonUEMobility
	default:
		return TerminationReasonUEDeregistered
	}
}

func (l *LMF) registeredOnAnotherNode(ctx context.Context, supi etsi.SUPI) bool {
	if l.db == nil || !supi.IsIMSI() {
		return false
	}

	for _, regType := range []string{db.UERegistrationTypeAMF3GPPAccess, db.UERegistrationTypeMME} {
		reg, err := l.db.GetUERegistration(ctx, supi.IMSI(), regType)
		if err != nil || reg == nil {
			continue
		}

		if !reg.Purged && reg.NodeID != l.db.RaftID() {
			return true
		}
	}

	return false
}

func (l *LMF) storeReport(run *deferredRun, seq int64, reportedAt time.Time, result *models.LocationResult) {
	ctx, cancel := context.WithTimeout(context.Background(), deferredWriteTimeout)
	defer cancel()

	data, err := json.Marshal(result)
	if err != nil {
		logger.LmfLog.Error("failed to encode location report", zap.String("id", run.id), zap.Error(err))
		return
	}

	report := &db.LocationReport{
		RequestID:    run.id,
		Seq:          seq,
		ReportedAt:   dbwriter.EpochMillis(reportedAt),
		LocationData: string(data),
	}

	if err := l.db.AddLocationReport(ctx, report, db.LocationReportsKept); err != nil {
		logger.LmfLog.Error("failed to store location report", zap.String("id", run.id), zap.Int64("seq", seq), zap.Error(err))
	}
}

func (l *LMF) endDeferred(run *deferredRun, cause TerminationCause, reason TerminationReason) {
	ctx, cancel := context.WithTimeout(context.Background(), deferredWriteTimeout)
	defer cancel()

	termination := db.DeferredLocationTermination{Cause: string(cause), At: time.Now()}

	if reason != "" {
		r := string(reason)
		termination.Reason = &r
	}

	if err := l.db.TerminateDeferredLocationRequest(ctx, run.id, termination); err != nil {
		logger.LmfLog.Warn("failed to record end of deferred location request", zap.String("id", run.id), zap.Error(err))
		return
	}

	logger.LmfLog.Info("deferred location request ended",
		zap.String("id", run.id),
		logger.SUPI(run.supi.String()),
		zap.String("cause", string(cause)),
		zap.String("reason", string(reason)),
	)
}

func sleepUntil(ctx context.Context, t time.Time) bool {
	d := time.Until(t)
	if d <= 0 {
		return ctx.Err() == nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
