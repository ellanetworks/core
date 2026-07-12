// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runAuthMatrix exercises the session auth endpoints. The test client
// authenticates with an API token, which is validated independently of the JWT
// secret, so logout (session-cookie only) and rotate-secret (JWT sessions only)
// leave it working. rotate-secret runs last, followed by an authenticated call
// that confirms the token survived.
func runAuthMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	lookup, err := c.LookupToken(ctx)
	if err != nil {
		t.Fatalf("lookup token: %v", err)
	}

	if !lookup.Valid {
		t.Fatalf("lookup token: got invalid, want valid")
	}

	if err := c.Logout(ctx); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if err := c.RotateSecret(ctx); err != nil {
		t.Fatalf("rotate secret: %v", err)
	}

	if _, err := c.GetMyUser(ctx); err != nil {
		t.Fatalf("get my user after rotate-secret (API token invalidated?): %v", err)
	}
}
