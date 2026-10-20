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
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

const (
	usageCollectionInterval = 30 * time.Second
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

func (l *usageLedger) restore(rows []models.SubscriberUsage) {
	for _, r := range rows {
		l.add(usageKey{epochDay: r.EpochDay, imsi: r.IMSI}, r.UplinkVolume, r.DownlinkVolume)
	}
}

func (l *usageLedger) close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.closed = true
}

func epochDay(t time.Time) int64 {
	return t.UTC().Unix() / 86400
}

func (s *SMF) StartUsageCollection() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	s.usageCancel = cancel
	s.usageDone = done

	go func() {
		defer close(done)

		ticker := time.NewTicker(usageCollectionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.collectUsage(ctx)
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

	s.usage.close()
	s.flushUsage(ctx)

	unrecorded := s.usage.take()
	if len(unrecorded) == 0 {
		return
	}

	logger.SmfLog.Error("usage bytes lost: usage could not be recorded before shutdown", usageFields(unrecorded)...)

	for _, u := range unrecorded {
		countDroppedUsage(usageDropShutdown, u.UplinkVolume+u.DownlinkVolume)
	}
}

func (s *SMF) collectUsage(ctx context.Context) {
	for _, sc := range s.pooledSessions() {
		if !sc.Mutex.TryLock() {
			continue
		}

		s.takeUsageLocked(ctx, sc)
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

			s.takeUsageLocked(ctx, sc)
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

func (s *SMF) takeUsageLocked(ctx context.Context, sc *SMContext) {
	if sc.PFCPContext == nil || !sc.PFCPContext.Established {
		return
	}

	report, err := s.upf.QueryUsage(ctx, sc.PFCPContext.SEID)
	if err != nil {
		logger.From(ctx, logger.SmfLog).Warn("could not read every usage counter of the session",
			logger.SEID(sc.PFCPContext.SEID), logger.SUPI(sc.Supi.String()), zap.Error(err))
	}

	s.applyUsage(ctx, sc, report, time.Now())
}

func (s *SMF) applyUsage(ctx context.Context, sc *SMContext, report models.UsageReport, now time.Time) {
	if report.UplinkVolume+report.DownlinkVolume == 0 {
		return
	}

	imsi := sc.Supi.IMSI()
	if imsi == "" {
		logger.From(ctx, logger.SmfLog).Error("usage bytes lost: the session names no IMSI",
			logger.SEID(report.SEID), logger.SUPI(sc.Supi.String()),
			logger.UplinkVolume(report.UplinkVolume), logger.DownlinkVolume(report.DownlinkVolume))

		return
	}

	if !s.usage.add(usageKey{epochDay: epochDay(now), imsi: imsi}, report.UplinkVolume, report.DownlinkVolume) {
		logger.From(ctx, logger.SmfLog).Error("usage bytes lost: usage taken after the final shutdown flush",
			logger.SEID(report.SEID), logger.SUPIFromIMSI(imsi),
			logger.UplinkVolume(report.UplinkVolume), logger.DownlinkVolume(report.DownlinkVolume))
		countDroppedUsage(usageDropShutdown, report.UplinkVolume+report.DownlinkVolume)
	}
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
				countDroppedUsage(usageDropOutcomeUnknown, u.UplinkVolume+u.DownlinkVolume)
			}

			continue
		}

		logger.From(ctx, logger.SmfLog).Warn("could not record usage; retrying on the next flush",
			append(usageFields(batch), zap.Error(err))...)
		s.usage.restore(batch)
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
