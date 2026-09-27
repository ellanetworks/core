// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"context"
	"sync"
	"time"
)

const sessionReconcileBackstop = 5 * time.Minute

// Reconciler subscribes to the session_reconcile changefeed topic and runs its
// sweeps, which reconcile every local session and UE against the current DB
// policy. It runs on every cluster node; Raft replication guarantees each node
// receives the wakeup after the write applies locally.
type Reconciler struct {
	sweeps   []func(context.Context)
	wakeup   <-chan struct{}
	backstop time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// New creates a reconciler running the given sweeps. wakeup is
// signalled when a profile/policy/subscriber write that affects session
// parameters has been applied; nil is fine (then only the backstop sweep
// fires). Start must be called explicitly.
func New(wakeup <-chan struct{}, sweeps ...func(context.Context)) *Reconciler {
	return &Reconciler{
		sweeps:   sweeps,
		wakeup:   wakeup,
		backstop: sessionReconcileBackstop,
	}
}

// Start launches the reconciler goroutine. Safe to call while already
// running; subsequent calls without a paired Stop are no-ops. The first
// reconcile runs synchronously in the goroutine immediately, then the
// periodic ticker takes over.
func (r *Reconciler) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})

	go r.loop(ctx, r.done)
}

// Stop signals the reconciler to exit and blocks until the goroutine
// has drained. Safe to call when not started.
func (r *Reconciler) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	done := r.done
	r.cancel = nil
	r.done = nil
	r.mu.Unlock()

	if cancel == nil {
		return
	}

	cancel()
	<-done
}

func (r *Reconciler) loop(ctx context.Context, done chan struct{}) {
	defer close(done)

	r.sweep(ctx)

	ticker := time.NewTicker(r.backstop)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.wakeup:
			r.sweep(ctx)
		case <-ticker.C:
			r.sweep(ctx)
		}
	}
}

func (r *Reconciler) sweep(ctx context.Context) {
	for _, sweep := range r.sweeps {
		sweep(ctx)
	}
}
