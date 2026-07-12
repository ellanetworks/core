// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runRadiosMatrix exercises the radio listing endpoint. No gNB connects in the
// matrix environment, so the inventory is empty.
func runRadiosMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	got, err := c.ListRadios(ctx, &client.ListParams{Page: 1, PerPage: 100})
	if err != nil {
		t.Fatalf("list radios: %v", err)
	}

	if got.TotalCount != 0 {
		t.Fatalf("list radios count: got %d, want 0", got.TotalCount)
	}

	if len(got.Items) != 0 {
		t.Fatalf("list radios items: got %d, want 0", len(got.Items))
	}
}
