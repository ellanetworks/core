// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	initializerInitialBackoff = 500 * time.Millisecond
	initializerMaxBackoff     = 30 * time.Second
)

func (db *Database) runStandaloneInitializer(ctx context.Context) {
	if !db.retryStandaloneInitialization(ctx) {
		return
	}

	ticker := time.NewTicker(migrationGateTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-db.migrationCheckCh:
		case <-ticker.C:
		}

		if db.standaloneInitialized(ctx) {
			continue
		}

		if !db.retryStandaloneInitialization(ctx) {
			return
		}
	}
}

func (db *Database) standaloneInitialized(ctx context.Context) bool {
	applied, err := db.CurrentSchemaVersion(ctx)
	if err != nil {
		return false
	}

	return applied >= SchemaVersion() && db.IsOperatorInitialized(ctx)
}

func (db *Database) retryStandaloneInitialization(ctx context.Context) bool {
	backoff := initializerInitialBackoff

	for {
		err := db.initializeStandaloneLeader(ctx)
		if err == nil {
			return true
		}

		if ctx.Err() != nil {
			return false
		}

		logger.DBLog.Warn("standalone leader initialization failed; retrying",
			zap.Error(err),
			zap.Duration("next_backoff", backoff))

		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > initializerMaxBackoff {
			backoff = initializerMaxBackoff
		}
	}
}

func (db *Database) initializeStandaloneLeader(ctx context.Context) error {
	if err := db.CheckPendingMigrations(ctx); err != nil {
		return fmt.Errorf("propose pending schema migrations: %w", err)
	}

	applied, err := db.CurrentSchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	if applied < SchemaVersion() {
		return fmt.Errorf("shared schema still at %d, want %d: migrations pending", applied, SchemaVersion())
	}

	return db.Initialize(ctx)
}
