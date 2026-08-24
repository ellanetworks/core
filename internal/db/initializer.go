// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	initializerInitialBackoff = 500 * time.Millisecond
	initializerMaxBackoff     = 30 * time.Second
)

type standaloneInitializer struct {
	db        *Database
	parentCtx context.Context

	mu     sync.Mutex
	cancel context.CancelFunc
	done   bool
}

func newStandaloneInitializer(db *Database, parentCtx context.Context) *standaloneInitializer {
	return &standaloneInitializer{db: db, parentCtx: parentCtx}
}

func (s *standaloneInitializer) OnBecameLeader() {
	s.mu.Lock()

	if s.done {
		s.mu.Unlock()
		return
	}

	if s.cancel != nil {
		s.cancel()
	}

	ctx, cancel := context.WithCancel(s.parentCtx)
	s.cancel = cancel
	s.mu.Unlock()

	go s.run(ctx)
}

func (s *standaloneInitializer) OnLostLeadership() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (s *standaloneInitializer) run(ctx context.Context) {
	backoff := initializerInitialBackoff

	for {
		err := s.db.Initialize(ctx)
		if err == nil {
			s.mu.Lock()
			s.done = true
			s.mu.Unlock()

			return
		}

		if ctx.Err() != nil {
			return
		}

		logger.DBLog.Warn("database initialization failed; retrying",
			zap.Error(err),
			zap.Duration("next_backoff", backoff))

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > initializerMaxBackoff {
			backoff = initializerMaxBackoff
		}
	}
}
