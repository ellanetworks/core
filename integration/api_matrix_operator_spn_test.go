package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runOperatorSPNMatrix round-trips the operator's full and short Service
// Provider Names. Each name is capped at 50 characters by the server.
func runOperatorSPNMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	baseline := &client.UpdateOperatorSPNOptions{
		FullName:  "Ella Networks",
		ShortName: "Ella",
	}

	if err := c.UpdateOperatorSPN(ctx, baseline); err != nil {
		t.Fatalf("set spn baseline: %v", err)
	}

	t.Cleanup(func() {
		if err := c.UpdateOperatorSPN(ctx, baseline); err != nil {
			t.Logf("cleanup: restore spn: %v", err)
		}
	})

	cases := []struct {
		field string
		opts  *client.UpdateOperatorSPNOptions
	}{
		{
			field: "FullName",
			opts: &client.UpdateOperatorSPNOptions{
				FullName:  "Ella Networks 5G Private",
				ShortName: baseline.ShortName,
			},
		},
		{
			field: "ShortName",
			opts: &client.UpdateOperatorSPNOptions{
				FullName:  baseline.FullName,
				ShortName: "ELLA5G",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			if err := c.UpdateOperatorSPN(ctx, tc.opts); err != nil {
				t.Fatalf("update spn: %v", err)
			}

			op, err := c.GetOperator(ctx)
			if err != nil {
				t.Fatalf("get operator after update: %v", err)
			}

			if op.SPN.FullName != tc.opts.FullName {
				t.Fatalf("FullName: got %q, want %q", op.SPN.FullName, tc.opts.FullName)
			}

			if op.SPN.ShortName != tc.opts.ShortName {
				t.Fatalf("ShortName: got %q, want %q", op.SPN.ShortName, tc.opts.ShortName)
			}
		})
	}
}
