// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runRadioEventsMatrix exercises the radio event listing endpoint. No gNB
// connects in the matrix environment, so no events are recorded.
func runRadioEventsMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	got, err := c.ListRadioEvents(ctx, &client.ListRadioEventsParams{Page: 1, PerPage: 100})
	if err != nil {
		t.Fatalf("list radio events: %v", err)
	}

	if got.TotalCount != 0 {
		t.Fatalf("list radio events count: got %d, want 0", got.TotalCount)
	}

	if len(got.Items) != 0 {
		t.Fatalf("list radio events items: got %d, want 0", len(got.Items))
	}

	if err := c.ClearRadioEvents(ctx); err != nil {
		t.Fatalf("clear radio events: %v", err)
	}

	afterClear, err := c.ListRadioEvents(ctx, &client.ListRadioEventsParams{Page: 1, PerPage: 100})
	if err != nil {
		t.Fatalf("list radio events after clear: %v", err)
	}

	if afterClear.TotalCount != 0 {
		t.Fatalf("list radio events after clear: got %d, want 0", afterClear.TotalCount)
	}
}
