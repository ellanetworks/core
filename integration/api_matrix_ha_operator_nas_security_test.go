// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runOperatorNASSecurityHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients

	baseline := &client.UpdateOperatorNASSecurityOptions{
		Ciphering: []string{"NULL", "SNOW3G", "AES"},
		Integrity: []string{"SNOW3G", "AES"},
	}

	if err := h.Leader.UpdateOperatorNASSecurity(ctx, baseline); err != nil {
		t.Fatalf("set nas security baseline: %v", err)
	}

	t.Cleanup(func() {
		if err := h.Leader.UpdateOperatorNASSecurity(ctx, baseline); err != nil {
			t.Logf("cleanup: restore nas security: %v", err)
		}
	})

	awaitConvergence(ctx, t, h)

	cases := []struct {
		field  string
		writer int
		opts   *client.UpdateOperatorNASSecurityOptions
	}{
		{
			field:  "ciphering_order",
			writer: 0,
			opts: &client.UpdateOperatorNASSecurityOptions{
				Ciphering: []string{"AES", "SNOW3G", "NULL"},
				Integrity: baseline.Integrity,
			},
		},
		{
			field:  "integrity_order",
			writer: 1,
			opts: &client.UpdateOperatorNASSecurityOptions{
				Ciphering: baseline.Ciphering,
				Integrity: []string{"AES", "SNOW3G", "NULL"},
			},
		},
		{
			field:  "single_algorithm",
			writer: 2,
			opts: &client.UpdateOperatorNASSecurityOptions{
				Ciphering: []string{"NULL"},
				Integrity: []string{"NULL"},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("update_"+tc.field, func(t *testing.T) {
			if err := nodes[tc.writer].UpdateOperatorNASSecurity(ctx, tc.opts); err != nil {
				t.Fatalf("update nas security on node %d: %v", tc.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				op, err := c.GetOperator(ctx)
				if err != nil {
					t.Fatalf("node %d get operator after update: %v", i+1, err)
				}

				Assert(t, equalStringSlices(op.NASSecurity.Ciphering, tc.opts.Ciphering), fmt.Sprintf("node %d Ciphering: got %v, want %v", i+1, op.NASSecurity.Ciphering, tc.opts.Ciphering))

				Assert(t, equalStringSlices(op.NASSecurity.Integrity, tc.opts.Integrity), fmt.Sprintf("node %d Integrity: got %v, want %v", i+1, op.NASSecurity.Integrity, tc.opts.Integrity))
			}
		})
	}
}
