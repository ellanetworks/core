// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package procedure is the procedure registry engine used by the 4G MME and 5G
// AMF per UE, and by the SMF per session. Each owner defines a small set of
// mutually-exclusive procedures, of which at most one runs at a time; the
// registry holds that one and supervises it with an optional deadline and
// cancel callback. The procedure type set lives with each owner; this package
// is the mechanism only.
package procedure

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Type identifies a kind of procedure tracked by the registry. Its values are
// defined by each owner.
type Type string

// Sentinel errors.
var (
	ErrConflict      = errors.New("conflicting procedure active")
	ErrAlreadyActive = errors.New("procedure already active")
	ErrNotActive     = errors.New("procedure not active")
	ErrSettling      = errors.New("procedure is being torn down")
)

type Disposition uint8

const (
	Release Disposition = iota
	Retain
)

// Retain re-invokes it an interval later, so its Retain path must have no side effects.
type CancelFunc func(context.Context) (Disposition, error)

// held is the single active procedure. A fresh value is allocated per Begin, so a
// deadline timer captures its own instance by pointer identity and cannot expire a
// later procedure that reused the same Type.
type held struct {
	typ      Type
	timer    *time.Timer
	cancel   CancelFunc
	interval time.Duration
	retains  int
	settling bool
	ended    bool
}

// Registry tracks the one active procedure of a single UE or session.
type Registry struct {
	mu     sync.Mutex
	log    *zap.Logger
	active *held
}

// NewRegistry returns an empty registry bound to a logger.
func NewRegistry(log *zap.Logger) *Registry {
	return &Registry{log: log}
}

// Begin starts t. Returns ErrAlreadyActive if t is already the active procedure, or
// ErrConflict if a different one is active — the tracked types are mutually
// exclusive, so any active procedure blocks any other.
func (r *Registry) Begin(t Type) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active != nil {
		if r.active.typ == t {
			r.log.Info("procedure rejected: already active", zap.String("type", string(t)))
			return ErrAlreadyActive
		}

		r.log.Info("procedure rejected: conflict",
			zap.String("incoming", string(t)),
			zap.String("active", string(r.active.typ)),
		)

		return ErrConflict
	}

	r.active = &held{typ: t}
	r.log.Debug("procedure started", zap.String("type", string(t)))

	return nil
}

// Supervise arms a deadline timer and cancel callback on the active procedure t.
// Use it when the supervision deadline and its cleanup are only known after Begin —
// e.g. an N2 handover whose target UE is created mid-handler and must be captured by
// the cancel. Arming after the relevant state is written gives the timer goroutine a
// happens-before edge to it. A subsequent End or Cancel stops the timer. Returns
// ErrNotActive if t is not active.
func (r *Registry) Supervise(t Type, deadline time.Time, cancel CancelFunc) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active == nil || r.active.typ != t {
		return ErrNotActive
	}

	if r.active.settling {
		return ErrSettling
	}

	if r.active.timer != nil {
		r.active.timer.Stop()
	}

	r.active.cancel = cancel
	h := r.active

	d := time.Until(deadline)
	if d <= 0 {
		d = time.Millisecond
	}

	h.interval, h.retains = d, 0
	h.timer = time.AfterFunc(d, func() { r.expire(h) })

	return nil
}

// End marks t as finished (success path). Does not invoke the cancel callback. A
// no-op if t is not the active procedure.
func (r *Registry) End(t Type) {
	r.mu.Lock()

	h := r.active
	if h == nil || h.typ != t {
		r.mu.Unlock()
		return
	}

	if h.settling {
		h.ended = true
		r.mu.Unlock()
		r.log.Debug("procedure ended mid-teardown; the slot frees when the teardown returns",
			zap.String("type", string(t)))

		return
	}

	r.active = nil
	r.mu.Unlock()

	if h.timer != nil {
		h.timer.Stop()
	}

	r.log.Debug("procedure ended", zap.String("type", string(t)))
}

// Cancel removes the active procedure t and invokes its cancel callback. Returns
// ErrNotActive if t is not active.
func (r *Registry) Cancel(ctx context.Context, t Type) error {
	r.mu.Lock()

	h := r.active
	if h == nil || h.typ != t {
		r.mu.Unlock()
		return ErrNotActive
	}

	if h.settling {
		r.mu.Unlock()
		return ErrSettling
	}

	h.settling = true
	r.mu.Unlock()

	if h.timer != nil {
		h.timer.Stop()
	}

	r.log.Info("procedure cancelled", zap.String("type", string(t)), zap.String("reason", "explicit"))
	r.settle(ctx, h)

	return nil
}

// Active reports whether t is the active procedure.
func (r *Registry) Active(t Type) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.active != nil && r.active.typ == t
}

// ActiveTypes returns the active procedure type (at most one), suitable for
// diagnostics/export.
func (r *Registry) ActiveTypes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active == nil {
		return nil
	}

	return []string{string(r.active.typ)}
}

// expire is called by the deadline timer. It removes the procedure only if its own
// instance is still active — a matching Type begun after End/Cancel is a different
// instance and must not be expired by this timer.
func (r *Registry) expire(h *held) {
	r.mu.Lock()

	if r.active != h || h.settling {
		r.mu.Unlock()
		return
	}

	h.settling = true
	r.mu.Unlock()

	r.log.Warn("procedure expired", zap.String("type", string(h.typ)), zap.String("reason", "timeout"))
	r.settle(context.Background(), h)
}

func (r *Registry) settle(ctx context.Context, h *held) {
	disposition := r.invokeCancel(ctx, h)

	r.mu.Lock()
	defer r.mu.Unlock()

	h.settling = false

	if r.active != h {
		r.log.Error("the procedure being torn down is no longer the active one",
			zap.String("type", string(h.typ)))

		return
	}

	if disposition == Retain && !h.ended {
		h.retains++
		h.timer = time.AfterFunc(h.interval, func() { r.expire(h) })

		r.log.Warn("procedure retained past its deadline by its cancel callback; supervision re-armed",
			zap.String("type", string(h.typ)),
			zap.Int("retains", h.retains),
			zap.Duration("interval", h.interval))

		return
	}

	r.active = nil
}

// invokeCancel calls the cancel callback outside the lock, recovering panics.
func (r *Registry) invokeCancel(ctx context.Context, h *held) (disposition Disposition) {
	if h.cancel == nil {
		return Release
	}

	defer func() {
		if rv := recover(); rv != nil {
			r.log.Error("cancel callback panicked", zap.String("type", string(h.typ)), zap.Any("panic", rv))

			disposition = Release
		}
	}()

	var err error

	disposition, err = h.cancel(ctx)
	if err != nil {
		r.log.Warn("cancel callback error", zap.String("type", string(h.typ)), zap.Error(err))
	}

	return disposition
}
