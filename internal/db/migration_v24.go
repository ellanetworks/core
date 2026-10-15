// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV24(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS positioning_sessions"); err != nil {
		return fmt.Errorf("drop positioning_sessions: %w", err)
	}

	stmt := fmt.Sprintf(`CREATE TABLE %s (
		id                     TEXT    PRIMARY KEY,
		supi                   TEXT    NOT NULL CHECK (supi != ''),
		method                 TEXT    NOT NULL CHECK (method != ''),
		mode                   TEXT,
		measurement_quantities TEXT,
		reporting_interval_s   INTEGER NOT NULL CHECK (reporting_interval_s BETWEEN 1 AND 8639999),
		reporting_amount       INTEGER NOT NULL CHECK (reporting_amount BETWEEN 1 AND 8639999),
		reporting_infinite     INTEGER NOT NULL DEFAULT 0 CHECK (reporting_infinite IN (0, 1)),
		scheduled_loc_time     INTEGER,
		status                 TEXT    NOT NULL CHECK (status IN ('ACTIVE', 'TERMINATED')),
		termination_cause      TEXT    CHECK (termination_cause IN ('NORMAL_TERMINATION', 'TERMINATION_BY_UE', 'TERMINATION_BY_NETWORK')),
		termination_reason     TEXT,
		created_at             INTEGER NOT NULL,
		terminated_at          INTEGER,
		CHECK (reporting_infinite = 1 OR reporting_amount * reporting_interval_s <= 8639999),
		CHECK ((status = 'ACTIVE') = (termination_cause IS NULL)),
		CHECK ((status = 'ACTIVE') = (terminated_at IS NULL)),
		CHECK (termination_reason IS NULL OR termination_cause IS NOT NULL)
	)`, DeferredLocationRequestsTableName)

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create %s: %w", DeferredLocationRequestsTableName, err)
	}

	for _, idx := range []string{
		fmt.Sprintf("CREATE INDEX idx_deferred_location_requests_supi ON %s (supi)", DeferredLocationRequestsTableName),
		fmt.Sprintf("CREATE INDEX idx_deferred_location_requests_terminated_at ON %s (terminated_at)", DeferredLocationRequestsTableName),
	} {
		if _, err := tx.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("create index on %s: %w", DeferredLocationRequestsTableName, err)
		}
	}

	stmt = fmt.Sprintf(`CREATE TABLE %s (
		request_id    TEXT    NOT NULL REFERENCES %s(id) ON DELETE CASCADE,
		seq           INTEGER NOT NULL CHECK (seq >= 1),
		reported_at   INTEGER NOT NULL,
		location_data TEXT    NOT NULL,
		PRIMARY KEY (request_id, seq)
	)`, LocationReportsTableName, DeferredLocationRequestsTableName)

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create %s: %w", LocationReportsTableName, err)
	}

	return nil
}
