// Copyright 2026 Ella Networks

package db

import (
	"context"
	"fmt"
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

func (db *Database) applyAllocatePKISerialOp(ctx context.Context, _ *emptyPayload) (any, error) {
	return db.applyAllocateSerial(ctx)
}

// applyBootstrapPKIOp writes the three bootstrap rows (PKI state + root
// + intermediate) inside a single capture so either all three persist
// or none do.
func (db *Database) applyBootstrapPKIOp(ctx context.Context, p *PKIBootstrap) (any, error) {
	if _, err := db.applyInitPKIState(ctx, &ClusterPKIState{HMACKey: p.HMACKey}); err != nil {
		return nil, fmt.Errorf("init pki state: %w", err)
	}

	if _, err := db.applyInsertPKIRoot(ctx, p.Root); err != nil {
		return nil, fmt.Errorf("insert pki root: %w", err)
	}

	if _, err := db.applyInsertPKIIntermediate(ctx, p.Intermediate); err != nil {
		return nil, fmt.Errorf("insert pki intermediate: %w", err)
	}

	return nil, nil
}
