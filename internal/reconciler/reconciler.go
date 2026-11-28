// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package reconciler

import (
	"context"
	"sync"
	"time"
)

const sessionReconcileBackstop = 5 * time.Minute

type Feed interface {
	C() <-chan struct{}
	Begin() uint64
	Done(index uint64)
}

// Reconciler subscribes to the session_reconcile changefeed topic and runs its
// sweeps, which reconcile every local session and UE against the current DB
// policy. It runs on every cluster node; Raft replication guarantees each node
// receives the wakeup after the write applies locally.
type Reconciler struct {
	sweeps   []func(context.Context) error
	feed     Feed
	backstop time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// New creates a reconciler running the given sweeps. feed is signalled
// when a profile/policy/subscriber write that affects session parameters
// has been applied. Start must be called explicitly.
func New(feed Feed, sweeps ...func(context.Context) error) *Reconciler {
	return &Reconciler{
		sweeps:   sweeps,
		feed:     feed,
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
		case <-r.feed.C():
			r.sweep(ctx)
		case <-ticker.C:
			r.sweep(ctx)
		}
	}
}

func (r *Reconciler) sweep(ctx context.Context) {
	target := r.feed.Begin()

	var failed bool

	for _, sweep := range r.sweeps {
		if err := sweep(ctx); err != nil {
			failed = true
		}
	}

	if !failed && ctx.Err() == nil {
		r.feed.Done(target)
	}
}
