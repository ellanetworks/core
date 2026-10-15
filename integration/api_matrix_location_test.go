// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runLocationMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	t.Run("cell_positions", func(t *testing.T) { runCellPositionsMatrix(ctx, t, c) })
	t.Run("deferred_location_requests", func(t *testing.T) { runDeferredLocationRequestsMatrix(ctx, t, c) })
}
