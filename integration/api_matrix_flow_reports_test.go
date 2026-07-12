// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runFlowReportsMatrix exercises the flow report listing and stats endpoints.
// No data-plane traffic flows in the matrix environment, so no flows are
// recorded.
func runFlowReportsMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	list, err := c.ListFlowReports(ctx, &client.ListFlowReportsParams{Page: 1, PerPage: 100})
	if err != nil {
		t.Fatalf("list flow reports: %v", err)
	}

	if list.TotalCount != 0 {
		t.Fatalf("list flow reports count: got %d, want 0", list.TotalCount)
	}

	if len(list.Items) != 0 {
		t.Fatalf("list flow reports items: got %d, want 0", len(list.Items))
	}

	stats, err := c.GetFlowReportStats(ctx, &client.ListFlowReportsParams{})
	if err != nil {
		t.Fatalf("get flow report stats: %v", err)
	}

	if len(stats.Protocols) != 0 {
		t.Fatalf("flow report stats protocols: got %d, want 0", len(stats.Protocols))
	}

	if len(stats.TopDestinationsUplink) != 0 {
		t.Fatalf("flow report stats top destinations: got %d, want 0", len(stats.TopDestinationsUplink))
	}
}
