// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ueregistration

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	defaultInterval  = 10 * time.Second
	defaultMinRunGap = time.Second
	purgeConcurrency = 16
)

type Store interface {
	RegisterUE(ctx context.Context, imsi, regType, nodeID, cancel string) (int64, error)
	RegisterVisitorUE(ctx context.Context, imsi, partnerID, regType, nodeID, cancel string) (int64, error)
	PurgeUERegistration(ctx context.Context, imsi, regType, nodeID string) error
	GetUERegistration(ctx context.Context, imsi, regType string) (*db.UERegistration, error)
	GetSubscriberOrVisitor(ctx context.Context, imsi string) (*db.Subscriber, error)
	GetVisitor(ctx context.Context, imsi string) (*db.Visitor, error)
	ListVisitorRegistrationsOfNode(ctx context.Context, nodeID string) ([]db.UERegistration, error)
}

type Withdrawal int

const (
	SubscriberDeleted Withdrawal = iota
	RoamingWithdrawn
	RoamerDeregistered
)

type Holder interface {
	HoldsUE(imsi string) bool
	ReconcileRegistrations(ctx context.Context)
}

type purgeKey struct {
	regType string
	imsi    string
}

type Registry struct {
	store    Store
	nodeID   string
	log      *zap.Logger
	locks    keyedMutex
	interval time.Duration
	minGap   time.Duration

	kick chan struct{}

	mu        sync.Mutex
	holders   map[string]Holder
	pending   map[purgeKey]struct{}
	leftovers bool
}

func New(store Store, nodeID string, log *zap.Logger) *Registry {
	return &Registry{
		store:    store,
		nodeID:   nodeID,
		log:      log,
		locks:    keyedMutex{entries: map[string]*keyedEntry{}},
		interval: defaultInterval,
		minGap:   defaultMinRunGap,
		kick:     make(chan struct{}, 1),
		holders:  map[string]Holder{},
		pending:  map[purgeKey]struct{}{},
	}
}

func (r *Registry) purgeLeftovers(ctx context.Context) {
	r.mu.Lock()
	done := r.leftovers
	r.mu.Unlock()

	if done {
		return
	}

	regs, err := r.store.ListVisitorRegistrationsOfNode(ctx, r.nodeID)
	if err != nil {
		r.log.Warn("failed to list the visitor registrations left by a previous run; will retry", zap.Error(err))
		return
	}

	r.mu.Lock()
	r.leftovers = true
	r.mu.Unlock()

	for _, reg := range regs {
		if h := r.holder(reg.Type); h != nil && h.HoldsUE(reg.Imsi) {
			continue
		}

		r.log.Info("purging a visitor registration left by a previous run", logger.SUPIFromIMSI(reg.Imsi), zap.String("type", reg.Type))
		r.enqueuePurge(reg.Type, reg.Imsi)
	}
}

func (r *Registry) Bind(regType, otherType string, h Holder) *Binding {
	r.mu.Lock()
	r.holders[regType] = h
	r.mu.Unlock()

	return &Binding{r: r, regType: regType, otherType: otherType}
}

func (r *Registry) Run(ctx context.Context, wakeup <-chan struct{}, onTick ...func(context.Context)) {
	tick := time.NewTicker(r.interval)
	defer tick.Stop()

	r.purgeLeftovers(ctx)

	deferred := time.NewTimer(r.minGap)
	deferred.Stop()

	defer deferred.Stop()

	var (
		lastRun time.Time
		pending bool
	)

	reconcile := func() {
		r.reconcileAll(ctx)

		lastRun = time.Now()
	}

	cancelPending := func() {
		deferred.Stop()

		pending = false
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-wakeup:
			if pending {
				continue
			}

			if wait := r.minGap - time.Since(lastRun); !lastRun.IsZero() && wait > 0 {
				deferred.Reset(wait)

				pending = true

				continue
			}

			reconcile()
		case <-deferred.C:
			pending = false

			reconcile()
		case <-r.kick:
			r.flushPurges(ctx)
		case <-tick.C:
			cancelPending()
			r.purgeLeftovers(ctx)
			r.flushPurges(ctx)
			reconcile()

			for _, f := range onTick {
				f(ctx)
			}
		}
	}
}

func (r *Registry) snapshotHolders() []Holder {
	r.mu.Lock()
	defer r.mu.Unlock()

	hs := make([]Holder, 0, len(r.holders))
	for _, h := range r.holders {
		hs = append(hs, h)
	}

	return hs
}

func (r *Registry) holder(regType string) Holder {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.holders[regType]
}

func (r *Registry) reconcileAll(ctx context.Context) {
	for _, h := range r.snapshotHolders() {
		h.ReconcileRegistrations(ctx)
	}
}

func (r *Registry) enqueuePurge(regType, imsi string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pending[purgeKey{regType: regType, imsi: imsi}] = struct{}{}

	select {
	case r.kick <- struct{}{}:
	default:
	}
}

func (r *Registry) flushPurges(ctx context.Context) {
	r.mu.Lock()
	keys := make([]purgeKey, 0, len(r.pending))

	for k := range r.pending {
		keys = append(keys, k)
	}
	r.mu.Unlock()

	if len(keys) == 0 || !r.purgePending(ctx, keys[0]) {
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	slots := make(chan struct{}, purgeConcurrency)

	for _, k := range keys[1:] {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
		}

		if ctx.Err() != nil {
			break
		}

		wg.Add(1)

		go func() {
			defer wg.Done()
			defer func() { <-slots }()

			if !r.purgePending(ctx, k) {
				cancel()
			}
		}()
	}

	wg.Wait()
}

func (r *Registry) purgePending(ctx context.Context, k purgeKey) bool {
	if err := r.purge(ctx, k); err != nil {
		if ctx.Err() == nil {
			r.log.Warn("failed to mark UE registration purged; will retry", logger.SUPIFromIMSI(k.imsi), zap.String("type", k.regType), zap.Error(err))
		}

		return false
	}

	r.mu.Lock()
	delete(r.pending, k)
	r.mu.Unlock()

	return true
}

func (r *Registry) purge(ctx context.Context, k purgeKey) error {
	defer r.locks.lock(k.imsi)()

	if h := r.holder(k.regType); h != nil && h.HoldsUE(k.imsi) {
		return nil
	}

	err := r.store.PurgeUERegistration(ctx, k.imsi, k.regType, r.nodeID)
	if errors.Is(err, db.ErrMigrationPending) {
		return nil
	}

	return err
}

type Binding struct {
	r         *Registry
	regType   string
	otherType string
}

func (b *Binding) Register(ctx context.Context, imsi string) (int64, error) {
	defer b.r.locks.lock(imsi)()

	version, err := b.r.store.RegisterUE(ctx, imsi, b.regType, b.r.nodeID, b.otherType)
	if errors.Is(err, db.ErrMigrationPending) {
		return 0, nil
	}

	return version, err
}

func (b *Binding) RegisterVisitor(ctx context.Context, imsi, partnerID string) (int64, error) {
	defer b.r.locks.lock(imsi)()

	version, err := b.r.store.RegisterVisitorUE(ctx, imsi, partnerID, b.regType, b.r.nodeID, b.otherType)
	if errors.Is(err, db.ErrMigrationPending) {
		return 0, nil
	}

	return version, err
}

func (b *Binding) Confirmed(ctx context.Context, imsi string, version int64) bool {
	if version == 0 {
		return false
	}

	reg, err := b.r.store.GetUERegistration(ctx, imsi, b.regType)
	if err != nil {
		return false
	}

	return reg.NodeID == b.r.nodeID && !reg.Purged && reg.Version != 0
}

func (b *Binding) Purge(imsi string) {
	b.r.enqueuePurge(b.regType, imsi)
}

func (b *Binding) Reconcile(ctx context.Context, imsi string, visitor bool, held func() int64, release func(context.Context), withdraw func(context.Context, Withdrawal)) {
	defer b.r.locks.lock(imsi)()

	switch o, why := b.outcome(ctx, imsi, visitor, held()); o {
	case outcomeSuperseded:
		release(ctx)
	case outcomeWithdrawn:
		withdraw(ctx, why)
	}
}

type outcome int

const (
	outcomeKept outcome = iota
	outcomeSuperseded
	outcomeWithdrawn
)

func (b *Binding) outcome(ctx context.Context, imsi string, visitor bool, held int64) (outcome, Withdrawal) {
	if held == 0 {
		return outcomeKept, 0
	}

	own, err := b.r.store.GetUERegistration(ctx, imsi, b.regType)
	if errors.Is(err, db.ErrNotFound) && b.subscriberDeleted(ctx, imsi) {
		return outcomeWithdrawn, SubscriberDeleted
	}

	if err != nil {
		return outcomeKept, 0
	}

	if own.NodeID != b.r.nodeID {
		if !own.Purged && own.Version > held {
			return outcomeSuperseded, 0
		}

		return outcomeKept, 0
	}

	if !own.Purged {
		if !visitor {
			return outcomeKept, 0
		}

		return b.roamingOutcome(ctx, imsi)
	}

	other, err := b.r.store.GetUERegistration(ctx, imsi, b.otherType)
	if err == nil && !other.Purged && other.NodeID == b.r.nodeID {
		return outcomeKept, 0
	}

	return outcomeSuperseded, 0
}

func (b *Binding) roamingOutcome(ctx context.Context, imsi string) (outcome, Withdrawal) {
	v, err := b.r.store.GetVisitor(ctx, imsi)
	if err != nil {
		return outcomeKept, 0
	}

	now := time.Now()

	switch {
	case v.Deregistering(now):
		return outcomeWithdrawn, RoamerDeregistered
	case v.Withdrawing(now):
		return outcomeWithdrawn, RoamingWithdrawn
	}

	return outcomeKept, 0
}

func (b *Binding) subscriberDeleted(ctx context.Context, imsi string) bool {
	_, err := b.r.store.GetSubscriberOrVisitor(ctx, imsi)

	return errors.Is(err, db.ErrNotFound)
}

type keyedEntry struct {
	mu   sync.Mutex
	refs int
}

type keyedMutex struct {
	mu      sync.Mutex
	entries map[string]*keyedEntry
}

func (k *keyedMutex) lock(key string) func() {
	k.mu.Lock()

	e, ok := k.entries[key]
	if !ok {
		e = &keyedEntry{}
		k.entries[key] = e
	}

	e.refs++
	k.mu.Unlock()

	e.mu.Lock()

	return func() {
		e.mu.Unlock()

		k.mu.Lock()

		e.refs--
		if e.refs == 0 {
			delete(k.entries, key)
		}

		k.mu.Unlock()
	}
}
