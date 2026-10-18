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
	"go.uber.org/zap"
)

type ProgramWaiter interface {
	Wait() error
	Close() error
}

type usageLedger struct {
	mu       sync.Mutex
	created  map[uint32]struct{}
	counted  map[uint32][]uint64
	uplink   uint64
	downlink uint64
	closed   bool
}

func (l *usageLedger) track(urrID uint32) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.created == nil {
		l.created = make(map[uint32]struct{})
	}

	l.created[urrID] = struct{}{}
}

func (l *usageLedger) urrIDsLocked(directions map[uint32]bool) []uint32 {
	ids := make(map[uint32]struct{}, len(l.created)+len(directions))

	for id := range l.created {
		ids[id] = struct{}{}
	}

	for id := range directions {
		ids[id] = struct{}{}
	}

	return slices.Sorted(maps.Keys(ids))
}

func (l *usageLedger) reportLocked(seid uint64) models.UsageReport {
	report := models.UsageReport{SEID: seid, UplinkVolume: l.uplink, DownlinkVolume: l.downlink}
	l.uplink, l.downlink = 0, 0

	return report
}

func urrDirections(pdrs map[uint32]SPDRInfo) map[uint32]bool {
	downlink := make(map[uint32]bool, len(pdrs))

	for _, pdr := range pdrs {
		id := pdr.PdrInfo.UrrID
		if id == 0 {
			continue
		}

		downlink[id] = downlink[id] || pdr.UEIP.IsValid()
	}

	return downlink
}

func counterDelta(counted, current []uint64) uint64 {
	var delta uint64

	for cpu, v := range current {
		var prev uint64
		if cpu < len(counted) {
			prev = counted[cpu]
		}

		if v < prev {
			delta += v
			continue
		}

		delta += v - prev
	}

	return delta
}

func (conn *SessionEngine) countURRLocked(session *Session, urrID uint32, downlink bool) error {
	if conn.BpfObjects == nil {
		return nil
	}

	l := &session.usage

	current, err := conn.BpfObjects.ReadUrr(session.SEID, urrID)
	if err != nil {
		return fmt.Errorf("read URR %d: %w", urrID, err)
	}

	delta := counterDelta(l.counted[urrID], current)

	if l.counted == nil {
		l.counted = make(map[uint32][]uint64)
	}

	l.counted[urrID] = current

	if downlink {
		l.downlink += delta
	} else {
		l.uplink += delta
	}

	return nil
}

func (conn *SessionEngine) TakeUsage(session *Session) (models.UsageReport, error) {
	l := &session.usage

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return models.UsageReport{SEID: session.SEID}, nil
	}

	directions := urrDirections(session.ListPDRs())

	var errs []error

	for _, id := range l.urrIDsLocked(directions) {
		if err := conn.countURRLocked(session, id, directions[id]); err != nil {
			errs = append(errs, err)
		}
	}

	return l.reportLocked(session.SEID), errors.Join(errs...)
}

func (conn *SessionEngine) ReturnUsage(report models.UsageReport) bool {
	session := conn.GetSession(report.SEID)
	if session == nil {
		return false
	}

	l := &session.usage

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return false
	}

	l.uplink += report.UplinkVolume
	l.downlink += report.DownlinkVolume

	return true
}

func (conn *SessionEngine) closeUsage(session *Session, directions map[uint32]bool) (models.UsageReport, error) {
	l := &session.usage

	l.mu.Lock()
	defer l.mu.Unlock()

	var errs []error

	for _, id := range l.urrIDsLocked(directions) {
		if err := conn.countURRLocked(session, id, directions[id]); err != nil {
			errs = append(errs, err)
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
	l.created = nil

	return l.reportLocked(session.SEID), errors.Join(errs...)
}

func (conn *SessionEngine) waitForPrograms(ctx context.Context) {
	if conn.programs == nil {
		return
	}

	if err := conn.programs.Wait(); err != nil {
		logger.From(ctx, logger.UpfLog).Warn("could not wait for running datapath programs", zap.Error(err))
	}
}
