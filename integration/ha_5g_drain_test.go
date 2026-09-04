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

func TestIntegration5GHADrain(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runHA3GPPScenario(t, "ha/drain_5g", func(ctx context.Context, leader *client.Client, nodeID int) error {
		resp, err := leader.DrainClusterMember(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("DrainClusterMember(%d): %w", nodeID, err)
		}

		if resp.DrainState != "draining" && resp.DrainState != "drained" {
			return fmt.Errorf("drainState = %q, want draining or drained", resp.DrainState)
		}

		HALogf(t, "drained node %d; drainState=%s", nodeID, resp.DrainState)

		if err := waitForDrained(ctx, leader, nodeID); err != nil {
			return err
		}

		if err := leader.ResumeClusterMember(ctx, nodeID); err != nil {
			return fmt.Errorf("ResumeClusterMember(%d): %w", nodeID, err)
		}

		HALogf(t, "resumed node %d", nodeID)

		return nil
	})
}
