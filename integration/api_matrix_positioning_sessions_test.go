// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runPositioningSessionsMatrix exercises the positioning-session listing
// endpoint. No positioning procedure runs in the matrix environment, so no
// sessions exist.
func runPositioningSessionsMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	got, err := c.ListPositioningSessions(ctx)
	if err != nil {
		t.Fatalf("list positioning sessions: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("list positioning sessions: got %d, want 0", len(got))
	}
}
