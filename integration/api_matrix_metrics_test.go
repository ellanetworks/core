// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runMetricsMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	got, err := c.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}

	if len(got) == 0 {
		t.Fatalf("metrics: got 0 series, want at least one")
	}
}
