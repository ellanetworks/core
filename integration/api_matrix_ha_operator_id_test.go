package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runOperatorIDHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients

	const (
		baselineMCC = "001"
		baselineMNC = "01"
	)

	if err := h.Leader.UpdateOperatorID(ctx, &client.UpdateOperatorIDOptions{Mcc: baselineMCC, Mnc: baselineMNC}); err != nil {
		t.Fatalf("set operator id baseline: %v", err)
	}

	t.Cleanup(func() {
		if err := h.Leader.UpdateOperatorID(ctx, &client.UpdateOperatorIDOptions{Mcc: baselineMCC, Mnc: baselineMNC}); err != nil {
			t.Logf("cleanup: restore operator id: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	cases := []struct {
		field  string
		writer int
		mcc    string
		mnc    string
	}{
		{field: "Mcc", writer: 0, mcc: "208", mnc: baselineMNC},
		{field: "Mnc", writer: 1, mcc: "208", mnc: "10"},
		{field: "Mnc_3digit", writer: 2, mcc: "208", mnc: "100"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			if err := nodes[tc.writer].UpdateOperatorID(ctx, &client.UpdateOperatorIDOptions{Mcc: tc.mcc, Mnc: tc.mnc}); err != nil {
				t.Fatalf("update operator id on node %d: %v", tc.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				op, err := c.GetOperator(ctx)
				if err != nil {
					t.Fatalf("node %d get operator after update: %v", i+1, err)
				}

				if op.ID.Mcc != tc.mcc {
					t.Fatalf("node %d Mcc: got %q, want %q", i+1, op.ID.Mcc, tc.mcc)
				}

				if op.ID.Mnc != tc.mnc {
					t.Fatalf("node %d Mnc: got %q, want %q", i+1, op.ID.Mnc, tc.mnc)
				}
			}
		})
	}
}
