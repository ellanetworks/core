// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"database/sql"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

const FsmStateTableName = "fsm_state"

// ensureFsmStateTable creates the local-only fsm_state singleton and seeds
// it with lastApplied = 0. The table tracks the Raft index of the last
// successfully applied log entry so that crash-recovery replay can skip
// already-applied entries (changesets are not idempotent).
func ensureFsmStateTable(ctx context.Context, conn *sql.DB) error {
	return ellaraft.EnsureFSMStateSchema(ctx, conn)
}
