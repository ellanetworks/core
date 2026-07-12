// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runBGPRoutesMatrix exercises the advertised and learned route endpoints. Both
// return a well-formed (non-nil) route set regardless of whether BGP is running.
func runBGPRoutesMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	advertised, err := c.GetBGPAdvertisedRoutes(ctx)
	if err != nil {
		t.Fatalf("get bgp advertised routes: %v", err)
	}

	if advertised.Routes == nil {
		t.Fatalf("advertised routes: got nil, want non-nil slice")
	}

	learned, err := c.GetBGPLearnedRoutes(ctx)
	if err != nil {
		t.Fatalf("get bgp learned routes: %v", err)
	}

	if learned.Routes == nil {
		t.Fatalf("learned routes: got nil, want non-nil slice")
	}
}
