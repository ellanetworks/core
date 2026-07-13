// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runBGPMatrix exercises the BGP resource: global settings, peers, and the
// advertised/learned route views, each as a subtest.
func runBGPMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	t.Run("settings", func(t *testing.T) { runBGPSettingsMatrix(ctx, t, c) })
	t.Run("peers", func(t *testing.T) { runBGPPeersMatrix(ctx, t, c) })
	t.Run("routes", func(t *testing.T) { runBGPRoutesMatrix(ctx, t, c) })
}
