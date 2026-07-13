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

	if err := c.ClearUsage(ctx); err != nil {
		t.Fatalf("clear usage: %v", err)
	}

	afterClear, err := c.ListUsage(ctx, &client.ListUsageParams{GroupBy: "subscriber"})
	if err != nil {
		t.Fatalf("list usage after clear: %v", err)
	}

	if len(*afterClear) != 0 {
		t.Fatalf("list usage after clear: got %d entries, want 0", len(*afterClear))
	}

	t.Run("retention", func(t *testing.T) { runSubscriberUsageRetentionMatrix(ctx, t, c) })
}
