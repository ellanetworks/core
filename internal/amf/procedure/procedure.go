// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package procedure

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Type identifies a kind of procedure tracked by the registry.
type Type string

const (
	Registration   Type = "Registration"
	Authentication Type = "Authentication"
	SecurityMode   Type = "SecurityMode"
	N2Handover     Type = "N2Handover"
	UEContextMod   Type = "UEContextModification"
	Paging         Type = "Paging"
)

// ID is an opaque handle returned by Begin. It uniquely identifies a
// procedure instance within the registry and is required for re-entrant
// types (e.g. Paging) where multiple instances of the same Type coexist.
type ID uint64

var nextID atomic.Uint64

// Procedure is the unit tracked by the registry.
type Procedure struct {
	Type      Type
	StartedAt time.Time
	Deadline  time.Time
	Cancel    func(context.Context) error
}

// Sentinel errors.
var (
	ErrConflict      = errors.New("conflicting procedure active")
	ErrAlreadyActive = errors.New("procedure already active")
	ErrNotActive     = errors.New("procedure not active")
)

// entry is the internal tracked state of an active procedure.
type entry struct {
	id        ID
	proc      Procedure
	timer     *time.Timer
	startedAt time.Time
}

// Registry tracks which procedures are active for a single UE and
// enforces concurrency rules from TS 33.501 §6.9.5.1 via a conflict
// matrix.
type Registry struct {
	mu      sync.Mutex
	active  []entry // ordered by insertion (Begin order)
	log     *zap.Logger
	stopped bool
}

// NewRegistry returns an empty registry bound to a logger.
func NewRegistry(log *zap.Logger) *Registry {
	return &Registry{log: log}
}

// Begin atomically starts p. Returns ErrConflict if any currently-active
// procedure is incompatible with p.Type per the conflict matrix, or
// ErrAlreadyActive if p.Type is already active and is not re-entrant.
// On success a deadline timer is armed if p.Deadline is non-zero.
func (r *Registry) Begin(ctx context.Context, p Procedure) (ID, error) {
	r.mu.Lock()

	if r.stopped {
		r.mu.Unlock()
		return 0, errors.New("registry stopped")
	}

	// Check conflicts against every active procedure.
	for _, e := range r.active {
		if e.proc.Type == p.Type {
			if isReentrant(p.Type) {
				continue // re-entrant: allow multiple instances
			}
			r.mu.Unlock()
			r.log.Info("procedure rejected: already active",
				zap.String("type", string(p.Type)),
			)

			return 0, ErrAlreadyActive
		}

		if blocked, rule := conflicts(e.proc.Type, p.Type); blocked {
			r.mu.Unlock()
			r.log.Info("procedure rejected: conflict",
				zap.String("incoming", string(p.Type)),
				zap.String("active", string(e.proc.Type)),
				zap.String("rule", rule),
			)

			return 0, ErrConflict
		}
	}

	now := time.Now()
	if p.StartedAt.IsZero() {
		p.StartedAt = now
	}

	id := ID(nextID.Add(1))
	e := entry{
		id:        id,
		proc:      p,
		startedAt: now,
	}

	// Arm deadline timer.
	if !p.Deadline.IsZero() {
		d := time.Until(p.Deadline)
		if d <= 0 {
			d = time.Millisecond
		}

		e.timer = time.AfterFunc(d, func() {
			r.expireByID(ctx, id)
		})
	}

	r.active = append(r.active, e)
	r.mu.Unlock()

	r.log.Debug("procedure started",
		zap.String("type", string(p.Type)),
		zap.Uint64("id", uint64(id)),
		zap.Time("deadline", p.Deadline),
	)

	return id, nil
}

// End marks t as finished (success path). Does not invoke Cancel.
// For non-re-entrant types this removes the single active instance.
// No-op if t is not active.
func (r *Registry) End(t Type) {
	r.mu.Lock()
	idx := -1

	for i, e := range r.active {
		if e.proc.Type == t {
			idx = i
			break
		}
	}

	if idx < 0 {
		r.mu.Unlock()
		return
	}

	e := r.active[idx]
	r.active = append(r.active[:idx], r.active[idx+1:]...)
	r.mu.Unlock()

	if e.timer != nil {
		e.timer.Stop()
	}

	r.log.Debug("procedure ended",
		zap.String("type", string(t)),
		zap.Uint64("id", uint64(e.id)),
	)
}

// EndByID marks a specific instance as finished. Required for
// re-entrant types where multiple instances coexist.
func (r *Registry) EndByID(id ID) {
	r.mu.Lock()

	idx := r.indexByID(id)
	if idx < 0 {
		r.mu.Unlock()
		return
	}

	e := r.active[idx]
	r.active = append(r.active[:idx], r.active[idx+1:]...)
	r.mu.Unlock()

	if e.timer != nil {
		e.timer.Stop()
	}

	r.log.Debug("procedure ended by ID",
		zap.String("type", string(e.proc.Type)),
		zap.Uint64("id", uint64(id)),
	)
}

// Cancel invokes the Cancel callback of the first instance of t
// (oldest, FIFO) and removes it. Returns ErrNotActive if t was not active.
func (r *Registry) Cancel(ctx context.Context, t Type) error {
	r.mu.Lock()
	idx := -1

	for i, e := range r.active {
		if e.proc.Type == t {
			idx = i
			break
		}
	}

	if idx < 0 {
		r.mu.Unlock()
		return ErrNotActive
	}

	e := r.active[idx]
	r.active = append(r.active[:idx], r.active[idx+1:]...)
	r.mu.Unlock()

	if e.timer != nil {
		e.timer.Stop()
	}

	r.log.Info("procedure cancelled",
		zap.String("type", string(t)),
		zap.Uint64("id", uint64(e.id)),
		zap.String("reason", "explicit"),
	)

	r.invokeCancel(ctx, e)

	return nil
}

// CancelByID cancels a specific instance by ID.
func (r *Registry) CancelByID(ctx context.Context, id ID) error {
	r.mu.Lock()

	idx := r.indexByID(id)
	if idx < 0 {
		r.mu.Unlock()
		return ErrNotActive
	}

	e := r.active[idx]
	r.active = append(r.active[:idx], r.active[idx+1:]...)
	r.mu.Unlock()

	if e.timer != nil {
		e.timer.Stop()
	}

	r.log.Info("procedure cancelled by ID",
		zap.String("type", string(e.proc.Type)),
		zap.Uint64("id", uint64(id)),
		zap.String("reason", "explicit"),
	)

	r.invokeCancel(ctx, e)

	return nil
}

// CancelAll invokes Cancel for every active procedure in reverse-Begin
// order (most recently started first), then clears the registry.
func (r *Registry) CancelAll(ctx context.Context) {
	r.mu.Lock()
	entries := make([]entry, len(r.active))
	copy(entries, r.active)
	r.active = r.active[:0]
	r.stopped = true
	r.mu.Unlock()

	// Stop timers first.
	for _, e := range entries {
		if e.timer != nil {
			e.timer.Stop()
		}
	}

	// Cancel in reverse-Begin order.
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		r.log.Info("procedure cancelled",
			zap.String("type", string(e.proc.Type)),
			zap.Uint64("id", uint64(e.id)),
			zap.String("reason", "teardown"),
		)
		r.invokeCancel(ctx, e)
	}
}

// Active returns true if t has at least one active instance.
func (r *Registry) Active(t Type) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, e := range r.active {
		if e.proc.Type == t {
			return true
		}
	}

	return false
}

// Snapshot returns a copy of the active set for diagnostics.
func (r *Registry) Snapshot() []Procedure {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]Procedure, len(r.active))
	for i, e := range r.active {
		p := e.proc
		p.Cancel = nil // don't leak callbacks
		out[i] = p
	}

	return out
}

// ActiveTypes returns the set of active procedure type strings,
// suitable for diagnostics/export.
func (r *Registry) ActiveTypes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.active) == 0 {
		return nil
	}

	seen := make(map[Type]bool, len(r.active))

	out := make([]string, 0, len(r.active))
	for _, e := range r.active {
		if !seen[e.proc.Type] {
			seen[e.proc.Type] = true
			out = append(out, string(e.proc.Type))
		}
	}

	return out
}

// expireByID is called by the deadline timer.
func (r *Registry) expireByID(ctx context.Context, id ID) {
	r.mu.Lock()

	idx := r.indexByID(id)
	if idx < 0 {
		r.mu.Unlock()
		return // already ended/cancelled
	}

	e := r.active[idx]
	r.active = append(r.active[:idx], r.active[idx+1:]...)
	r.mu.Unlock()

	// Timer already fired; no need to stop it.
	r.log.Warn("procedure expired",
		zap.String("type", string(e.proc.Type)),
		zap.Uint64("id", uint64(e.id)),
		zap.String("reason", "timeout"),
	)

	r.invokeCancel(ctx, e)
}

// invokeCancel calls the cancel callback outside the lock, recovering panics.
func (r *Registry) invokeCancel(ctx context.Context, e entry) {
	if e.proc.Cancel == nil {
		return
	}

	func() {
		defer func() {
			if rv := recover(); rv != nil {
				r.log.Error("cancel callback panicked",
					zap.String("type", string(e.proc.Type)),
					zap.Any("panic", rv),
				)
			}
		}()

		if err := e.proc.Cancel(ctx); err != nil {
			r.log.Warn("cancel callback error",
				zap.String("type", string(e.proc.Type)),
				zap.Error(err),
			)
		}
	}()
}

func (r *Registry) indexByID(id ID) int {
	for i, e := range r.active {
		if e.id == id {
			return i
		}
	}

	return -1
}
