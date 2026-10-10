// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"slices"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runOperatorIMSHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	nodes := h.Clients

	t.Cleanup(func() {
		if err := h.Leader.UpdateOperatorIMS(ctx, &client.UpdateOperatorIMSOptions{}); err != nil {
			t.Logf("cleanup: clear P-CSCF addresses: %v", err)
		}
	})

	cases := []struct {
		name      string
		writer    int
		addresses []string
	}{
		{name: "set", writer: 0, addresses: []string{"192.0.2.20", "2001:db8::20"}},
		{name: "clear", writer: 1, addresses: []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := nodes[tc.writer].UpdateOperatorIMS(ctx, &client.UpdateOperatorIMSOptions{PCSCFAddresses: tc.addresses}); err != nil {
				t.Fatalf("update IMS on node %d: %v", tc.writer+1, err)
			}

			awaitConvergence(ctx, t, h)

			for i, c := range nodes {
				op, err := c.GetOperator(ctx)
				if err != nil {
					t.Fatalf("node %d get operator after update: %v", i+1, err)
				}

				if !slices.Equal(op.IMS.PCSCFAddresses, tc.addresses) {
					t.Fatalf("node %d pcscfAddresses: got %v, want %v", i+1, op.IMS.PCSCFAddresses, tc.addresses)
				}
			}
		})
	}
}
