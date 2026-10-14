// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package apimatrix

import (
	"context"
	"testing"
)

func stabilizeLocal(t *testing.T, ctx context.Context, h *HAEnv) {
	t.Helper()

	idx, err := leaderAppliedIndex(ctx, h.Leader)
	if err != nil {
		t.Fatalf("stabilizeLocal: read leader applied index: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, h.Clients, idx); err != nil {
		t.Fatalf("stabilizeLocal: nodes did not converge on leader index %d: %v", idx, err)
	}
}

// haMatrixEnv holds the three clients of a healthy 3-node cluster plus
// a resolved leader reference. The cluster is brought up once per
// TestAPIMatrixHA invocation; each registered resource runs as a
// subtest against the same env.
// setupHAMatrixEnv stages a 3-node HA cluster, waits for readiness, and
// resolves the current leader. Compose teardown and diagnostics are
// registered on t.
func awaitConvergence(ctx context.Context, t *testing.T, h *HAEnv) {
	t.Helper()

	idx, err := leaderAppliedIndex(ctx, h.Leader)
	if err != nil {
		t.Fatalf("leader applied index: %v", err)
	}

	if err := waitForFollowerConvergence(ctx, h.Clients, idx); err != nil {
		t.Fatalf("followers did not converge: %v", err)
	}
}
