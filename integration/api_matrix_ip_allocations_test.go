// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runIPAllocationsMatrix exercises the IPv4 and IPv6 allocation listing
// endpoints against a freshly created dual-stack data network. No PDU sessions
// exist in the matrix environment, so the reported count matches the returned
// page.
func runIPAllocationsMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	name := apiMatrixName("alloc-dn")

	if err := c.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
		Name:     name,
		IPv4Pool: "10.251.0.0/16",
		IPv6Pool: "fd97::/48",
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

	opts := &client.ListIPAllocationsOptions{DataNetworkName: name}
	params := &client.ListParams{Page: 1, PerPage: 100}

	v4, err := c.ListIPv4Allocations(ctx, opts, params)
	if err != nil {
		t.Fatalf("list ipv4 allocations for %q: %v", name, err)
	}

	if v4.TotalCount != len(v4.Items) {
		t.Fatalf("ipv4 allocations: TotalCount %d != returned items %d", v4.TotalCount, len(v4.Items))
	}

	v6, err := c.ListIPv6Allocations(ctx, opts, params)
	if err != nil {
		t.Fatalf("list ipv6 allocations for %q: %v", name, err)
	}

	if v6.TotalCount != len(v6.Items) {
		t.Fatalf("ipv6 allocations: TotalCount %d != returned items %d", v6.TotalCount, len(v6.Items))
	}
}
