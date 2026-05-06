// Copyright 2026 Ella Networks

package db

import (
	"context"
)

// emptyPayload is the payload type for replicated ops that carry no
// parameters (bulk deletes that operate on whole tables, counter
// allocations). Kept as a distinct named type so forwarded ops can
// round-trip through JSON unambiguously.
type emptyPayload struct{}

// Wrappers adapt apply functions whose native signatures do not match
// the ChangesetOp[P] contract func(db, ctx, *P) (any, error).

func (db *Database) applyClearDailyUsageOp(ctx context.Context, _ *emptyPayload) (any, error) {
	return nil, db.applyClearDailyUsage(ctx)
}

func (db *Database) applyDeleteAllSessionsOp(ctx context.Context, _ *emptyPayload) (any, error) {
	return nil, db.applyDeleteAllSessions(ctx)
}
