package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runOperatorSPNHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients

	baseline := &client.UpdateOperatorSPNOptions{
		FullName:  "Ella Networks",
		ShortName: "Ella",
	}

	if err := h.Leader.UpdateOperatorSPN(ctx, baseline); err != nil {
		t.Fatalf("set spn baseline: %v", err)
	}

	t.Cleanup(func() {
		if err := h.Leader.UpdateOperatorSPN(ctx, baseline); err != nil {
			t.Logf("cleanup: restore spn: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	cases := []struct {
		field  string
		writer int
		opts   *client.UpdateOperatorSPNOptions
	}{
		{
			field:  "FullName",
			writer: 0,
			opts: &client.UpdateOperatorSPNOptions{
				FullName:  "Ella Networks 5G Private",
				ShortName: baseline.ShortName,
			},
		},
		{
			field:  "ShortName",
			writer: 1,
			opts: &client.UpdateOperatorSPNOptions{
				FullName:  baseline.FullName,
				ShortName: "ELLA5G",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			if err := nodes[tc.writer].UpdateOperatorSPN(ctx, tc.opts); err != nil {
				t.Fatalf("update spn on node %d: %v", tc.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				op, err := c.GetOperator(ctx)
				if err != nil {
					t.Fatalf("node %d get operator after update: %v", i+1, err)
				}

				if op.SPN.FullName != tc.opts.FullName {
					t.Fatalf("node %d FullName: got %q, want %q", i+1, op.SPN.FullName, tc.opts.FullName)
				}

				if op.SPN.ShortName != tc.opts.ShortName {
					t.Fatalf("node %d ShortName: got %q, want %q", i+1, op.SPN.ShortName, tc.opts.ShortName)
				}
			}
		})
	}
}
