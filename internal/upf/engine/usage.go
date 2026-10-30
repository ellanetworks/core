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
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

var errCounterWentBackwards = errors.New("counter went backwards")

var usageClock = func() (time.Time, uint64) {
	var ts unix.Timespec

	now := time.Now()
	if err := unix.ClockGettime(unix.CLOCK_BOOTTIME, &ts); err != nil {
		return now, 0
	}

	return now, uint64(ts.Nano())
}

type urrMeter struct {
	counted        []ebpf.N3N6EntrypointUrrCounters
	pending        urrDelta
	firstPacketNs  uint64
	lastPacketNs   uint64
	carriedFirstNs uint64
	start          time.Time
	sequence       uint32
}

type usageLedger struct {
	mu     sync.Mutex
	urrs   map[uint32]*urrMeter
	closed bool
}

func (l *usageLedger) track(urrID uint32, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.trackLocked(urrID, now)
}

func (l *usageLedger) trackLocked(urrID uint32, now time.Time) {
	if l.urrs == nil {
		l.urrs = make(map[uint32]*urrMeter)
	}

	if _, ok := l.urrs[urrID]; !ok {
		l.urrs[urrID] = &urrMeter{start: now}
	}
}

func (l *usageLedger) untrack(urrID uint32) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.urrs, urrID)
}

func (l *usageLedger) tracks(urrID uint32) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	_, ok := l.urrs[urrID]

	return ok
}

func (l *usageLedger) tracksURRs() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.urrs) > 0
}

func (l *usageLedger) trackedIDs() []uint32 {
	l.mu.Lock()
	defer l.mu.Unlock()

	return slices.Sorted(maps.Keys(l.urrs))
}

type urrDelta struct {
	uplink, downlink               uint64
	uplinkPackets, downlinkPackets uint64
	earliestLastNs, latestLastNs   uint64
}

func (d urrDelta) packets() uint64 {
	return d.uplinkPackets + d.downlinkPackets
}

func (d *urrDelta) add(o urrDelta) {
	d.uplink += o.uplink
	d.downlink += o.downlink
	d.uplinkPackets += o.uplinkPackets
	d.downlinkPackets += o.downlinkPackets
}

func counterGrowth(prev, current uint64) (uint64, bool) {
	if current < prev {
		return current, false
	}

	return current - prev, true
}

func counterDelta(counted, current []ebpf.N3N6EntrypointUrrCounters) (urrDelta, error) {
	var (
		d   urrDelta
		err error
	)

	for cpu, v := range current {
		var prev ebpf.N3N6EntrypointUrrCounters
		if cpu < len(counted) {
			prev = counted[cpu]
		}

		up, upOK := counterGrowth(prev.Uplink, v.Uplink)
		down, downOK := counterGrowth(prev.Downlink, v.Downlink)
		upPackets, upPacketsOK := counterGrowth(prev.UplinkPackets, v.UplinkPackets)
		downPackets, downPacketsOK := counterGrowth(prev.DownlinkPackets, v.DownlinkPackets)

		if !upOK || !downOK || !upPacketsOK || !downPacketsOK {
			err = fmt.Errorf("CPU %d: %w", cpu, errCounterWentBackwards)
		}

		d.uplink += up
		d.downlink += down
		d.uplinkPackets += upPackets
		d.downlinkPackets += downPackets

		if upPackets+downPackets == 0 || v.LastPacketNs == 0 {
			continue
		}

		if d.earliestLastNs == 0 || v.LastPacketNs < d.earliestLastNs {
			d.earliestLastNs = v.LastPacketNs
		}

		d.latestLastNs = max(d.latestLastNs, v.LastPacketNs)
	}

	return d, err
}

func (m *urrMeter) record(d urrDelta, firstNs uint64) {
	m.pending.add(d)

	if d.packets() == 0 {
		if firstNs != 0 && (m.carriedFirstNs == 0 || firstNs < m.carriedFirstNs) {
			m.carriedFirstNs = firstNs
		}

		return
	}

	if m.carriedFirstNs != 0 && (firstNs == 0 || m.carriedFirstNs < firstNs) {
		firstNs = m.carriedFirstNs
	}

	m.carriedFirstNs = 0

	if firstNs == 0 || firstNs > d.earliestLastNs {
		firstNs = d.earliestLastNs
	}

	if firstNs != 0 && (m.firstPacketNs == 0 || firstNs < m.firstPacketNs) {
		m.firstPacketNs = firstNs
	}

	m.lastPacketNs = max(m.lastPacketNs, d.latestLastNs)
}

func bootToWall(ns uint64, now time.Time, bootNow uint64, start time.Time) time.Time {
	if ns == 0 {
		return time.Time{}
	}

	t := now
	if ns < bootNow {
		t = now.Add(-time.Duration(bootNow - ns))
	}

	if t.Before(start) {
		return start
	}

	return t
}

func (m *urrMeter) report(urrID uint32, trigger models.UsageReportTrigger, now time.Time, bootNow uint64) models.UsageReport {
	end := now
	if end.Before(m.start) {
		end = m.start
	}

	r := models.UsageReport{
		URRID:           urrID,
		SequenceNumber:  m.sequence,
		Trigger:         trigger,
		StartTime:       m.start,
		EndTime:         end,
		FirstPacket:     bootToWall(m.firstPacketNs, end, bootNow, m.start),
		LastPacket:      bootToWall(m.lastPacketNs, end, bootNow, m.start),
		UplinkVolume:    m.pending.uplink,
		DownlinkVolume:  m.pending.downlink,
		UplinkPackets:   m.pending.uplinkPackets,
		DownlinkPackets: m.pending.downlinkPackets,
	}

	m.sequence++
	m.start = end
	m.pending = urrDelta{}
	m.firstPacketNs, m.lastPacketNs = 0, 0

	return r
}

func (conn *SessionEngine) countURRLocked(session *Session, urrID uint32, m *urrMeter) error {
	if conn.BpfObjects == nil {
		return nil
	}

	firstNs, takeErr := conn.BpfObjects.TakeUrrFirstPacket(session.SEID, urrID)

	current, err := conn.BpfObjects.ReadUrr(session.SEID, urrID)
	if err != nil {
		m.record(urrDelta{}, firstNs)
		return errors.Join(fmt.Errorf("read URR %d: %w", urrID, err), takeErr)
	}

	d, deltaErr := counterDelta(m.counted, current)
	m.counted = current
	deltaErr = errors.Join(deltaErr, takeErr)

	m.record(d, firstNs)

	if deltaErr != nil {
		return fmt.Errorf("URR %d: %w", urrID, deltaErr)
	}

	return nil
}

func (conn *SessionEngine) TakeUsage(session *Session, trigger models.UsageReportTrigger) ([]models.UsageReport, error) {
	l := &session.usage

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil, nil
	}

	var (
		errs    []error
		reports []models.UsageReport
	)

	for _, id := range slices.Sorted(maps.Keys(l.urrs)) {
		m := l.urrs[id]

		if err := conn.countURRLocked(session, id, m); err != nil {
			errs = append(errs, err)

			if !errors.Is(err, errCounterWentBackwards) {
				continue
			}
		}

		now, bootNow := usageClock()
		reports = append(reports, m.report(id, trigger, now, bootNow))
	}

	return reports, errors.Join(errs...)
}

func (conn *SessionEngine) finalReportsLocked(ctx context.Context, session *Session, ids []uint32) ([]models.UsageReport, error) {
	l := &session.usage

	var (
		errs    []error
		reports []models.UsageReport
	)

	for _, id := range ids {
		m, ok := l.urrs[id]
		if !ok {
			continue
		}

		if err := conn.countURRLocked(session, id, m); err != nil {
			logger.From(ctx, logger.UpfLog).Error("usage bytes lost: the final URR reading is not exact",
				logger.SEID(session.SEID), logger.URRID(id), zap.Error(err))
		}

		if conn.BpfObjects != nil {
			if err := conn.BpfObjects.DeleteUrr(session.SEID, id); err != nil {
				errs = append(errs, fmt.Errorf("remove URR %d: %w", id, err))
				now, bootNow := usageClock()
				reports = append(reports, m.report(id, models.UsageTriggerTermination, now, bootNow))

				continue
			}
		}

		now, bootNow := usageClock()
		reports = append(reports, m.report(id, models.UsageTriggerTermination, now, bootNow))

		delete(l.urrs, id)
	}

	return reports, errors.Join(errs...)
}

func (conn *SessionEngine) removeURRs(ctx context.Context, session *Session, ids []uint32) ([]models.UsageReport, error) {
	l := &session.usage

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil, nil
	}

	return conn.finalReportsLocked(ctx, session, slices.Sorted(slices.Values(ids)))
}

func (conn *SessionEngine) closeUsage(ctx context.Context, session *Session) ([]models.UsageReport, error) {
	l := &session.usage

	l.mu.Lock()
	defer l.mu.Unlock()

	reports, err := conn.finalReportsLocked(ctx, session, slices.Sorted(maps.Keys(l.urrs)))
	if err != nil {
		return reports, err
	}

	l.closed = true
	l.urrs = nil

	return reports, nil
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

func (conn *SessionEngine) WaitForDatapath() error {
	programs := conn.programWaiter()
	if programs == nil {
		return nil
	}

	return programs.Wait()
}

func (conn *SessionEngine) waitForPrograms(session *Session) error {
	programs := conn.programWaiter()
	if programs == nil || !session.usage.tracksURRs() && len(session.qerMeters) == 0 {
		return nil
	}

	return programs.Wait()
}
