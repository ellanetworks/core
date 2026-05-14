package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runOperatorIDMatrix exercises GET /api/v1/operator (for the ID
// section) and PUT /api/v1/operator/id, round-tripping MCC and MNC.
// Handler-side validators (api_operator.go:78-106) require MCC = 3
// digits and MNC = 2 or 3 digits.
//
// The matrix test sets a known baseline (001/01) before mutating so
// it works against both a fresh post-init operator (empty MCC/MNC) and
// a previously-configured one. The post-Cleanup state is the same known
// baseline regardless of pre-test state.
func runOperatorIDMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	const (
		baselineMCC = "001"
		baselineMNC = "01"
	)

	if err := c.UpdateOperatorID(ctx, &client.UpdateOperatorIDOptions{Mcc: baselineMCC, Mnc: baselineMNC}); err != nil {
		t.Fatalf("set operator id baseline: %v", err)
	}

	t.Cleanup(func() {
		if err := c.UpdateOperatorID(ctx, &client.UpdateOperatorIDOptions{Mcc: baselineMCC, Mnc: baselineMNC}); err != nil {
			t.Logf("cleanup: restore operator id: %v", err)
		}
	})

	cases := []struct {
		field string
		mcc   string
		mnc   string
	}{
		{field: "Mcc", mcc: "208", mnc: baselineMNC},
		{field: "Mnc", mcc: "208", mnc: "10"},
		{field: "Mnc_3digit", mcc: "208", mnc: "100"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			if err := c.UpdateOperatorID(ctx, &client.UpdateOperatorIDOptions{Mcc: tc.mcc, Mnc: tc.mnc}); err != nil {
				t.Fatalf("update operator id: %v", err)
			}

			op, err := c.GetOperator(ctx)
			if err != nil {
				t.Fatalf("get operator after update: %v", err)
			}

			if op.ID.Mcc != tc.mcc {
				t.Fatalf("Mcc: got %q, want %q", op.ID.Mcc, tc.mcc)
			}

			if op.ID.Mnc != tc.mnc {
				t.Fatalf("Mnc: got %q, want %q", op.ID.Mnc, tc.mnc)
			}
		})
	}
}
