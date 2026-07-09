// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package procedure is the per-UE procedure registry engine shared by the 4G MME
// and 5G AMF. Each RAT tracks a small set of mutually-exclusive key-changing
// procedures, of which at most one runs per UE at a time; the registry holds that
// single active procedure and supervises it with an optional deadline and cancel
// callback. The RAT-specific procedure type set lives with each RAT; this package
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
// defined per-RAT.
type Type string

// Sentinel errors.
var (
	ErrConflict      = errors.New("conflicting procedure active")
	ErrAlreadyActive = errors.New("procedure already active")
	ErrNotActive     = errors.New("procedure not active")
)

// held is the single active procedure. A fresh value is allocated per Begin, so a
// deadline timer captures its own instance by pointer identity and cannot expire a
// later procedure that reused the same Type.
type held struct {
	typ    Type
	timer  *time.Timer
	cancel func(context.Context) error
}

// Registry tracks the one active key-changing procedure for a single UE.
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
func (r *Registry) Supervise(t Type, deadline time.Time, cancel func(context.Context) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.active == nil || r.active.typ != t {
		return ErrNotActive
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

	r.active.timer = time.AfterFunc(d, func() { r.expire(h) })

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

	r.active = nil
	r.mu.Unlock()

	if h.timer != nil {
		h.timer.Stop()
	}

	r.log.Info("procedure cancelled", zap.String("type", string(t)), zap.String("reason", "explicit"))
	r.invokeCancel(ctx, h)

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

	if r.active != h {
		r.mu.Unlock()
		return
	}

	r.active = nil
	r.mu.Unlock()

	r.log.Warn("procedure expired", zap.String("type", string(h.typ)), zap.String("reason", "timeout"))
	r.invokeCancel(context.Background(), h)
}

// invokeCancel calls the cancel callback outside the lock, recovering panics.
func (r *Registry) invokeCancel(ctx context.Context, h *held) {
	if h.cancel == nil {
		return
	}

	defer func() {
		if rv := recover(); rv != nil {
			r.log.Error("cancel callback panicked", zap.String("type", string(h.typ)), zap.Any("panic", rv))
		}
	}()

	if err := h.cancel(ctx); err != nil {
		r.log.Warn("cancel callback error", zap.String("type", string(h.typ)), zap.Error(err))
	}
}
