// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package procedure is the per-UE procedure registry engine shared by the 4G MME
// and 5G AMF. It tracks which procedures are active for one UE, enforces
// concurrency via a per-RAT conflict policy (Rules) supplied at construction, and
// supervises each with an optional deadline and cancel callback. The RAT-specific
// procedure type set and conflict matrix live with each RAT; this package is the
// mechanism only.
package procedure

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Type identifies a kind of procedure tracked by the registry. Its values are
// defined per-RAT.
type Type string

// Rules is the per-RAT concurrency policy. A nil Rules permits everything and
// makes no type re-entrant.
type Rules interface {
	// Conflicts reports whether an active procedure blocks an incoming one,
	// returning a short rule code for diagnostics.
	Conflicts(active, incoming Type) (blocked bool, rule string)
	// Reentrant reports whether multiple instances of t may coexist.
	Reentrant(t Type) bool
}

// ID is an opaque handle returned by Begin. It uniquely identifies a
// procedure instance within the registry and is required for re-entrant
// types (e.g. Paging) where multiple instances of the same Type coexist.
type ID uint64

var nextID atomic.Uint64

// Procedure is the unit tracked by the registry. State is procedure-specific
// transient state owned by handlers; the registry treats it opaquely.
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
	done      chan struct{}
}

// Registry tracks which procedures are active for a single UE and enforces the
// conflict rules supplied at construction.
type Registry struct {
	mu      sync.Mutex
	active  []entry // ordered by insertion (Begin order)
	log     *zap.Logger
	rules   Rules
	stopped bool
}

// NewRegistry returns an empty registry bound to a logger and a per-RAT conflict
// policy. A nil rules permits everything and makes nothing re-entrant.
func NewRegistry(log *zap.Logger, rules Rules) *Registry {
	return &Registry{log: log, rules: rules}
}

func (r *Registry) reentrant(t Type) bool {
	return r.rules != nil && r.rules.Reentrant(t)
}

func (r *Registry) conflicts(active, incoming Type) (bool, string) {
	if r.rules == nil {
		return false, ""
	}

	return r.rules.Conflicts(active, incoming)
}

// Begin atomically starts p. Returns ErrConflict if any currently-active
// procedure is incompatible with p.Type per the conflict rules, or
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
			if r.reentrant(p.Type) {
				continue // re-entrant: allow multiple instances
			}
			r.mu.Unlock()
			r.log.Info("procedure rejected: already active",
				zap.String("type", string(p.Type)),
			)

			return 0, ErrAlreadyActive
		}

		if blocked, rule := r.conflicts(e.proc.Type, p.Type); blocked {
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
	done := make(chan struct{})
	e := entry{
		id:        id,
		proc:      p,
		startedAt: now,
		done:      done,
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

	// ctx is intentionally captured: the watcher's role is to abort the
	// procedure when ctx is cancelled.
	// #nosec G118 -- captured ctx is the cancellation signal we observe
	go r.watchCtx(ctx, id, done)

	return id, nil
}

// watchCtx aborts the procedure when ctx is cancelled. Exits cleanly when
// the procedure ends (or is cancelled) by other means.
func (r *Registry) watchCtx(ctx context.Context, id ID, done <-chan struct{}) {
	select {
	case <-ctx.Done():
		_ = r.CancelByID(context.Background(), id)
	case <-done:
	}
}

// Supervise arms a deadline timer and cancel callback on the already-active
// procedure of type t. Use it when the supervision deadline and its cleanup are
// only known after Begin — e.g. an N2 handover whose target UE is created
// mid-handler and must be captured by the cancel. Arming after the relevant state
// is written gives the timer goroutine a happens-before edge to it. A subsequent
// End or Cancel stops the timer. Returns ErrNotActive if t is not active.
func (r *Registry) Supervise(ctx context.Context, t Type, deadline time.Time, cancel func(context.Context) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.active {
		if r.active[i].proc.Type != t {
			continue
		}

		if r.active[i].timer != nil {
			r.active[i].timer.Stop()
		}

		r.active[i].proc.Cancel = cancel
		r.active[i].proc.Deadline = deadline

		id := r.active[i].id

		d := time.Until(deadline)
		if d <= 0 {
			d = time.Millisecond
		}

		r.active[i].timer = time.AfterFunc(d, func() { r.expireByID(ctx, id) })

		return nil
	}

	return ErrNotActive
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

	close(e.done)

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

	close(e.done)

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

	close(e.done)

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

	close(e.done)

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

	close(e.done)

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
