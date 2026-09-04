// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestIntegration4GHADrain(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runHA3GPPScenario(t, "ha/drain_4g", func(ctx context.Context, leader *client.Client, nodeID int) error {
		resp, err := leader.DrainClusterMember(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("DrainClusterMember(%d): %w", nodeID, err)
		}

		if resp.DrainState != "draining" && resp.DrainState != "drained" {
			return fmt.Errorf("drainState = %q, want draining or drained", resp.DrainState)
		}

		HALogf(t, "drained node %d; drainState=%s", nodeID, resp.DrainState)

		return waitForDrained(ctx, leader, nodeID)
	})
}
