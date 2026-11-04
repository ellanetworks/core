// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"errors"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type roamerWithdrawal struct {
	timeout time.Duration
	poll    time.Duration
	margin  time.Duration
}

var roamerWithdrawalTiming = roamerWithdrawal{timeout: time.Minute, poll: 250 * time.Millisecond, margin: 30 * time.Second}

var errRoamersStillRegistered = errors.New("roamers are still registered")

func withdrawRoamers(ctx context.Context, mark func(context.Context, time.Time) error, remaining func(context.Context) (int, error)) error {
	timing := roamerWithdrawalTiming

	if err := mark(ctx, time.Now().Add(timing.timeout+timing.margin)); err != nil {
		return err
	}

	deadline := time.Now().Add(timing.timeout)

	for {
		n, err := remaining(ctx)
		if err != nil || n == 0 {
			return err
		}

		if time.Now().After(deadline) {
			return errRoamersStillRegistered
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(timing.poll):
		}
	}
}

func clearWithdrawal(ctx context.Context, mark func(context.Context, time.Time) error, what string) {
	if err := mark(context.WithoutCancel(ctx), time.Time{}); err != nil && !errors.Is(err, db.ErrNotFound) {
		logger.APILog.Warn("failed to clear a roaming withdrawal; it expires on its own", zap.String("of", what), zap.Error(err))
	}
}
