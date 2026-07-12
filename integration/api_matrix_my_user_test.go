// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runMyUserMatrix exercises the logged-in-user endpoint. The test client
// authenticates as the bootstrap admin, so the endpoint returns that identity.
func runMyUserMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	got, err := c.GetMyUser(ctx)
	if err != nil {
		t.Fatalf("get my user: %v", err)
	}

	if got.Email != "admin@ellanetworks.com" {
		t.Fatalf("logged-in user email: got %q, want %q", got.Email, "admin@ellanetworks.com")
	}
}
