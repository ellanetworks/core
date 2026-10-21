// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"slices"
	"sync"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

type SessionEventKind string

const (
	SessionEstablished       SessionEventKind = "established"
	SessionUsageApplied      SessionEventKind = "usage_applied"
	SessionConditionsChanged SessionEventKind = "conditions_changed"
	SessionSDFStarted        SessionEventKind = "sdf_started"
	SessionSDFTerminated     SessionEventKind = "sdf_terminated"
	SessionHandoverStarted   SessionEventKind = "inter_system_handover_started"
	SessionHandoverCancelled SessionEventKind = "inter_system_handover_cancelled"
	SessionHandoverCompleted SessionEventKind = "inter_system_handover_completed"
	SessionReleased          SessionEventKind = "released"
)

type SessionEvent struct {
	Kind              SessionEventKind
	At                time.Time
	Ref               string
	SUPI              etsi.SUPI
	ChargingID        uint32
	Triggers          []TriggerType
	Prev              ChargingConditions
	Next              ChargingConditions
	Flow              ChargingFlow
	LastOfRatingGroup bool
	Usage             []URRUsage
	Release           *models.SessionReleaseRequest
}

type SessionObserver func(SessionEvent)

func WithSessionObserver(o SessionObserver) Option {
	return func(s *SMF) { s.observers = append(s.observers, o) }
}

type sessionMutex struct {
	mu sync.Mutex
	sc *SMContext
}

func (m *sessionMutex) Lock() {
	m.mu.Lock()
	m.sc.settleLocked(time.Now())
}

func (m *sessionMutex) TryLock() bool {
	if !m.mu.TryLock() {
		return false
	}

	m.sc.settleLocked(time.Now())

	return true
}

func (m *sessionMutex) Unlock() {
	m.sc.settleLocked(time.Now())
	queued := m.sc.queueEventsLocked()
	m.mu.Unlock()

	if queued {
		m.sc.outbox.drain(m.sc.observers)
	}
}

type sessionEvents struct {
	charged *ChargingConditions
	lastAt  time.Time
	pending []SessionEvent
}

type eventOutbox struct {
	mu       sync.Mutex
	queue    []SessionEvent
	draining bool
}

func (o *eventOutbox) push(events []SessionEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.queue = append(o.queue, events...)
}

func (o *eventOutbox) drain(observers []SessionObserver) {
	o.mu.Lock()

	if o.draining {
		o.mu.Unlock()
		return
	}

	o.draining = true

	for len(o.queue) > 0 {
		batch := o.queue
		o.queue = nil
		o.mu.Unlock()

		for _, e := range batch {
			for _, observe := range observers {
				observe(e)
			}
		}

		o.mu.Lock()
	}

	o.draining = false
	o.mu.Unlock()
}

func (sc *SMContext) queueEventsLocked() bool {
	if sc == nil || len(sc.events.pending) == 0 {
		return false
	}

	pending := sc.events.pending
	sc.events.pending = nil

	if len(sc.observers) == 0 {
		return false
	}

	sc.outbox.push(pending)

	return true
}

func (sc *SMContext) emitLocked(e SessionEvent) {
	if sc.events.charged == nil {
		return
	}

	if e.At.IsZero() {
		e.At = time.Now()
	}

	if e.At.Before(sc.events.lastAt) {
		e.At = sc.events.lastAt
	}

	sc.events.lastAt = e.At
	e.Ref, e.SUPI, e.ChargingID = sc.Ref, sc.Supi, sc.ChargingID
	sc.events.pending = append(sc.events.pending, e)
}

func (sc *SMContext) establishedLocked(now time.Time) {
	c := sc.chargingConditionsLocked()
	sc.events.charged = &c
	sc.emitLocked(SessionEvent{Kind: SessionEstablished, At: now, Prev: c, Next: c})
}

func (sc *SMContext) settleLocked(now time.Time) {
	if sc == nil || sc.events.charged == nil || sc.releasing || sc.Tunnel == nil {
		return
	}

	prev := *sc.events.charged
	next := sc.chargingConditionsLocked()
	sc.events.charged = &next

	started, terminated := flowChanges(prev.Flows, next.Flows)
	triggers := conditionTriggers(prev, next)

	for _, f := range terminated {
		sc.emitLocked(SessionEvent{Kind: SessionSDFTerminated, At: now, Prev: prev, Next: next, Flow: f, LastOfRatingGroup: lastOfRatingGroup(f, next.Flows)})
	}

	if len(triggers) > 0 {
		at := now

		if !slices.ContainsFunc(triggers, func(t TriggerType) bool { return !t.fromUEInfo() }) {
			if changed := sc.UEInfoChangedAt(); !changed.IsZero() && changed.Before(now) {
				at = changed
			}
		}

		sc.emitLocked(SessionEvent{Kind: SessionConditionsChanged, At: at, Triggers: triggers, Prev: prev, Next: next})
	}

	for _, f := range started {
		sc.emitLocked(SessionEvent{Kind: SessionSDFStarted, At: now, Prev: prev, Next: next, Flow: f})
	}
}

func (sc *SMContext) usageAppliedLocked(usage []URRUsage) {
	if sc.events.charged == nil {
		return
	}

	sc.emitLocked(SessionEvent{Kind: SessionUsageApplied, Prev: *sc.events.charged, Next: *sc.events.charged, Usage: usage})
}

func (sc *SMContext) handoverLocked(kind SessionEventKind) {
	if kind == SessionHandoverCompleted {
		sc.settleLocked(time.Now())
	}

	if sc.events.charged == nil {
		return
	}

	sc.emitLocked(SessionEvent{Kind: kind, Prev: *sc.events.charged, Next: *sc.events.charged})
}

func (sc *SMContext) releasedLocked(release models.SessionReleaseRequest) {
	if sc.events.charged == nil {
		return
	}

	next := *sc.events.charged
	next.adoptUEInfo(sc.UEInfo())

	sc.emitLocked(SessionEvent{Kind: SessionReleased, Prev: *sc.events.charged, Next: next, Release: &release})
	sc.events.charged = nil
}

func logSessionEvent(e SessionEvent) {
	fields := []zap.Field{
		zap.String("event", string(e.Kind)),
		logger.SMContextRef(e.Ref),
		logger.SUPI(e.SUPI.String()),
		zap.Uint32("charging_id", e.ChargingID),
		zap.Time("at", e.At),
	}

	switch e.Kind {
	case SessionConditionsChanged:
		fields = append(fields, zap.Any("triggers", e.Triggers))
	case SessionSDFStarted, SessionSDFTerminated:
		fields = append(fields, zap.Uint32("rating_group", e.Flow.Key.RatingGroup), zap.Uint32("flow_charging_id", e.Flow.ChargingID),
			zap.Bool("last_of_rating_group", e.LastOfRatingGroup))
	case SessionUsageApplied:
		var uplink, downlink uint64

		for _, u := range e.Usage {
			uplink += u.UplinkVolume
			downlink += u.DownlinkVolume
		}

		fields = append(fields, zap.Int("reports", len(e.Usage)), logger.UplinkVolume(uplink), logger.DownlinkVolume(downlink))
	case SessionReleased:
		fields = append(fields, zap.Stringer("reason", e.Release.Reason))
	}

	logger.SmfLog.Debug("session event", fields...)
}
