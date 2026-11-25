// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"fmt"
)

const SubscriberCountTableName = "subscriber_count"

const (
	createSubscriberCountSQL = "CREATE TABLE " + SubscriberCountTableName + " (singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton), total INTEGER NOT NULL)"
	seedSubscriberCountSQL   = "INSERT OR REPLACE INTO " + SubscriberCountTableName + " (singleton, total) VALUES (TRUE, (SELECT COUNT(*) FROM " + SubscribersTableName + ") + (SELECT COUNT(*) FROM " + VisitorsTableName + "))"
	getSubscriberCountSQL    = "SELECT total FROM " + SubscriberCountTableName + " WHERE singleton"
	addSubscriberCountSQL    = "UPDATE " + SubscriberCountTableName + " SET total = total + ? WHERE singleton"
)

func (db *Database) subscriberCount(ctx context.Context, q querier) (int, error) {
	count, err := queryCount(ctx, q, getSubscriberCountSQL)
	if err != nil {
		return 0, fmt.Errorf("read subscriber count: %w", err)
	}

	return count, nil
}

func (db *Database) addSubscriberCount(ctx context.Context, delta int64) error {
	if delta == 0 {
		return nil
	}

	res, err := db.runner(ctx).ExecContext(ctx, addSubscriberCountSQL, delta)
	if err != nil {
		return fmt.Errorf("update subscriber count: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update subscriber count: %w", err)
	}

	if n != 1 {
		return fmt.Errorf("update subscriber count: %d rows", n)
	}

	return nil
}
