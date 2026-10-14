// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package apimatrix

import (
	"context"
	"testing"
)

// runLocationMatrix exercises the beta location subsystem: operator-provisioned
// cell positions and runtime positioning sessions, each as a subtest.
func runLocationMatrix(ctx context.Context, t *testing.T, env *Env) {
	c := env.Client

	t.Run("cell_positions", func(t *testing.T) { runCellPositionsMatrix(ctx, t, c) })
	t.Run("positioning_sessions", func(t *testing.T) { runPositioningSessionsMatrix(ctx, t, c) })
}
