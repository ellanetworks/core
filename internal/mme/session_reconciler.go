// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"sync"
	"time"
)

// sessionReconcileBackstop is the period of the data-network reconciler's safety
// sweep, matching the 5G AMF's session reconciler. It recovers from a dropped or
// coalesced changefeed wakeup and from UEs that were transitioning (mid-attach,
// idle) when a data-network change applied.
const sessionReconcileBackstop = 5 * time.Minute

// MMESessionReconciler propagates data-network reconfiguration to active EPS
// bearers: on a session_reconcile changefeed wakeup it re-evaluates every
// connected bearer against the current policy and modifies or reactivates it
// (TS 24.301 §6.4.2 / §6.4.4.2), and a periodic backstop sweep recovers from a
// dropped wakeup. It mirrors the 5G AMF's SessionReconciler; unlike the AMF (which
// delegates the diff to the SMF) the diff itself is MME-owned, since the EPC has
// no SMF for session management.
type MMESessionReconciler struct {
	mme      *MME
	wakeup   <-chan struct{}
	backstop time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewMMESessionReconciler creates a reconciler for the given MME. wakeup is
// signalled when a write that affects bearer data-network parameters has been
// applied; nil is fine (then only the backstop sweep fires). Start must be called
// explicitly.
func NewMMESessionReconciler(m *MME, wakeup <-chan struct{}) *MMESessionReconciler {
	return &MMESessionReconciler{
		mme:      m,
		wakeup:   wakeup,
		backstop: sessionReconcileBackstop,
	}
}

// Start launches the reconciler goroutine. Safe to call while already running;
// subsequent calls without a paired Stop are no-ops. The first reconcile runs in
// the goroutine immediately, then the wakeup and periodic ticker take over.
func (r *MMESessionReconciler) Start() {
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

// Stop signals the reconciler to exit and blocks until the goroutine has drained.
// Safe to call when not started.
func (r *MMESessionReconciler) Stop() {
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

func (r *MMESessionReconciler) loop(ctx context.Context, done chan struct{}) {
	defer close(done)

	r.mme.ReconcileDataNetwork(ctx)

	ticker := time.NewTicker(r.backstop)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.wakeup:
			r.mme.ReconcileDataNetwork(ctx)
		case <-ticker.C:
			r.mme.ReconcileDataNetwork(ctx)
		}
	}
}
