// Copyright 2026 Ella Networks

package bgp

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// LeaseStore is the narrow view the reconciler needs over the lease
// table. It is satisfied by *db.Database but kept minimal so the bgp
// package does not depend on the full db surface.
type LeaseStore interface {
	ListActiveLeasesByNode(ctx context.Context, nodeID int) ([]Lease, error)
}

// RIB is the narrow view the reconciler needs over the BGP service's
// route-information base. *BGPService satisfies it. A test-only
// implementation lets us exercise Reconcile without bringing up GoBGP.
type RIB interface {
	Announce(ip netip.Addr, owner string) error
	Withdraw(ip netip.Addr) error
	Paths() map[string]string
}

// Lease is the minimal per-lease record the reconciler consumes. Matches
// the fields read off *db.IPLease. The db package provides an adapter;
// see LeaseStoreFromDB below for the expected shape.
type Lease struct {
	Address netip.Addr
	IMSI    string
}

// reconcileBackstop is the periodic invariant-checking sweep when no
// wakeups have fired. Primary trigger is wakeup signals from the
// caller (typically the lease changefeed); this exists to recover
// from missed signals.
const reconcileBackstop = 5 * time.Minute

// Reconciler continuously reconciles a BGPService's RIB against the set
// of active leases owned by this cluster node.
//
// The replicated ip_leases table is the single source of truth for which
// /32 routes this node should advertise. The reconciler computes a
// desired set (leases where nodeID == local) and diffs it against the
// BGP service's current RIB on each tick. Announce and Withdraw are
// idempotent at the service level, so the reconciler's output is
// convergent regardless of how the RIB was perturbed.
type Reconciler struct {
	rib      RIB
	store    LeaseStore
	nodeID   int
	wakeup   <-chan struct{}
	backstop time.Duration
	log      *zap.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewReconciler wires a reconciler for the given RIB, lease store, and
// cluster node id. wakeup is signalled by the caller when a relevant
// replicated change has applied; nil is fine (then only the backstop
// sweep fires). Start must be called explicitly.
func NewReconciler(rib RIB, store LeaseStore, nodeID int, wakeup <-chan struct{}) *Reconciler {
	return &Reconciler{
		rib:      rib,
		store:    store,
		nodeID:   nodeID,
		wakeup:   wakeup,
		backstop: reconcileBackstop,
		log:      logger.EllaLog.With(zap.String("component", "BGPReconciler")),
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

// Reconcile performs one pass: compute desired set, diff against
// current RIB, apply. Exposed for on-demand triggering (e.g. after a
// known lease change) and unit tests. Safe to call concurrently with
// the background loop; BGPService methods serialise access internally.
func (r *Reconciler) Reconcile(ctx context.Context) error {
	leases, err := r.store.ListActiveLeasesByNode(ctx, r.nodeID)
	if err != nil {
		return fmt.Errorf("list active leases by node: %w", err)
	}

	desired := make(map[string]string, len(leases))
	for _, l := range leases {
		if !l.Address.IsValid() {
			continue
		}

		desired[l.Address.String()] = l.IMSI
	}

	current := r.rib.Paths()

	toAnnounce, toWithdraw := reconcileDiff(desired, current)

	for ipStr, imsi := range toAnnounce {
		ip, parseErr := netip.ParseAddr(ipStr)
		if parseErr != nil {
			continue
		}

		if annErr := r.rib.Announce(ip, imsi); annErr != nil {
			r.log.Warn("announce failed during reconcile",
				zap.String("ip", ipStr), zap.String("imsi", imsi), zap.Error(annErr))
		}
	}

	for _, ipStr := range toWithdraw {
		ip, parseErr := netip.ParseAddr(ipStr)
		if parseErr != nil {
			continue
		}

		if wdErr := r.rib.Withdraw(ip); wdErr != nil {
			r.log.Warn("withdraw failed during reconcile",
				zap.String("ip", ipStr), zap.Error(wdErr))
		}
	}

	return nil
}

func (r *Reconciler) loop(ctx context.Context, done chan struct{}) {
	defer close(done)

	if err := r.Reconcile(ctx); err != nil {
		r.log.Warn("initial BGP reconcile failed", zap.Error(err))
	}

	backstop := time.NewTicker(r.backstop)
	defer backstop.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.wakeup:
		case <-backstop.C:
		}

		if err := r.Reconcile(ctx); err != nil {
			r.log.Warn("BGP reconcile failed", zap.Error(err))
		}
	}
}

// reconcileDiff returns the set of IPs to announce (with their IMSI
// owner) and the set of IPs to withdraw, given a desired and current
// path map. Pure function, exported via tests only.
//
// Rules:
//   - IP in desired, not in current                 → announce.
//   - IP in desired and current, different IMSI     → announce (owner update).
//   - IP in current, not in desired                 → withdraw.
//   - IP in both with same IMSI                     → no-op.
func reconcileDiff(desired, current map[string]string) (toAnnounce map[string]string, toWithdraw []string) {
	toAnnounce = make(map[string]string)

	for ip, imsi := range desired {
		if curIMSI, have := current[ip]; !have || curIMSI != imsi {
			toAnnounce[ip] = imsi
		}
	}

	for ip := range current {
		if _, keep := desired[ip]; !keep {
			toWithdraw = append(toWithdraw, ip)
		}
	}

	return toAnnounce, toWithdraw
}
