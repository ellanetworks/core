// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

var errCounterWentBackwards = errors.New("counter went backwards")

type usageLedger struct {
	mu       sync.Mutex
	counted  map[uint32][]ebpf.N3N6EntrypointUrrCounters
	uplink   uint64
	downlink uint64
	closed   bool
}

func (l *usageLedger) track(urrID uint32) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.counted == nil {
		l.counted = make(map[uint32][]ebpf.N3N6EntrypointUrrCounters)
	}

	if _, ok := l.counted[urrID]; !ok {
		l.counted[urrID] = nil
	}
}

func (l *usageLedger) tracksURRs() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.counted) > 0
}

func (l *usageLedger) reportLocked(session *Session) models.UsageReport {
	report := models.UsageReport{
		SEID:           session.SEID,
		IMSI:           session.IMSI(),
		UplinkVolume:   l.uplink,
		DownlinkVolume: l.downlink,
	}
	l.uplink, l.downlink = 0, 0

	return report
}

func counterGrowth(prev, current uint64) (uint64, bool) {
	if current < prev {
		return current, false
	}

	return current - prev, true
}

func counterDelta(counted, current []ebpf.N3N6EntrypointUrrCounters) (uint64, uint64, error) {
	var (
		uplink, downlink uint64
		err              error
	)

	for cpu, v := range current {
		var prev ebpf.N3N6EntrypointUrrCounters
		if cpu < len(counted) {
			prev = counted[cpu]
		}

		up, upOK := counterGrowth(prev.Uplink, v.Uplink)
		down, downOK := counterGrowth(prev.Downlink, v.Downlink)

		if !upOK || !downOK {
			err = fmt.Errorf("CPU %d: %w", cpu, errCounterWentBackwards)
		}

		uplink += up
		downlink += down
	}

	return uplink, downlink, err
}

func (conn *SessionEngine) countURRLocked(session *Session, urrID uint32) error {
	if conn.BpfObjects == nil {
		return nil
	}

	l := &session.usage

	current, err := conn.BpfObjects.ReadUrr(session.SEID, urrID)
	if err != nil {
		return fmt.Errorf("read URR %d: %w", urrID, err)
	}

	uplink, downlink, err := counterDelta(l.counted[urrID], current)

	l.counted[urrID] = current
	l.uplink += uplink
	l.downlink += downlink

	if err != nil {
		return fmt.Errorf("URR %d: %w", urrID, err)
	}

	return nil
}

func (conn *SessionEngine) TakeUsage(session *Session) (models.UsageReport, error) {
	if !conn.enter() {
		return models.UsageReport{SEID: session.SEID}, ErrClosed
	}
	defer conn.exit()

	l := &session.usage

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return models.UsageReport{SEID: session.SEID}, nil
	}

	var errs []error

	for _, id := range slices.Sorted(maps.Keys(l.counted)) {
		if err := conn.countURRLocked(session, id); err != nil {
			errs = append(errs, err)
		}
	}

	return l.reportLocked(session), errors.Join(errs...)
}

func (conn *SessionEngine) HoldUsage(report models.UsageReport) {
	if report.UplinkVolume+report.DownlinkVolume == 0 {
		return
	}

	conn.heldMu.Lock()
	defer conn.heldMu.Unlock()

	if conn.held == nil {
		conn.held = make(map[string]models.UsageReport)
	}

	held := conn.held[report.IMSI]
	held.SEID = report.SEID
	held.IMSI = report.IMSI
	held.UplinkVolume += report.UplinkVolume
	held.DownlinkVolume += report.DownlinkVolume
	conn.held[report.IMSI] = held
}

func (conn *SessionEngine) TakeHeldUsage() []models.UsageReport {
	conn.heldMu.Lock()
	defer conn.heldMu.Unlock()

	reports := slices.Collect(maps.Values(conn.held))
	conn.held = nil

	return reports
}

func (conn *SessionEngine) closeUsage(ctx context.Context, session *Session) (models.UsageReport, error) {
	l := &session.usage

	l.mu.Lock()
	defer l.mu.Unlock()

	var errs []error

	for _, id := range slices.Sorted(maps.Keys(l.counted)) {
		if err := conn.countURRLocked(session, id); err != nil {
			logger.From(ctx, logger.UpfLog).Error("usage bytes lost: the final URR reading is not exact",
				logger.SEID(session.SEID), logger.URRID(id), zap.Error(err))
		}

		if conn.BpfObjects == nil {
			continue
		}

		if err := conn.BpfObjects.DeleteUrr(session.SEID, id); err != nil {
			errs = append(errs, fmt.Errorf("remove URR %d: %w", id, err))
		}
	}

	l.closed = true
	l.counted = nil

	return l.reportLocked(session), errors.Join(errs...)
}

func (conn *SessionEngine) programWaiter() ebpf.ProgramWaiter {
	if conn.programs != nil {
		return conn.programs
	}

	if conn.BpfObjects != nil && conn.BpfObjects.Programs != nil {
		return conn.BpfObjects.Programs
	}

	return nil
}

func (conn *SessionEngine) waitForPrograms(ctx context.Context, session *Session) {
	programs := conn.programWaiter()
	if programs == nil || !session.usage.tracksURRs() {
		return
	}

	if err := programs.Wait(); err != nil {
		logger.From(ctx, logger.UpfLog).Error("usage bytes lost: could not wait for running datapath programs before the final reading",
			logger.SEID(session.SEID), zap.Error(err))
	}
}
