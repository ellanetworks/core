// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package drain

import (
	"context"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	reconcileBackstop = 30 * time.Second

	offloadInterval = 2 * time.Second

	offloadBatchSize = 16

	DefaultDeadline = time.Hour
)

type Eligibility interface {
	SetEligible(ctx context.Context, eligible bool) int
}

type Offloader interface {
	Offload(ctx context.Context, batch int) int
	RemainingOffloadable() int
}

type BGPSpeaker interface {
	IsRunning() bool
	Stop() error
	Restart(ctx context.Context) error
}

type Store interface {
	NodeID() int
	ClusterEnabled() bool
	IsBGPEnabled(ctx context.Context) (bool, error)
	GetClusterMember(ctx context.Context, nodeID int) (*db.ClusterMember, error)
	ListClusterMembers(ctx context.Context) ([]db.ClusterMember, error)
	SetDrainState(ctx context.Context, nodeID int, state string) error
}

type Reconciler struct {
	store    Store
	nfs      []Eligibility
	bgp      BGPSpeaker
	wakeup   <-chan struct{}
	backstop time.Duration
	deadline time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func New(store Store, bgp BGPSpeaker, wakeup <-chan struct{}, nfs ...Eligibility) *Reconciler {
	return &Reconciler{
		store:    store,
		nfs:      nfs,
		bgp:      bgp,
		wakeup:   wakeup,
		backstop: reconcileBackstop,
		deadline: DefaultDeadline,
	}
}

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

func (r *Reconciler) Stop() {
	r.mu.Lock()
	cancel, done := r.cancel, r.done
	r.cancel, r.done = nil, nil
	r.mu.Unlock()

	if cancel == nil {
		return
	}

	cancel()
	<-done
}

func (r *Reconciler) loop(ctx context.Context, done chan struct{}) {
	defer close(done)

	backstop := time.NewTicker(r.backstop)
	defer backstop.Stop()

	offload := time.NewTicker(offloadInterval)
	defer offload.Stop()

	r.Reconcile(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.wakeup:
			r.Reconcile(ctx)
		case <-backstop.C:
			r.Reconcile(ctx)
		case <-offload.C:
			r.sweep(ctx)
		}
	}
}

func (r *Reconciler) Reconcile(ctx context.Context) {
	state, ok := r.localState(ctx)
	if !ok {
		return
	}

	eligible := state == db.DrainStateActive

	for _, nf := range r.nfs {
		nf.SetEligible(ctx, eligible)
	}

	r.reconcileBGP(ctx, eligible)
}

func (r *Reconciler) localState(ctx context.Context) (string, bool) {
	if !r.store.ClusterEnabled() {
		return db.DrainStateActive, true
	}

	member, err := r.store.GetClusterMember(ctx, r.store.NodeID())
	if err != nil {
		logger.EllaLog.Warn("drain reconcile: could not read local cluster member", zap.Error(err))
		return "", false
	}

	if member.DrainState == "" {
		return db.DrainStateActive, true
	}

	return member.DrainState, true
}

func (r *Reconciler) reconcileBGP(ctx context.Context, eligible bool) {
	if r.bgp == nil {
		return
	}

	if !eligible {
		if r.bgp.IsRunning() {
			if err := r.bgp.Stop(); err != nil {
				logger.EllaLog.Warn("drain reconcile: BGP stop failed", zap.Error(err))
			}
		}

		return
	}

	if r.bgp.IsRunning() {
		return
	}

	enabled, err := r.store.IsBGPEnabled(ctx)
	if err != nil || !enabled {
		return
	}

	if err := r.bgp.Restart(ctx); err != nil {
		logger.EllaLog.Warn("drain reconcile: BGP restart failed", zap.Error(err))
	}
}

func (r *Reconciler) sweep(ctx context.Context) {
	if !r.store.ClusterEnabled() {
		return
	}

	member, err := r.store.GetClusterMember(ctx, r.store.NodeID())
	if err != nil || member.DrainState != db.DrainStateDraining {
		return
	}

	if !r.hasSomewhereToGo(ctx) {
		return
	}

	batch := offloadBatchSize
	if r.pastDeadline(member) {
		batch = 0
	}

	remaining := 0

	for _, nf := range r.nfs {
		o, ok := nf.(Offloader)
		if !ok {
			continue
		}

		o.Offload(ctx, batch)

		remaining += o.RemainingOffloadable()
	}

	if remaining > 0 && batch > 0 {
		return
	}

	if err := r.store.SetDrainState(ctx, r.store.NodeID(), db.DrainStateDrained); err != nil {
		logger.EllaLog.Warn("drain reconcile: could not mark drain complete", zap.Error(err))
		return
	}

	logger.EllaLog.Info("drain complete; no subscribers left to off-load")
}

func (r *Reconciler) pastDeadline(member *db.ClusterMember) bool {
	if member.DrainUpdatedAt == 0 {
		return false
	}

	return time.Since(time.Unix(member.DrainUpdatedAt, 0)) >= r.deadline
}

func (r *Reconciler) hasSomewhereToGo(ctx context.Context) bool {
	members, err := r.store.ListClusterMembers(ctx)
	if err != nil {
		return false
	}

	for _, m := range members {
		if m.NodeID == r.store.NodeID() {
			continue
		}

		if m.DrainState == "" || m.DrainState == db.DrainStateActive {
			return true
		}
	}

	return false
}
