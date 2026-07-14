// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// sessionTxn records the inverse of each datapath change applied during a
// session operation so a partial failure can be unwound. Without it, a session
// that fails before it is registered leaves eBPF entries and TEIDs that no later
// teardown can reach.
type sessionTxn struct {
	undo []func() error
}

func (t *sessionTxn) onRollback(f func() error) {
	t.undo = append(t.undo, f)
}

func (t *sessionTxn) rollback(ctx context.Context) {
	for i := len(t.undo) - 1; i >= 0; i-- {
		if err := t.undo[i](); err != nil {
			logger.WithTrace(ctx, logger.UpfLog).Warn("session rollback step failed", zap.Error(err))
		}
	}
}
