// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ueregistration

import (
	"context"
	"errors"
	"hash/fnv"
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const stripes = 256

type Store interface {
	RegisterUE(ctx context.Context, imsi, regType, nodeID string, cancel ...string) error
	PurgeUERegistration(ctx context.Context, imsi, regType, nodeID string) error
	GetUERegistration(ctx context.Context, imsi, regType string) (*db.UERegistration, error)
}

type Registry struct {
	store     Store
	nodeID    string
	regType   string
	otherType string
	log       *zap.Logger
	locks     [stripes]sync.Mutex
}

func New(store Store, nodeID, regType, otherType string, log *zap.Logger) *Registry {
	return &Registry{
		store:     store,
		nodeID:    nodeID,
		regType:   regType,
		otherType: otherType,
		log:       log,
	}
}

func (r *Registry) NodeID() string {
	return r.nodeID
}

func (r *Registry) lock(imsi string) func() {
	h := fnv.New32a()
	_, _ = h.Write([]byte(imsi))
	mu := &r.locks[h.Sum32()%stripes]
	mu.Lock()

	return mu.Unlock
}

func (r *Registry) Register(ctx context.Context, imsi string) error {
	defer r.lock(imsi)()

	return r.register(ctx, imsi)
}

func (r *Registry) register(ctx context.Context, imsi string) error {
	err := r.store.RegisterUE(ctx, imsi, r.regType, r.nodeID, r.otherType)
	if errors.Is(err, db.ErrMigrationPending) {
		return nil
	}

	return err
}

func (r *Registry) Purge(ctx context.Context, imsi string, stillAbsent func() bool) {
	defer r.lock(imsi)()

	if !stillAbsent() {
		return
	}

	err := r.store.PurgeUERegistration(ctx, imsi, r.regType, r.nodeID)
	if err != nil && !errors.Is(err, db.ErrMigrationPending) {
		r.log.Warn("failed to mark UE registration purged", logger.SUPIFromIMSI(imsi), zap.String("type", r.regType), zap.Error(err))
	}
}

func (r *Registry) Superseded(ctx context.Context, imsi string, registeredAt int64) bool {
	defer r.lock(imsi)()

	reg, err := r.store.GetUERegistration(ctx, imsi, r.regType)
	if errors.Is(err, db.ErrNotFound) {
		if err := r.register(ctx, imsi); err != nil {
			r.log.Warn("failed to register UE", logger.SUPIFromIMSI(imsi), zap.String("type", r.regType), zap.Error(err))
		}

		return false
	}

	if err != nil {
		r.log.Debug("could not read UE registration", logger.SUPIFromIMSI(imsi), zap.String("type", r.regType), zap.Error(err))
		return false
	}

	if reg.NodeID != r.nodeID {
		return reg.RegistrationTime > registeredAt
	}

	if !reg.Purged {
		return false
	}

	other, err := r.store.GetUERegistration(ctx, imsi, r.otherType)
	if err != nil {
		return false
	}

	return !other.Purged && other.NodeID != r.nodeID && other.RegistrationTime > registeredAt
}
