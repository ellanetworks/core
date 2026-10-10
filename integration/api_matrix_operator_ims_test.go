// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"slices"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runOperatorIMSMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	t.Cleanup(func() {
		if err := c.UpdateOperatorIMS(ctx, &client.UpdateOperatorIMSOptions{}); err != nil {
			t.Logf("cleanup: clear P-CSCF addresses: %v", err)
		}
	})

	cases := []struct {
		name      string
		addresses []string
	}{
		{name: "set", addresses: []string{"192.0.2.20", "2001:db8::20"}},
		{name: "reorder", addresses: []string{"2001:db8::20", "192.0.2.20"}},
		{name: "clear", addresses: []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := c.UpdateOperatorIMS(ctx, &client.UpdateOperatorIMSOptions{PCSCFAddresses: tc.addresses}); err != nil {
				t.Fatalf("update IMS: %v", err)
			}

			op, err := c.GetOperator(ctx)
			if err != nil {
				t.Fatalf("get operator after update: %v", err)
			}

			if !slices.Equal(op.IMS.PCSCFAddresses, tc.addresses) {
				t.Fatalf("pcscfAddresses: got %v, want %v", op.IMS.PCSCFAddresses, tc.addresses)
			}
		})
	}
}
