// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package apimatrix

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runClusterHealthMatrix(ctx context.Context, t *testing.T, env *Env) {
	c := env.Client

	got, err := c.GetClusterHealth(ctx)
	if err != nil {
		t.Fatalf("get cluster health: %v", err)
	}

	if got.State != client.ClusterHealthHealthy {
		t.Fatalf("State: got %q, want %q", got.State, client.ClusterHealthHealthy)
	}

	if got.TotalVoters != 1 {
		t.Fatalf("TotalVoters: got %d, want 1 on a single-server node", got.TotalVoters)
	}

	if got.HealthyVoters == nil || *got.HealthyVoters != 1 {
		t.Fatalf("HealthyVoters: got %v, want 1", got.HealthyVoters)
	}
}
