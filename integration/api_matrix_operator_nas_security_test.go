// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runOperatorNASSecurityMatrix round-trips the ciphering and integrity
// algorithm preference orders. Valid values: NEA0/1/2 for ciphering,
// NIA0/1/2 for integrity. Max 3 each, no duplicates.
func runOperatorNASSecurityMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	baseline := &client.UpdateOperatorNASSecurityOptions{
		Ciphering: []string{"NEA0", "NEA1", "NEA2"},
		Integrity: []string{"NIA1", "NIA2"},
	}

	if err := c.UpdateOperatorNASSecurity(ctx, baseline); err != nil {
		t.Fatalf("set nas security baseline: %v", err)
	}

	t.Cleanup(func() {
		if err := c.UpdateOperatorNASSecurity(ctx, baseline); err != nil {
			t.Logf("cleanup: restore nas security: %v", err)
		}
	})

	cases := []struct {
		field string
		opts  *client.UpdateOperatorNASSecurityOptions
	}{
		{
			field: "ciphering_order",
			opts: &client.UpdateOperatorNASSecurityOptions{
				Ciphering: []string{"NEA2", "NEA1", "NEA0"},
				Integrity: baseline.Integrity,
			},
		},
		{
			field: "integrity_order",
			opts: &client.UpdateOperatorNASSecurityOptions{
				Ciphering: baseline.Ciphering,
				Integrity: []string{"NIA2", "NIA1", "NIA0"},
			},
		},
		{
			field: "single_algorithm",
			opts: &client.UpdateOperatorNASSecurityOptions{
				Ciphering: []string{"NEA0"},
				Integrity: []string{"NIA0"},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			if err := c.UpdateOperatorNASSecurity(ctx, tc.opts); err != nil {
				t.Fatalf("update nas security: %v", err)
			}

			op, err := c.GetOperator(ctx)
			if err != nil {
				t.Fatalf("get operator after update: %v", err)
			}

			Assert(t, equalStringSlices(op.NASSecurity.Ciphering, tc.opts.Ciphering), fmt.Sprintf("Ciphering: got %v, want %v", op.NASSecurity.Ciphering, tc.opts.Ciphering))

			Assert(t, equalStringSlices(op.NASSecurity.Integrity, tc.opts.Integrity), fmt.Sprintf("Integrity: got %v, want %v", op.NASSecurity.Integrity, tc.opts.Integrity))
		})
	}
}
