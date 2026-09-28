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
	defaultBackstop   = 5 * time.Minute
	defaultPurgeRetry = 10 * time.Second
)

type Store interface {
	RegisterUE(ctx context.Context, imsi, regType, nodeID string, cancel ...string) (int64, error)
	PurgeUERegistration(ctx context.Context, imsi, regType, nodeID string) error
	GetUERegistration(ctx context.Context, imsi, regType string) (*db.UERegistration, error)
	ListUERegistrationsSince(ctx context.Context, version int64) ([]db.UERegistration, error)
	MaxUERegistrationVersion(ctx context.Context) (int64, error)
}

type Holder interface {
	HoldsUE(imsi string) bool
	ReconcileRegistration(ctx context.Context, imsi string)
	ReconcileRegistrations(ctx context.Context)
}

type purgeKey struct {
	regType string
	imsi    string
}

type Registry struct {
	store  Store
	nodeID string
	log    *zap.Logger
	locks  keyedMutex

	backstop   time.Duration
	purgeRetry time.Duration

	mu      sync.Mutex
	holders map[string]Holder
	pending map[purgeKey]struct{}
	kick    chan struct{}
}

func New(store Store, nodeID string, log *zap.Logger) *Registry {
	return &Registry{
		store:      store,
		nodeID:     nodeID,
		log:        log,
		locks:      keyedMutex{entries: map[string]*keyedEntry{}},
		backstop:   defaultBackstop,
		purgeRetry: defaultPurgeRetry,
		holders:    map[string]Holder{},
		pending:    map[purgeKey]struct{}{},
		kick:       make(chan struct{}, 1),
	}
}

func (r *Registry) Bind(regType, otherType string, h Holder) *Binding {
	r.mu.Lock()
	r.holders[regType] = h
	r.mu.Unlock()

	return &Binding{r: r, regType: regType, otherType: otherType}
}

func (r *Registry) Run(ctx context.Context, wakeup <-chan struct{}) {
	backstop := time.NewTicker(r.backstop)
	defer backstop.Stop()

	retry := time.NewTicker(r.purgeRetry)
	defer retry.Stop()

	watermark, err := r.store.MaxUERegistrationVersion(ctx)
	if err != nil {
		watermark = 0
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-wakeup:
			watermark = r.reconcileChanges(ctx, watermark)
		case <-r.kick:
			r.flushPurges(ctx)
		case <-retry.C:
			r.flushPurges(ctx)
		case <-backstop.C:
			watermark = r.reconcileChanges(ctx, watermark)
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

func (r *Registry) reconcileChanges(ctx context.Context, watermark int64) int64 {
	rows, err := r.store.ListUERegistrationsSince(ctx, watermark)
	if err != nil {
		r.log.Debug("could not list UE registration changes", zap.Error(err))
		return watermark
	}

	seen := map[string]struct{}{}
	holders := r.snapshotHolders()

	for _, row := range rows {
		if row.Version > watermark {
			watermark = row.Version
		}

		if _, done := seen[row.Imsi]; done {
			continue
		}

		seen[row.Imsi] = struct{}{}

		for _, h := range holders {
			h.ReconcileRegistration(ctx, row.Imsi)
		}
	}

	return watermark
}

func (r *Registry) reconcileAll(ctx context.Context) {
	for _, h := range r.snapshotHolders() {
		h.ReconcileRegistrations(ctx)
	}
}

func (r *Registry) enqueuePurge(regType, imsi string) {
	r.mu.Lock()
	r.pending[purgeKey{regType: regType, imsi: imsi}] = struct{}{}
	r.mu.Unlock()

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

	for _, k := range keys {
		if r.purge(ctx, k) {
			r.mu.Lock()
			delete(r.pending, k)
			r.mu.Unlock()
		}
	}
}

func (r *Registry) purge(ctx context.Context, k purgeKey) bool {
	defer r.locks.lock(k.imsi)()

	if h := r.holder(k.regType); h != nil && h.HoldsUE(k.imsi) {
		return true
	}

	err := r.store.PurgeUERegistration(ctx, k.imsi, k.regType, r.nodeID)
	if err == nil || errors.Is(err, db.ErrMigrationPending) {
		return true
	}

	r.log.Warn("failed to mark UE registration purged; will retry", logger.SUPIFromIMSI(k.imsi), zap.String("type", k.regType), zap.Error(err))

	return false
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

	return reg.NodeID == b.r.nodeID && !reg.Purged
}

func (b *Binding) Purge(imsi string) {
	b.r.enqueuePurge(b.regType, imsi)
}

func (b *Binding) Reconcile(ctx context.Context, imsi string, held func() int64, release func(context.Context)) {
	defer b.r.locks.lock(imsi)()

	if b.superseded(ctx, imsi, held()) {
		release(ctx)
	}
}

func (b *Binding) superseded(ctx context.Context, imsi string, held int64) bool {
	if held == 0 {
		return false
	}

	own, err := b.r.store.GetUERegistration(ctx, imsi, b.regType)
	if err != nil {
		return false
	}

	if own.NodeID != b.r.nodeID {
		return !own.Purged && own.Version > held
	}

	if !own.Purged {
		return false
	}

	other, err := b.r.store.GetUERegistration(ctx, imsi, b.otherType)
	if err == nil && !other.Purged && other.NodeID == b.r.nodeID {
		return false
	}

	return true
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
