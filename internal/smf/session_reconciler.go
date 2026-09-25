// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const sessionReconcileBackstop = 5 * time.Minute

// SessionReconciler subscribes to the session_reconcile changefeed topic and
// reconciles every local session, 5G and EPS, against the current DB policy. It runs
// on every cluster node; Raft replication guarantees each node receives the
// wakeup after the write applies locally.
type SessionReconciler struct {
	smf      *SMF
	wakeup   <-chan struct{}
	backstop time.Duration
	log      *zap.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewSessionReconciler creates a reconciler for the given SMF. wakeup is
// signalled when a profile/policy/subscriber write that affects session
// parameters has been applied; nil is fine (then only the backstop sweep
// fires). Start must be called explicitly.
func NewSessionReconciler(smf *SMF, wakeup <-chan struct{}) *SessionReconciler {
	return &SessionReconciler{
		smf:      smf,
		wakeup:   wakeup,
		backstop: sessionReconcileBackstop,
		log:      logger.Scope("SMF/session-reconciler"),
	}
}

// Start launches the reconciler goroutine. Safe to call while already
// running; subsequent calls without a paired Stop are no-ops. The first
// reconcile runs synchronously in the goroutine immediately, then the
// periodic ticker takes over.
func (r *SessionReconciler) Start() {
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
func (r *SessionReconciler) Stop() {
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

func (r *SessionReconciler) loop(ctx context.Context, done chan struct{}) {
	defer close(done)

	r.smf.Reconcile(ctx)

	ticker := time.NewTicker(r.backstop)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.wakeup:
			r.smf.Reconcile(ctx)
		case <-ticker.C:
			r.smf.Reconcile(ctx)
		}
	}
}
