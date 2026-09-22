// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runClusterHealthMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	got, err := c.GetClusterHealth(ctx)
	if err != nil {
		t.Fatalf("get cluster health: %v", err)
	}

	if got.Enabled {
		t.Fatalf("Enabled: got true, want false on a standalone node")
	}

	if got.TotalVoters != 0 {
		t.Fatalf("TotalVoters: got %d, want 0", got.TotalVoters)
	}

	if got.HealthyVoters > got.TotalVoters {
		t.Fatalf("HealthyVoters %d exceeds TotalVoters %d", got.HealthyVoters, got.TotalVoters)
	}
}
