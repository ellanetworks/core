// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runOperatorNASSecurityMatrix round-trips the ciphering and integrity
// algorithm preference orders. Valid values: NULL, SNOW3G, AES for both
// ciphering and integrity. Max 3 each, no duplicates.
func runOperatorNASSecurityMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	baseline := &client.UpdateOperatorNASSecurityOptions{
		Ciphering: []string{"NULL", "SNOW3G", "AES"},
		Integrity: []string{"SNOW3G", "AES"},
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
				Ciphering: []string{"AES", "SNOW3G", "NULL"},
				Integrity: baseline.Integrity,
			},
		},
		{
			field: "integrity_order",
			opts: &client.UpdateOperatorNASSecurityOptions{
				Ciphering: baseline.Ciphering,
				Integrity: []string{"AES", "SNOW3G", "NULL"},
			},
		},
		{
			field: "single_algorithm",
			opts: &client.UpdateOperatorNASSecurityOptions{
				Ciphering: []string{"NULL"},
				Integrity: []string{"NULL"},
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
