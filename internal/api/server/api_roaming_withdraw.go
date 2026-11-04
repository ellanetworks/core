// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"errors"
	"fmt"
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

func applyWithdrawingRoamers(ctx context.Context, dbInstance *db.Database, existing db.RoamingPartner, updated *db.RoamingPartner, apply func(context.Context) error) error {
	err := apply(ctx)
	if !errors.Is(err, db.ErrRoamersAttached) {
		return err
	}

	timing := roamerWithdrawalTiming
	withdrawals := db.RoamingWithdrawalsFor(existing, updated)

	if len(withdrawals) == 0 {
		return err
	}

	if err := dbInstance.WithdrawRoamers(ctx, existing.ID, withdrawals, timing.timeout+timing.margin); err != nil {
		return fmt.Errorf("withdraw the roamers of roaming partner %s: %w", existing.Name, err)
	}

	logger.APILog.Info("deregistering the roamers of a roaming partner before changing it",
		zap.String("roaming_partner", existing.Name), zap.Int("scopes", len(withdrawals)))

	deadline := time.Now().Add(timing.timeout)

	for {
		remaining, err := dbInstance.CountWithdrawnRoamers(ctx, existing.ID)
		if err != nil {
			clearWithdrawals(ctx, dbInstance, existing)
			return err
		}

		if remaining == 0 {
			break
		}

		if time.Now().After(deadline) {
			clearWithdrawals(ctx, dbInstance, existing)
			return fmt.Errorf("%w: %d roamers are still registered (a node that holds them may be unreachable); retry later", db.ErrRoamersAttached, remaining)
		}

		select {
		case <-ctx.Done():
			clearWithdrawals(ctx, dbInstance, existing)
			return ctx.Err()
		case <-time.After(timing.poll):
		}
	}

	if err := apply(ctx); err != nil {
		clearWithdrawals(ctx, dbInstance, existing)
		return err
	}

	return nil
}

func clearWithdrawals(ctx context.Context, dbInstance *db.Database, partner db.RoamingPartner) {
	if err := dbInstance.ClearRoamingWithdrawals(context.WithoutCancel(ctx), partner.ID); err != nil && !errors.Is(err, db.ErrNotFound) {
		logger.APILog.Warn("failed to clear the roamer withdrawal of a roaming partner; it expires on its own",
			zap.String("roaming_partner", partner.Name), zap.Error(err))
	}
}
