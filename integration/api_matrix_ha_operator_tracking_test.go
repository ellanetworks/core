package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runOperatorTrackingHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients
	baseline := []string{"000001"}

	if err := h.Leader.UpdateOperatorTracking(ctx, &client.UpdateOperatorTrackingOptions{SupportedTacs: baseline}); err != nil {
		t.Fatalf("set operator tracking baseline: %v", err)
	}

	t.Cleanup(func() {
		if err := h.Leader.UpdateOperatorTracking(ctx, &client.UpdateOperatorTrackingOptions{SupportedTacs: baseline}); err != nil {
			t.Logf("cleanup: restore operator tracking: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	cases := []struct {
		field  string
		writer int
		tacs   []string
	}{
		{field: "single", writer: 0, tacs: []string{"00000a"}},
		{field: "multiple", writer: 1, tacs: []string{"000010", "000020", "000030"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			if err := nodes[tc.writer].UpdateOperatorTracking(ctx, &client.UpdateOperatorTrackingOptions{SupportedTacs: tc.tacs}); err != nil {
				t.Fatalf("update operator tracking on node %d: %v", tc.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				op, err := c.GetOperator(ctx)
				if err != nil {
					t.Fatalf("node %d get operator after update: %v", i+1, err)
				}

				if !equalStringSlices(op.Tracking.SupportedTacs, tc.tacs) {
					t.Fatalf("node %d SupportedTacs: got %v, want %v", i+1, op.Tracking.SupportedTacs, tc.tacs)
				}
			}
		})
	}
}
