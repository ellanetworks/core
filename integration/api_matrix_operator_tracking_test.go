package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runOperatorTrackingMatrix sets a known baseline before mutating since
// the operator may be freshly initialised with no TACs. Cleanup restores
// the same baseline. TAC values are 6-character hex strings.
func runOperatorTrackingMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	baseline := []string{"000001"}

	if err := c.UpdateOperatorTracking(ctx, &client.UpdateOperatorTrackingOptions{SupportedTacs: baseline}); err != nil {
		t.Fatalf("set operator tracking baseline: %v", err)
	}

	t.Cleanup(func() {
		if err := c.UpdateOperatorTracking(ctx, &client.UpdateOperatorTrackingOptions{SupportedTacs: baseline}); err != nil {
			t.Logf("cleanup: restore operator tracking: %v", err)
		}
	})

	cases := []struct {
		field string
		tacs  []string
	}{
		{field: "single", tacs: []string{"00000a"}},
		{field: "multiple", tacs: []string{"000010", "000020", "000030"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			if err := c.UpdateOperatorTracking(ctx, &client.UpdateOperatorTrackingOptions{SupportedTacs: tc.tacs}); err != nil {
				t.Fatalf("update operator tracking: %v", err)
			}

			op, err := c.GetOperator(ctx)
			if err != nil {
				t.Fatalf("get operator after update: %v", err)
			}

			if !equalStringSlices(op.Tracking.SupportedTacs, tc.tacs) {
				t.Fatalf("SupportedTacs: got %v, want %v", op.Tracking.SupportedTacs, tc.tacs)
			}
		})
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
