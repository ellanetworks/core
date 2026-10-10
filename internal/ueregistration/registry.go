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

const defaultInterval = 10 * time.Second

type Store interface {
	RegisterUE(ctx context.Context, imsi, regType, nodeID, cancel string) (int64, error)
	PurgeUERegistration(ctx context.Context, imsi, regType, nodeID string) error
	GetUERegistration(ctx context.Context, imsi, regType string) (*db.UERegistration, error)
	GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error)
}

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

	mu      sync.Mutex
	holders map[string]Holder
	pending map[purgeKey]struct{}
}

func New(store Store, nodeID string, log *zap.Logger) *Registry {
	return &Registry{
		store:    store,
		nodeID:   nodeID,
		log:      log,
		locks:    keyedMutex{entries: map[string]*keyedEntry{}},
		interval: defaultInterval,
		holders:  map[string]Holder{},
		pending:  map[purgeKey]struct{}{},
	}
}

func (r *Registry) Bind(regType, otherType string, h Holder) *Binding {
	r.mu.Lock()
	r.holders[regType] = h
	r.mu.Unlock()

	return &Binding{r: r, regType: regType, otherType: otherType}
}

func (r *Registry) Run(ctx context.Context, wakeup <-chan struct{}) {
	tick := time.NewTicker(r.interval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wakeup:
			r.reconcileAll(ctx)
		case <-tick.C:
			r.flushPurges(ctx)
			r.reconcileAll(ctx)
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
}

func (r *Registry) flushPurges(ctx context.Context) {
	r.mu.Lock()
	keys := make([]purgeKey, 0, len(r.pending))

	for k := range r.pending {
		keys = append(keys, k)
	}
	r.mu.Unlock()

	for _, k := range keys {
		if err := r.purge(ctx, k); err != nil {
			r.log.Warn("failed to mark UE registration purged; will retry", logger.SUPIFromIMSI(k.imsi), zap.String("type", k.regType), zap.Error(err))
			return
		}

		r.mu.Lock()
		delete(r.pending, k)
		r.mu.Unlock()
	}
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

func (b *Binding) Reconcile(ctx context.Context, imsi string, held func() int64, release func(context.Context), withdraw func(context.Context)) {
	defer b.r.locks.lock(imsi)()

	switch b.outcome(ctx, imsi, held()) {
	case outcomeSuperseded:
		release(ctx)
	case outcomeWithdrawn:
		withdraw(ctx)
	}
}

type outcome int

const (
	outcomeKept outcome = iota
	outcomeSuperseded
	outcomeWithdrawn
)

func (b *Binding) outcome(ctx context.Context, imsi string, held int64) outcome {
	if held == 0 {
		return outcomeKept
	}

	own, err := b.r.store.GetUERegistration(ctx, imsi, b.regType)
	if errors.Is(err, db.ErrNotFound) && b.subscriberDeleted(ctx, imsi) {
		return outcomeWithdrawn
	}

	if err != nil {
		return outcomeKept
	}

	if own.NodeID != b.r.nodeID {
		if !own.Purged && own.Version > held {
			return outcomeSuperseded
		}

		return outcomeKept
	}

	if !own.Purged {
		return outcomeKept
	}

	other, err := b.r.store.GetUERegistration(ctx, imsi, b.otherType)
	if err == nil && !other.Purged && other.NodeID == b.r.nodeID {
		return outcomeKept
	}

	return outcomeSuperseded
}

func (b *Binding) subscriberDeleted(ctx context.Context, imsi string) bool {
	_, err := b.r.store.GetSubscriber(ctx, imsi)

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
