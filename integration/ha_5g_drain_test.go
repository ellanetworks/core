// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
)

func TestIntegration5GHADrain(t *testing.T) {
	suites.Require(t, suites.HA3GPP5G)

	runHA3GPPScenario(t, "ha/drain_5g", func(ctx context.Context, leader *client.Client, nodeID client.NodeID) error {
		resp, err := leader.DrainClusterMember(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("DrainClusterMember(%s): %w", nodeID, err)
		}

		if resp.DrainState != "draining" && resp.DrainState != "drained" {
			return fmt.Errorf("drainState = %q, want draining or drained", resp.DrainState)
		}

		HALogf(t, "drained node %s; drainState=%s", nodeID, resp.DrainState)

		if err := waitForDrained(ctx, leader, nodeID); err != nil {
			return err
		}

		if err := leader.ResumeClusterMember(ctx, nodeID); err != nil {
			return fmt.Errorf("ResumeClusterMember(%s): %w", nodeID, err)
		}

		HALogf(t, "resumed node %s", nodeID)

		return nil
	})
}
