// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runLocationMatrix exercises the beta location subsystem: operator-provisioned
// cell positions and runtime positioning sessions, each as a subtest.
func runLocationMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	t.Run("cell_positions", func(t *testing.T) { runCellPositionsMatrix(ctx, t, c) })
	t.Run("positioning_sessions", func(t *testing.T) { runPositioningSessionsMatrix(ctx, t, c) })
}
