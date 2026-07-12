// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runSubscriberUsageMatrix exercises the usage listing endpoint. No data-plane
// traffic flows in the matrix environment, so no usage is recorded. The server
// requires group_by to be "day" or "subscriber".
func runSubscriberUsageMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	got, err := c.ListUsage(ctx, &client.ListUsageParams{GroupBy: "subscriber"})
	if err != nil {
		t.Fatalf("list usage: %v", err)
	}

	if len(*got) != 0 {
		t.Fatalf("list usage: got %d entries, want 0", len(*got))
	}
}
