// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"cmp"
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

const (
	usageCollectionInterval = 30 * time.Second
	timeZoneCheckInterval   = time.Minute
	usageFlushBatchSize     = 2000
	usageShutdownRetry      = 10 * time.Millisecond
)

type usageKey struct {
	epochDay int64
	imsi     string
}

type usageVolume struct {
	uplink   uint64
	downlink uint64
}

type usageLedger struct {
	mu      sync.Mutex
	pending map[usageKey]usageVolume
	closed  bool
}

func (l *usageLedger) add(key usageKey, uplink, downlink uint64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return false
	}

	if l.pending == nil {
		l.pending = make(map[usageKey]usageVolume)
	}

	v := l.pending[key]
	v.uplink += uplink
	v.downlink += downlink
	l.pending[key] = v

	return true
}

func (l *usageLedger) take() []models.SubscriberUsage {
	l.mu.Lock()
	pending := l.pending
	l.pending = nil
	l.mu.Unlock()

	keys := slices.SortedFunc(maps.Keys(pending), func(a, b usageKey) int {
		return cmp.Or(cmp.Compare(a.epochDay, b.epochDay), strings.Compare(a.imsi, b.imsi))
	})

	rows := make([]models.SubscriberUsage, 0, len(keys))

	for _, k := range keys {
		v := pending[k]
		rows = append(rows, models.SubscriberUsage{
			EpochDay:       k.epochDay,
			IMSI:           k.imsi,
			UplinkVolume:   v.uplink,
			DownlinkVolume: v.downlink,
		})
	}

	return rows
}

func (l *usageLedger) restore(rows []models.SubscriberUsage) []models.SubscriberUsage {
	var refused []models.SubscriberUsage

	for _, r := range rows {
		if !l.add(usageKey{epochDay: r.EpochDay, imsi: r.IMSI}, r.UplinkVolume, r.DownlinkVolume) {
			refused = append(refused, r)
		}
	}

	return refused
}

func (l *usageLedger) empty() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.pending) == 0
}

func (l *usageLedger) close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.closed = true
}

func epochDay(t time.Time) int64 {
	return t.UTC().Unix() / 86400
}

func nextEpochDay(t time.Time) time.Time {
	return time.Unix((epochDay(t)+1)*86400, 0)
}

type usageConsumer func(ctx context.Context, sc *SMContext, usage []urrUsage)

func (s *SMF) StartUsageCollection() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	s.usageCancel = cancel
	s.usageDone = done

	go func() {
		defer close(done)

		ticker := time.NewTicker(usageCollectionInterval)
		defer ticker.Stop()

		dayEnd := time.NewTimer(time.Until(nextEpochDay(time.Now())))
		defer dayEnd.Stop()

		timeZoneCheck := time.NewTicker(timeZoneCheckInterval)
		defer timeZoneCheck.Stop()

		s.refreshTimeZone(time.Now())

		for {
			select {
			case <-timeZoneCheck.C:
				s.refreshTimeZone(time.Now())
			case <-ticker.C:
				s.collectUsage(ctx)
			case <-dayEnd.C:
				s.collectUsage(ctx)
				dayEnd.Reset(time.Until(nextEpochDay(time.Now())))
			case <-s.usageFlush:
				s.flushUsage(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *SMF) StopUsageCollection(ctx context.Context) {
	if s.usageCancel != nil {
		s.usageCancel()

		select {
		case <-s.usageDone:
		case <-ctx.Done():
		}
	}

	busy := s.collectAllUsage(ctx)
	if busy > 0 {
		logger.SmfLog.Error("usage bytes lost: sessions stayed busy through the final usage collection",
			zap.Int("sessions", busy))
	}

	for {
		s.flushUsage(ctx)

		if s.usage.empty() || ctx.Err() != nil {
			break
		}

		select {
		case <-ctx.Done():
		case <-time.After(usageShutdownRetry):
		}
	}

	s.usage.close()

	unrecorded := s.usage.take()
	if len(unrecorded) == 0 {
		return
	}

	logger.SmfLog.Error("usage bytes lost: usage could not be recorded before shutdown", usageFields(unrecorded)...)

	for _, u := range unrecorded {
		metrics.DroppedUsage(metrics.UsageDropShutdown, u.UplinkVolume+u.DownlinkVolume)
	}
}

func (s *SMF) collectUsage(ctx context.Context) {
	for _, sc := range s.pooledSessions() {
		if !sc.Mutex.TryLock() {
			continue
		}

		s.takeUsageLocked(ctx, sc, models.UsageTriggerPeriodic)
		sc.Mutex.Unlock()
	}

	s.flushUsage(ctx)
}

func (s *SMF) collectAllUsage(ctx context.Context) int {
	remaining := s.pooledSessions()

	for {
		busy := remaining[:0]

		for _, sc := range remaining {
			if !sc.Mutex.TryLock() {
				busy = append(busy, sc)
				continue
			}

			s.takeUsageLocked(ctx, sc, models.UsageTriggerQuery)
			sc.Mutex.Unlock()
		}

		remaining = busy
		if len(remaining) == 0 {
			return 0
		}

		select {
		case <-ctx.Done():
			return len(remaining)
		case <-time.After(usageShutdownRetry):
		}
	}
}

func (s *SMF) pooledSessions() []*SMContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return slices.Collect(maps.Values(s.pool))
}

func (s *SMF) takeUsageLocked(ctx context.Context, sc *SMContext, trigger models.UsageReportTrigger) {
	if sc.PFCPContext == nil || !sc.PFCPContext.Established {
		return
	}

	reports, err := s.upf.QueryUsage(ctx, sc.PFCPContext.SEID, trigger)
	if err != nil {
		logger.From(ctx, logger.SmfLog).Warn("could not read every usage counter of the session",
			logger.SEID(sc.PFCPContext.SEID), logger.SUPI(sc.Supi.String()), zap.Error(err))
	}

	s.deliverUsage(ctx, sc, sc.dataPlane().chargedUsage(reports))
}

func (sc *SMContext) dataPlane() dataPlane {
	if sc.Tunnel == nil {
		return dataPlane{}
	}

	return sc.Tunnel.dataPlane
}

func (s *SMF) deliverUsage(ctx context.Context, sc *SMContext, usage []urrUsage) {
	if len(usage) == 0 {
		return
	}

	for _, consume := range s.usageConsumers {
		consume(ctx, sc, usage)
	}
}

func usageTime(r models.UsageReport) time.Time {
	switch {
	case !r.LastPacket.IsZero():
		return r.LastPacket
	case !r.EndTime.IsZero():
		return r.EndTime
	default:
		return time.Now()
	}
}

func (s *SMF) recordSubscriberUsage(ctx context.Context, sc *SMContext, usage []urrUsage) {
	imsi := sc.Supi.IMSI()

	for _, u := range usage {
		if u.TotalVolume() == 0 {
			continue
		}

		if imsi == "" {
			logger.From(ctx, logger.SmfLog).Error("usage bytes lost: the session names no IMSI",
				logger.SEID(sc.seid()), logger.SUPI(sc.Supi.String()), logger.URRID(u.URRID),
				logger.UplinkVolume(u.UplinkVolume), logger.DownlinkVolume(u.DownlinkVolume))
			metrics.DroppedUsage(metrics.UsageDropNoIMSI, u.TotalVolume())

			continue
		}

		if !s.usage.add(usageKey{epochDay: epochDay(usageTime(u.UsageReport)), imsi: imsi}, u.UplinkVolume, u.DownlinkVolume) {
			logger.From(ctx, logger.SmfLog).Error("usage bytes lost: usage taken after the final shutdown flush",
				logger.SEID(sc.seid()), logger.SUPIFromIMSI(imsi), logger.URRID(u.URRID),
				logger.UplinkVolume(u.UplinkVolume), logger.DownlinkVolume(u.DownlinkVolume))
			metrics.DroppedUsage(metrics.UsageDropShutdown, u.TotalVolume())
		}
	}
}

func (sc *SMContext) seid() uint64 {
	if sc.PFCPContext == nil {
		return 0
	}

	return sc.PFCPContext.SEID
}

func (s *SMF) requestUsageFlush() {
	select {
	case s.usageFlush <- struct{}{}:
	default:
	}
}

func (s *SMF) flushUsage(ctx context.Context) {
	rows := s.usage.take()

	for batch := range slices.Chunk(rows, usageFlushBatchSize) {
		err := s.store.IncrementDailyUsageBatch(ctx, batch)
		if err == nil {
			continue
		}

		if errors.Is(err, models.ErrUsageOutcomeUnknown) {
			logger.From(ctx, logger.SmfLog).Error("usage write outcome unknown: dropping the usage rather than double-counting it",
				append(usageFields(batch), zap.Error(err))...)

			for _, u := range batch {
				metrics.DroppedUsage(metrics.UsageDropOutcomeUnknown, u.UplinkVolume+u.DownlinkVolume)
			}

			continue
		}

		logger.From(ctx, logger.SmfLog).Warn("could not record usage; retrying on the next flush",
			append(usageFields(batch), zap.Error(err))...)

		refused := s.usage.restore(batch)
		if len(refused) == 0 {
			continue
		}

		logger.From(ctx, logger.SmfLog).Error("usage bytes lost: usage could not be recorded before shutdown", usageFields(refused)...)

		for _, u := range refused {
			metrics.DroppedUsage(metrics.UsageDropShutdown, u.UplinkVolume+u.DownlinkVolume)
		}
	}
}

func usageFields(rows []models.SubscriberUsage) []zap.Field {
	var uplink, downlink uint64

	for _, u := range rows {
		uplink += u.UplinkVolume
		downlink += u.DownlinkVolume
	}

	return []zap.Field{
		zap.Int("rows", len(rows)),
		logger.UplinkVolume(uplink),
		logger.DownlinkVolume(downlink),
	}
}
