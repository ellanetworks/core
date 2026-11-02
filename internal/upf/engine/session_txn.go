// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type sessionTxn struct {
	undo []func() error
}

func (t *sessionTxn) onRollback(f func() error) {
	t.undo = append(t.undo, f)
}

func (t *sessionTxn) rollback(ctx context.Context) error {
	var errs []error

	for i := len(t.undo) - 1; i >= 0; i-- {
		if err := t.undo[i](); err != nil {
			logger.From(ctx, logger.UpfLog).Warn("session rollback step failed", zap.Error(err))
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
