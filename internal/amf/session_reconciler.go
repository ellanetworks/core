// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

const sessionReconcileBackstop = 5 * time.Minute

// SessionReconciler subscribes to the session_reconcile changefeed topic and
// reconciles every local PDU session against the current DB policy. It runs
// on every cluster node; Raft replication guarantees each node receives the
// wakeup after the write applies locally.
type SessionReconciler struct {
	amf      *AMF
	wakeup   <-chan struct{}
	backstop time.Duration
	log      *zap.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewSessionReconciler creates a reconciler for the given AMF. wakeup is
// signalled when a profile/policy/subscriber write that affects session
// parameters has been applied; nil is fine (then only the backstop sweep
// fires). Start must be called explicitly.
func NewSessionReconciler(amf *AMF, wakeup <-chan struct{}) *SessionReconciler {
	return &SessionReconciler{
		amf:      amf,
		wakeup:   wakeup,
		backstop: sessionReconcileBackstop,
		log:      logger.AmfLog.With(zap.String("component", "SessionReconciler")),
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

	r.Reconcile()

	ticker := time.NewTicker(r.backstop)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.wakeup:
			r.Reconcile()
		case <-ticker.C:
			r.Reconcile()
		}
	}
}

// Reconcile iterates every registered UE and its PDU sessions, asking the
// SMF to compare the current session policy against the latest DB values
// and push updates to the UPF and UE where needed.
func (r *SessionReconciler) Reconcile() {
	r.amf.mu.RLock()
	ues := make([]*UeContext, 0, len(r.amf.UEs))

	for _, ue := range r.amf.UEs {
		if ue.State() == Registered {
			ues = append(ues, ue)
		}
	}

	r.amf.mu.RUnlock()

	if len(ues) == 0 {
		return
	}

	for _, ue := range ues {
		r.reconcileUE(ue)
	}
}

func (r *SessionReconciler) reconcileUE(ue *UeContext) {
	r.amf.ReconcileSessionsForUE(context.Background(), ue)
}

// ReconcileSessionsForUE re-evaluates every PDU session of a UE against the
// current DB policy and applies any change (UPF, gNB, and UE) via the SMF.
func (amf *AMF) ReconcileSessionsForUE(ctx context.Context, ue *UeContext) {
	if ue == nil {
		return
	}

	ue.mu.Lock()
	smContextRefs := make([]string, 0, len(ue.SmContextList))

	for _, smCtx := range ue.SmContextList {
		smContextRefs = append(smContextRefs, smCtx.Ref)
	}

	ue.mu.Unlock()

	for _, ref := range smContextRefs {
		if ref == "" {
			continue
		}

		policy, reason := amf.fetchSessionPolicy(ref)

		// ReconcileSkip signals a transient error; let the backstop timer retry.
		if reason == models.ReconcileSkip {
			continue
		}

		if err := amf.Session.ReconcileSmContext(ctx, &models.SessionReconcileRequest{
			SmContextRef: ref,
			NewPolicy:    policy,
			Reason:       reason,
		}); err != nil {
			logger.AmfLog.Warn("session reconcile failed",
				zap.String("smContextRef", ref),
				zap.Error(err))
		}
	}
}

// fetchSessionPolicy reads the latest policy for a session from the DB.
// Returns (nil, ReconcileSliceMismatch) when the policy cannot be resolved
// because the session's stored Snssai matches no active slice
// (the admin changed SST/SD). Returns (nil, ReconcileSkip) when the policy
// cannot be determined (transient DB error, session gone, nil policy) so the
// caller skips reconciliation and the backstop retries later.
func (amf *AMF) fetchSessionPolicy(smContextRef string) (*models.SessionPolicyDelta, models.SessionReconcileReason) {
	sm := amf.Session.GetSession(smContextRef)
	if sm == nil {
		// Session already removed from the SMF pool (e.g. after a
		// network-initiated release); nothing to reconcile.
		return nil, models.ReconcileSkip
	}

	policy, err := amf.Session.GetSessionPolicy(context.Background(), sm.Supi, sm.Snssai, sm.Dnn)
	if err != nil {
		// Distinguish "no matching policy" (genuine slice mismatch) from
		// transient infrastructure errors (DB down, Raft timeout, etc.).
		if errors.Is(err, smf.ErrNoPolicyMatch) || errors.Is(err, smf.ErrDNNNotFound) {
			logger.AmfLog.Debug("session policy not found, triggering slice mismatch release",
				zap.String("smContextRef", smContextRef),
				zap.Error(err))

			return nil, models.ReconcileSliceMismatch
		}

		// The backstop timer will retry.
		logger.AmfLog.Warn("transient error fetching session policy, skipping reconciliation",
			zap.String("smContextRef", smContextRef),
			zap.Error(err))

		return nil, models.ReconcileSkip
	}

	if policy == nil {
		return nil, models.ReconcileSkip
	}

	dnsStr := ""
	if policy.DNS != nil {
		dnsStr = policy.DNS.String()
	}

	delta := &models.SessionPolicyDelta{
		SessionAmbrUplink:   policy.Ambr.Uplink,
		SessionAmbrDownlink: policy.Ambr.Downlink,
		Var5qi:              policy.QosData.Var5qi,
		DNS:                 dnsStr,
		MTU:                 policy.MTU,
		IPv4Pool:            policy.IPv4Pool,
		IPv6Pool:            policy.IPv6Pool,
	}
	if policy.QosData.Arp != nil {
		delta.Arp = policy.QosData.Arp.PriorityLevel
		delta.PreemptCap = policy.QosData.Arp.PreemptCap
		delta.PreemptVuln = policy.QosData.Arp.PreemptVuln
	}

	return delta, models.ReconcilePolicyChange
}
