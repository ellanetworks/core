// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runIPv4AllocationsMatrix exercises the IPv4 allocation listing endpoint
// against a freshly created data network. No PDU sessions exist in the matrix
// environment, so the reported count matches the returned page.
func runIPv4AllocationsMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	name := apiMatrixName("alloc-dn")

	if err := c.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
		Name:     name,
		IPv4Pool: "10.251.0.0/16",
		IPv6Pool: "fd98::/48",
		DNS:      "8.8.8.8",
		Mtu:      1500,
	}); err != nil {
		t.Fatalf("create data network %q: %v", name, err)
	}

	t.Cleanup(func() {
		if err := c.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{Name: name}); err != nil {
			t.Logf("cleanup: delete data network %q: %v", name, err)
		}
	})

	got, err := c.ListIPv4Allocations(ctx, &client.ListIPAllocationsOptions{DataNetworkName: name}, &client.ListParams{Page: 1, PerPage: 100})
	if err != nil {
		t.Fatalf("list ipv4 allocations for %q: %v", name, err)
	}

	if got.TotalCount != len(got.Items) {
		t.Fatalf("ipv4 allocations: TotalCount %d != returned items %d", got.TotalCount, len(got.Items))
	}
}
