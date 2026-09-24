// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	initializerInitialBackoff = 500 * time.Millisecond
	initializerMaxBackoff     = 30 * time.Second
)

func (db *Database) runStandaloneInitializer(ctx context.Context) {
	backoff := initializerInitialBackoff

	for {
		err := db.Initialize(ctx)
		if err == nil {
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
